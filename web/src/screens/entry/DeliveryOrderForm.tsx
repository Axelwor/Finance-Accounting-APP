import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { openPrintWindow } from "../../lib/print";
import { draftNumber } from "../../workbench/modules";
import type { DeliveryLineInput, Item, SalesOrderListItem } from "../../types";
import type { PrefillRef } from "../../workbench/types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
  /** Workflow-chain prefill: {kind:"sales-order", id} selects that SO. */
  prefill?: PrefillRef;
}

interface Line {
  id: string;
  itemId: string;
  itemCode: string;
  itemName: string;
  qty: number;
  unitCostCents: number;
  cogsCents: number;
}

export function DeliveryOrderForm({ tabId, entryId, initialTitle, prefill }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("delivery-order-entry"));
  const [salesOrderId, setSalesOrderId] = useState(
    prefill?.kind === "sales-order" ? String(prefill.id) : "",
  );
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [items, setItems] = useState<Item[]>([]);
  const [salesOrders, setSalesOrders] = useState<SalesOrderListItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [doId, setDoId] = useState<number | null>(typeof entryId === "number" ? entryId : null);
  const [doStatus, setDoStatus] = useState("DRAFT");
  const [totalCOGS, setTotalCOGS] = useState(0);
  /** Ordered qty per item for the selected SO (over-delivery validation). */
  const [orderedQty, setOrderedQty] = useState<Record<string, number>>({});
  const [loadingSO, setLoadingSO] = useState(false);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, salesOrderId, notes, lines, workbench]);

  useEffect(() => {
    void api.listItems().then(setItems);
    void api.listSalesOrders("CONFIRMED").then(setSalesOrders);
  }, []);

  // Auto-fill: when a sales order is picked (manually or via workflow-chain
  // prefill), copy its lines into the delivery so nothing is re-keyed.
  useEffect(() => {
    if (!salesOrderId || doId !== null) {
      setOrderedQty({});
      return;
    }
    let cancelled = false;
    setLoadingSO(true);
    void api
      .getSalesOrder(Number(salesOrderId))
      .then((so) => {
        if (cancelled) return;
        const ordered: Record<string, number> = {};
        for (const l of so.lines) ordered[String(l.item_id)] = Number(l.qty) || 0;
        setOrderedQty(ordered);
        setLines(
          so.lines.length > 0
            ? so.lines.map((l) => ({
                id: `ln-src-${l.id}`,
                itemId: String(l.item_id),
                itemCode: l.item_code ?? "",
                itemName: l.item_name ?? "",
                qty: Number(l.qty) || 1,
                // COGS is resolved server-side for FIFO/average items; the
                // cost entered here only matters for specific-identification.
                unitCostCents: 0,
                cogsCents: 0,
              }))
            : [seedLine()],
        );
        toast.info(`Loaded ${so.lines.length} line(s) from ${so.number}`);
      })
      .catch(() => {
        if (!cancelled) setOrderedQty({});
      })
      .finally(() => {
        if (!cancelled) setLoadingSO(false);
      });
    return () => {
      cancelled = true;
    };
  }, [salesOrderId, doId, toast]);

  // Over-delivery warning: delivering more than ordered is allowed (backorder
  // corrections happen) but must be a conscious choice, so warn loudly.
  const overDelivered = useMemo(
    () =>
      lines.filter((l) => {
        const ordered = orderedQty[l.itemId];
        return l.itemId && ordered !== undefined && l.qty > ordered;
      }),
    [lines, orderedQty],
  );

  useEffect(() => {
    if (doId) {
      void loadDO(doId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doId]);

  const loadDO = async (id: number) => {
    try {
      const deliv = await api.getDeliveryOrder(id);
      setNumber(deliv.number);
      setDoStatus(deliv.status);
      setTotalCOGS(deliv.total_cogs_cents);
      setDate(deliv.delivery_date);
      setSalesOrderId(String(deliv.sales_order_id));
      setNotes(deliv.notes ?? "");
      setLines(
        deliv.lines.map((l) => ({
          id: `ln-${l.id}`,
          itemId: String(l.item_id),
          itemCode: l.item_code ?? "",
          itemName: l.item_name ?? "",
          qty: Number(l.qty),
          unitCostCents: l.unit_cost_cents,
          cogsCents: l.cogs_cents,
        })),
      );
      workbench.markUnsaved(tabId, false);
    } catch {
      // new DO or fetch failed
    }
  };

  const totalCOGSCents = useMemo(
    () => lines.reduce((sum, l) => sum + l.cogsCents, 0),
    [lines],
  );

  const setItem = (id: string, itemId: string) => {
    const item = items.find((i) => String(i.id) === itemId);
    setLines((cur) =>
      cur.map((l) =>
        l.id === id ? { ...l, itemId, itemCode: item?.code ?? "", itemName: item?.name ?? "" } : l,
      ),
    );
  };

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, qty, cogsCents: Math.round(qty) * l.unitCostCents } : l)),
    );
  };

  const setUnitCost = (id: string, unitCostCents: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, unitCostCents, cogsCents: Math.round(l.qty) * unitCostCents } : l)),
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!salesOrderId) {
      setError("Pick a sales order to deliver against.");
      return;
    }
    const payloadLines: DeliveryLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: l.qty > 0 ? l.qty : 1,
        unit_cost_cents: l.unitCostCents,
        description: undefined,
      }));
    if (payloadLines.length === 0) {
      setError("Add at least one item line.");
      return;
    }
    setSaving(true);
    try {
      const created = await api.createDeliveryOrder({
        sales_order_id: Number(salesOrderId),
        delivery_date: date,
        notes: notes.trim() || undefined,
        lines: payloadLines,
      });
      setDoId(created.id);
      setDoStatus(created.status);
      setTotalCOGS(created.total_cogs_cents);
      setNumber(created.number);
      workbench.replaceDraft(tabId, created.number, created.status);
      workbench.markUnsaved(tabId, false);
      toast.success(`✓ Saved ${created.number}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the delivery order.");
    } finally {
      setSaving(false);
    }
  };

  /** Workflow chain: open an Invoice draft pre-filled from this delivery. */
  const handleCreateInvoice = () => {
    if (!doId) return;
    workbench.openEntryDraftFromParent("sales-invoice", { kind: "delivery-order", id: doId });
  };

  const handlePrint = () => {
    const so = salesOrders.find((s) => String(s.id) === salesOrderId);
    openPrintWindow({
      title: `Delivery Order ${number}`,
      subtitle: isExisting ? doStatus : "DRAFT",
      meta: [
        ["Sales Order", so ? so.number : salesOrderId || "-"],
        ["Delivery date", formatDateID(date)],
        ["Notes", notes || "-"],
      ],
      columns: [
        { label: "Item" },
        { label: "Qty", right: true },
        { label: "Unit Cost", right: true },
        { label: "COGS", right: true },
      ],
      rows: lines
        .filter((l) => l.itemId)
        .map((l) => [
          `${l.itemCode || l.itemId}${l.itemName ? ` · ${l.itemName}` : ""}`,
          l.qty,
          formatIDR(l.unitCostCents),
          formatIDR(l.cogsCents),
        ]),
      totals: [["Total COGS", formatIDR(isExisting ? totalCOGS : totalCOGSCents)]],
    });
  };

  const isExisting = doId !== null;

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Delivery Order</span>
          <span className={`entrytab__status ${isExisting ? "" : "entrytab__status--draft"}`}>
            {isExisting ? doStatus : "DRAFT"}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDateID(date)}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          <div className="entrytab__header-grid">
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">Sales Order</span>
                <select className="input" value={salesOrderId} onChange={(e) => setSalesOrderId(e.target.value)} disabled={isExisting}>
                  <option value="">Choose sales order...</option>
                  {salesOrders.map((so) => (
                    <option key={so.id} value={so.id}>
                      {so.number} · {so.customer_name ?? `#${so.customer_id}`}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Delivery Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} disabled={isExisting} />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">No</span>
                <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
              </label>
            </div>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Delivery instructions, carrier info..." disabled={isExisting} />
          </label>

          {overDelivered.length > 0 && (
            <div className="field-warning" role="alert">
              Delivery qty exceeds the ordered qty for:{" "}
              {overDelivered
                .map((l) => `${l.itemCode || l.itemName || `item ${l.itemId}`} (ordered ${orderedQty[l.itemId]}, delivering ${l.qty})`)
                .join("; ")}
              . Double-check before saving.
            </div>
          )}

          <div className="entrytab__detail">
            <div className="entrytab__detail-title">Item lines *{loadingSO ? " — loading from sales order..." : ""}</div>
            <div className="detail-grid detail-grid--grn">
              <div className="detail-grid__head">
                <div>Item</div>
                <div>Qty</div>
                <div className="right">Unit Cost</div>
                <div className="right">COGS</div>
                <div aria-hidden="true" />
              </div>
              {lines.map((line) => (
                <div className="detail-grid__row" key={line.id}>
                  <div>
                    <select className="input" value={line.itemId} onChange={(e) => setItem(line.id, e.target.value)} disabled={isExisting}>
                      <option value="">Choose item...</option>
                      {items.map((i) => (
                        <option key={i.id} value={i.id}>
                          {i.code} · {i.name}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <input className="amount" type="number" min={1} step="any" value={line.qty || ""} onChange={(e) => setQty(line.id, Number(e.target.value))} placeholder="1" disabled={isExisting} />
                  </div>
                  <div>
                    <input className="amount right" type="text" inputMode="numeric" value={centsInput(line.unitCostCents)} onChange={(e) => setUnitCost(line.id, parseCents(e.target.value))} placeholder="0" disabled={isExisting} />
                  </div>
                  <div className="right">
                    <span className="ledger-table__amount">{formatIDR(line.cogsCents)}</span>
                  </div>
                  <div>
                    <button type="button" className="detail-grid__remove" onClick={() => setLines((cur) => (cur.length > 1 ? cur.filter((l) => l.id !== line.id) : cur))} aria-label="Remove line" disabled={isExisting || lines.length === 1}>
                      ×
                    </button>
                  </div>
                </div>
              ))}
              {!isExisting && (
                <div className="detail-grid__row detail-grid__row--add">
                  <div>
                    <button type="button" className="btn btn--secondary btn--sm" onClick={() => setLines((cur) => [...cur, seedLine()])}>
                      + Add item
                    </button>
                  </div>
                  <div />
                  <div />
                  <div />
                  <div />
                </div>
              )}
            </div>
          </div>

          <div className="entrytab__total">
            <span className="entrytab__total-label">Total COGS</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? totalCOGS : totalCOGSCents)}</span>
          </div>

          {isExisting && (
            <NextStepsBar number={number} hint={doStatus}>
              <button type="button" className="next-steps__btn next-steps__btn--primary" onClick={handleCreateInvoice}>
                Create Invoice
              </button>
              <button type="button" className="next-steps__btn" onClick={handlePrint}>
                Print DO
              </button>
              <button type="button" className="next-steps__btn" onClick={() => workbench.close(tabId)}>
                Close
              </button>
            </NextStepsBar>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving} title="Create delivery (posts COGS journal)">
              <DiskIcon />
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function seedLine(): Line {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 1,
    unitCostCents: 0,
    cogsCents: 0,
  };
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function centsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(cents);
}

function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function formatDateID(iso: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="currentColor" />
      <path d="M12 7v5l3 2" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}
