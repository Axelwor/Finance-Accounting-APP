import { useCallback, useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDRFromCents, parseRupiahToCents } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { PurchaseOrderListItem, Item, GRNLineInput } from "../../types";
import type { PrefillRef } from "../../workbench/types";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
  /** Workflow-chain prefill: {kind:"purchase-order", id} selects that PO. */
  prefill?: PrefillRef;
}

interface Line {
  id: string;
  itemId: string;
  qty: string;
  unitCostCents: string;
}

/** PO line data kept for auto-fill and the "Receive All" shortcut. */
interface PoLineInfo {
  itemId: string;
  remainingQty: number;
  unitCostCents: number;
}

export function GRNForm({ tabId, entryId, initialTitle, prefill }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();

  const [purchaseOrders, setPurchaseOrders] = useState<PurchaseOrderListItem[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [poId, setPoId] = useState(prefill?.kind === "purchase-order" ? String(prefill.id) : "");
  const [grnDate, setGrnDate] = useState(new Date().toISOString().slice(0, 10));
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([{ id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0" }]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  /** PO lines pending receipt, used by auto-fill and "Receive All". */
  const [poLines, setPoLines] = useState<PoLineInfo[]>([]);
  const [loadingPO, setLoadingPO] = useState(false);
  /** Backend id once the GRN is saved. */
  const [savedId, setSavedId] = useState<number | null>(
    entryId != null && !Number.isNaN(Number(entryId)) ? Number(entryId) : null,
  );
  const [savedNumber, setSavedNumber] = useState(initialTitle ?? "");
  const [savedStatus, setSavedStatus] = useState("");

  /** Read-only once the GRN exists (reopened tab or just saved). */
  const isExisting = !!entryId || savedId !== null;

  const loadMasterData = useCallback(() => {
    api.listPurchaseOrders("CONFIRMED").then(setPurchaseOrders).catch(() => {});
    api.listPurchaseOrders("PARTIALLY_RECEIVED").then((partial) => {
      setPurchaseOrders((prev) => [...prev, ...partial]);
    }).catch(() => {});
    api.listItems().then(setItems).catch(() => {});
  }, []);
  useEffect(() => {
    loadMasterData();
  }, [loadMasterData]);
  useTabRefresh(loadMasterData);

  // Auto-fill: when a PO is picked (manually or via workflow-chain prefill),
  // load its lines and pre-populate the remaining qty per item (D-08).
  useEffect(() => {
    if (!poId || isExisting) {
      setPoLines([]);
      return;
    }
    let cancelled = false;
    setLoadingPO(true);
    void api
      .getPurchaseOrder(Number(poId))
      .then((po) => {
        if (cancelled) return;
        const infos: PoLineInfo[] = po.lines.map((l) => ({
          itemId: String(l.item_id),
          remainingQty: Math.max((Number(l.qty) || 0) - (Number(l.received_qty) || 0), 0),
          unitCostCents: l.unit_price_cents,
        }));
        setPoLines(infos);
        const receivable = infos.filter((l) => l.remainingQty > 0);
        if (receivable.length === 0) {
          toast.warning(`All lines on ${po.number} are already fully received.`);
          return;
        }
        setLines(
          receivable.map((l) => ({
            id: crypto.randomUUID(),
            itemId: l.itemId,
            qty: String(l.remainingQty),
            unitCostCents: String(l.unitCostCents),
          })),
        );
        toast.info(`Loaded ${receivable.length} line(s) from ${po.number}`);
      })
      .catch(() => {
        if (!cancelled) setPoLines([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingPO(false);
      });
    return () => {
      cancelled = true;
    };
  }, [poId, isExisting, toast]);

  /** Set every line's qty back to the PO's remaining quantity. */
  function receiveAll() {
    const receivable = poLines.filter((l) => l.remainingQty > 0);
    if (receivable.length === 0) {
      toast.warning("Nothing left to receive on this purchase order.");
      return;
    }
    setLines(
      receivable.map((l) => ({
        id: crypto.randomUUID(),
        itemId: l.itemId,
        qty: String(l.remainingQty),
        unitCostCents: String(l.unitCostCents),
      })),
    );
    markDirty();
  }

  function markDirty() {
    workbench.markUnsaved(tabId, true);
  }

  const totalCents = lines.reduce(
    (sum, l) => sum + Math.round((parseFloat(l.qty) || 0) * (parseInt(l.unitCostCents) || 0)),
    0,
  );

  function setLine(id: string, field: keyof Line, value: string) {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
    workbench.markUnsaved(tabId, true);
  }

  function addLine() {
    setLines((prev) => [...prev, { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0" }]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!poId) { setError("Purchase order is required."); return; }
    if (lines.every((l) => !l.itemId)) { setError("At least one item line is required."); return; }
    const inputLines: GRNLineInput[] = lines.filter((l) => l.itemId).map((l) => ({
      item_id: Number(l.itemId),
      qty: parseFloat(l.qty) || 0,
      unit_cost_cents: parseInt(l.unitCostCents) || 0,
    }));
    if (inputLines.some((l) => l.qty <= 0)) { setError("Quantity must be positive."); return; }
    setSaving(true);
    try {
      const grn = await api.createGRN({
        purchase_order_id: Number(poId),
        grn_date: grnDate,
        notes: notes || undefined,
        lines: inputLines,
      });
      workbench.replaceDraft(tabId, grn.number, grn.status);
      workbench.markUnsaved(tabId, false);
      setSavedId(grn.id);
      setSavedNumber(grn.number);
      setSavedStatus(grn.status);
      toast.success(`✓ Saved ${grn.number}`);
    } catch (err: any) {
      setError(err?.message || "Failed to create GRN.");
    } finally {
      setSaving(false);
    }
  }

  /** Workflow chain: open a Supplier Invoice draft from this GRN. */
  function handleCreateSupplierInvoice() {
    if (!savedId) return;
    workbench.openEntryDraftFromParent("supplier-invoice-entry", { kind: "grn", id: savedId });
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{savedNumber || initialTitle || "Goods Received Note"}</div>
          <div className="entrytab__header-number">{isExisting ? savedNumber || initialTitle : draftNumber("grn-entry")}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Purchase Order *</span>
              <select className="input" value={poId} onChange={(e) => setPoId(e.target.value)} disabled={isExisting}>
                <option value="">Choose PO...</option>
                {purchaseOrders.map((po) => (
                  <option key={po.id} value={po.id}>
                    {po.number} · {po.supplier_name ?? `#${po.supplier_id}`}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">GRN Date *</span>
              <input className="input" type="date" value={grnDate} onChange={(e) => setGrnDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={isExisting} />
          </label>

          <div className="entrytab__detail-title">
            Received items *{loadingPO ? " — loading from purchase order..." : ""}
            {!isExisting && poLines.some((l) => l.remainingQty > 0) && (
              <Button
                variant="outlined"
                size="sm"
                style={{ marginLeft: 12 }}
                onClick={receiveAll}
                title="Set every line to the remaining ordered quantity"
              >
                Receive All
              </Button>
            )}
          </div>
          <div className="detail-grid detail-grid--grn">
            <div className="detail-grid__head">
              <div>Item</div>
              <div>Qty</div>
              <div className="right">Unit Cost</div>
              <div className="right">Line Total</div>
              <div aria-hidden="true" />
            </div>
            {lines.map((line) => (
              <div className="detail-grid__row" key={line.id}>
                <div>
                  <select className="input" value={line.itemId} onChange={(e) => setLine(line.id, "itemId", e.target.value)} disabled={isExisting}>
                    <option value="">Choose item...</option>
                    {items.map((i) => (
                      <option key={i.id} value={i.id}>
                        {i.code} · {i.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <input className="input input--narrow" type="number" step="0.001" value={line.qty} onChange={(e) => setLine(line.id, "qty", e.target.value)} disabled={isExisting} />
                </div>
                <div>
                  <input
                    className="input input--narrow right"
                    type="number"
                    min="0"
                    value={Number(line.unitCostCents) / 100 || 0}
                    onChange={(e) => setLine(line.id, "unitCostCents", String(parseRupiahToCents(e.target.value)))}
                    disabled={isExisting}
                  />
                </div>
                <div className="right">
                  {formatIDRFromCents(Math.round((parseFloat(line.qty) || 0) * (parseInt(line.unitCostCents) || 0)))}
                </div>
                <div>
                  {!isExisting && (
                    <button type="button" className="detail-grid__remove" onClick={() => removeLine(line.id)} aria-label="Remove line">
                      ×
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
          {!isExisting && (
            <Button
              variant="text"
              onClick={addLine}
              style={{ marginTop: 8 }}
            >
              + Add line
            </Button>
          )}

          <div className="entrytab__total">
            <span className="entrytab__total-label">Total (Dr Inventory / Cr Payable)</span>
            <span className="entrytab__total-value">{formatIDRFromCents(totalCents)}</span>
          </div>

          {savedId !== null && (
            <NextStepsBar number={savedNumber || undefined} hint={savedStatus || undefined}>
              <button type="button" className="next-steps__btn next-steps__btn--primary" onClick={handleCreateSupplierInvoice}>
                Create Supplier Invoice
              </button>
              <button type="button" className="next-steps__btn" onClick={() => workbench.close(tabId)}>
                Close
              </button>
            </NextStepsBar>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
