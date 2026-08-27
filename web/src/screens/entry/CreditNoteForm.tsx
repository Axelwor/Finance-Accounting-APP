import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { StaticCombobox, type ComboboxOption } from "../../components/Combobox";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDRFromCents, parseRupiahToCents } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { InvoiceListItem, InvoiceLine, CreditNoteLineInput } from "../../types";
import type { PrefillRef } from "../../workbench/types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
  /** Workflow-chain prefill: {kind:"invoice", id} selects that invoice. */
  prefill?: PrefillRef;
}

interface Line {
  id: string;
  itemId: string;
  itemCode: string;
  itemName: string;
  qty: number;
  unitPriceCents: number;
  unitCostCents: number;
  lineTotalCents: number;
  cogsReversedCents: number;
  /** Source invoice line (for backend cost lookup + qty validation). */
  invoiceLineId?: number;
  deliveryOrderId?: number;
  invoicedQty: number;
}

interface ItemOption extends ComboboxOption {
  value: string;
  invoiceLine: InvoiceLine;
}

const REFUND_METHODS: { value: string; label: string }[] = [
  { value: "deduct", label: "Deduct from AR" },
  { value: "refund", label: "Cash refund" },
];

/**
 * Credit Note (CN) entry form. Posts a return journal:
 *  - Dr 4201 Sales Returns / Cr 1201 AR (or Cash for refund method)
 *  - Dr 1301 Inventory / Cr 5101 COGS (reverse cost of returned goods)
 *
 * Lines are auto-filled from the selected invoice; return qty is validated
 * against the invoiced qty per item.
 */
export function CreditNoteForm({ tabId, entryId, initialTitle, prefill }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [cnDate, setCnDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("credit-note-entry"));
  const [invoiceId, setInvoiceId] = useState(prefill?.kind === "invoice" ? String(prefill.id) : "");
  const [refundMethod, setRefundMethod] = useState("deduct");
  const [reason, setReason] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [invoices, setInvoices] = useState<InvoiceListItem[]>([]);
  const [selectedInvoice, setSelectedInvoice] = useState<InvoiceListItem | null>(null);
  /** Lines of the selected invoice — source for auto-fill + item options. */
  const [invoiceLines, setInvoiceLines] = useState<InvoiceLine[]>([]);
  const [loadingInvoice, setLoadingInvoice] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [savedId, setSavedId] = useState<number | null>(
    entryId != null && !Number.isNaN(Number(entryId)) ? Number(entryId) : null,
  );

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, cnDate, number, invoiceId, refundMethod, reason, lines, workbench]);

  useEffect(() => {
    void api.listInvoices().then((list) => {
      // Only show ISSUED or PARTIALLY_PAID invoices — can't return what hasn't been invoiced.
      setInvoices(list.filter((i) => i.status === "ISSUED" || i.status === "PARTIALLY_PAID"));
    });
  }, []);

  // Auto-fill: load the selected invoice's lines (manual pick or prefill).
  useEffect(() => {
    if (!invoiceId) {
      setInvoiceLines([]);
      setSelectedInvoice(null);
      return;
    }
    let cancelled = false;
    setLoadingInvoice(true);
    void api
      .getInvoice(Number(invoiceId))
      .then((inv) => {
        if (cancelled) return;
        setSelectedInvoice({
          id: inv.id,
          number: inv.number,
          sales_order_id: inv.sales_order_id,
          customer_id: inv.customer_id,
          customer_name: inv.customer_name,
          invoice_date: inv.invoice_date,
          due_date: inv.due_date,
          notes: inv.notes,
          status: inv.status,
          total_cents: inv.total_cents,
          dp_applied_cents: inv.dp_applied_cents,
          receivable_cents: inv.receivable_cents,
        });
        setInvoiceLines(inv.lines);
        setLines(
          inv.lines.length > 0
            ? inv.lines.map((l) => {
                const qty = Number(l.qty) || 0;
                return {
                  id: `ln-src-${l.id}`,
                  itemId: String(l.item_id),
                  itemCode: l.item_code ?? "",
                  itemName: l.item_name ?? "",
                  qty,
                  unitPriceCents: l.unit_price_cents,
                  unitCostCents: 0,
                  lineTotalCents: Math.round(qty * l.unit_price_cents),
                  cogsReversedCents: 0,
                  invoiceLineId: l.id,
                  deliveryOrderId: l.delivery_id,
                  invoicedQty: qty,
                };
              })
            : [seedLine()],
        );
        toast.info(`Loaded ${inv.lines.length} line(s) from invoice ${inv.number}`);
      })
      .catch(() => {
        if (!cancelled) {
          setInvoiceLines([]);
          setSelectedInvoice(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingInvoice(false);
      });
    return () => {
      cancelled = true;
    };
  }, [invoiceId, toast]);

  /** Combobox options: only items that appear on the selected invoice. */
  const itemOptions = useMemo<ItemOption[]>(
    () =>
      invoiceLines.map((l) => ({
        value: String(l.item_id),
        label: l.item_name ?? `Item ${l.item_id}`,
        code: l.item_code,
        invoiceLine: l,
      })),
    [invoiceLines],
  );

  const totalCents = useMemo(() => lines.reduce((s, l) => s + l.lineTotalCents, 0), [lines]);
  const totalCOGSReversed = useMemo(() => lines.reduce((s, l) => s + l.cogsReversedCents, 0), [lines]);

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              qty,
              lineTotalCents: qty > 0 ? Math.round(qty * l.unitPriceCents) : 0,
              cogsReversedCents: qty > 0 ? Math.round(qty * l.unitCostCents) : 0,
            }
          : l,
      ),
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, unitPriceCents, lineTotalCents: l.qty > 0 ? Math.round(l.qty * unitPriceCents) : 0 }
          : l,
      ),
    );
  };

  const setCost = (id: string, unitCostCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, unitCostCents, cogsReversedCents: l.qty > 0 ? Math.round(l.qty * unitCostCents) : 0 }
          : l,
      ),
    );
  };

  /** Swap a line's item for another invoiced item via the combobox. */
  const setItem = (id: string, option: ItemOption | null) => {
    if (!option) return;
    const src = option.invoiceLine;
    const invoicedQty = Number(src.qty) || 0;
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              itemId: String(src.item_id),
              itemCode: src.item_code ?? "",
              itemName: src.item_name ?? "",
              unitPriceCents: src.unit_price_cents,
              invoiceLineId: src.id,
              deliveryOrderId: src.delivery_id,
              invoicedQty,
              lineTotalCents: l.qty > 0 ? Math.round(l.qty * src.unit_price_cents) : 0,
            }
          : l,
      ),
    );
  };

  const addLine = () => setLines((cur) => [...cur, seedLine()]);
  const removeLine = (id: string) => setLines((cur) => (cur.length > 1 ? cur.filter((l) => l.id !== id) : cur));

  /** Aggregate returned qty per item and compare with invoiced qty. */
  const overReturned = useMemo(() => {
    const returnedByItem = new Map<string, number>();
    for (const l of lines) {
      if (!l.itemId || l.qty <= 0) continue;
      returnedByItem.set(l.itemId, (returnedByItem.get(l.itemId) ?? 0) + l.qty);
    }
    const problems: string[] = [];
    for (const [itemId, returned] of returnedByItem) {
      const line = lines.find((l) => l.itemId === itemId);
      if (!line) continue;
      if (line.invoicedQty > 0 && returned > line.invoicedQty) {
        problems.push(
          `${line.itemCode || line.itemName || `item ${itemId}`}: returning ${returned}, invoiced ${line.invoicedQty}`,
        );
      }
    }
    return problems;
  }, [lines]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!invoiceId) {
      setError("Please select an invoice.");
      return;
    }
    if (lines.length === 0) {
      setError("At least one return line is required.");
      return;
    }
    if (overReturned.length > 0) {
      setError(`Return qty cannot exceed the invoiced qty — ${overReturned.join("; ")}.`);
      return;
    }

    setSaving(true);
    try {
      const payloadLines: CreditNoteLineInput[] = lines
        .filter((l) => l.qty > 0 && l.itemId)
        .map((l) => ({
          item_id: parseInt(l.itemId, 10),
          invoice_line_id: l.invoiceLineId,
          delivery_order_id: l.deliveryOrderId,
          qty: l.qty,
          unit_price_cents: l.unitPriceCents,
          unit_cost_cents: l.unitCostCents,
        }));
      if (payloadLines.length === 0) {
        setError("Enter a return quantity on at least one line.");
        setSaving(false);
        return;
      }
      const cn = await api.createCreditNote({
        invoice_id: parseInt(invoiceId, 10),
        customer_id: selectedInvoice?.customer_id ?? 0,
        cn_date: cnDate,
        refund_method: refundMethod as "deduct" | "refund",
        reason,
        lines: payloadLines,
      });
      workbench.replaceDraft(tabId, cn.number, cn.status);
      workbench.markUnsaved(tabId, false);
      setNumber(cn.number);
      setSavedId(cn.id);
      toast.success(`✓ Saved ${cn.number}`);
    } catch (err) {
      // ApiError carries the backend message on .message — read it safely
      // regardless of how the API layer throws.
      const message =
        typeof (err as { message?: unknown } | null | undefined)?.message === "string"
          ? (err as { message: string }).message
          : "Failed to create credit note.";
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <header className="entrytab__head">
        <span className="entrytab__number">{number}</span>
        <span className="kind-mark is-muted">Credit Note</span>
      </header>

      <div className="entrytab__body">
        <div className="entrytab__fields">
          <label className="field">
            <span className="field__label">CN Date *</span>
            <input type="date" className="input" value={cnDate} onChange={(e) => setCnDate(e.target.value)} required />
          </label>

          <label className="field">
            <span className="field__label">Invoice *</span>
            <select
              className="input"
              value={invoiceId}
              onChange={(e) => {
                const inv = invoices.find((i) => i.id === parseInt(e.target.value, 10));
                setInvoiceId(e.target.value);
                setSelectedInvoice(inv ?? null);
              }}
              required
            >
              <option value="">Choose invoice...</option>
              {invoices.map((inv) => (
                <option key={inv.id} value={inv.id}>
                  {inv.number} · {inv.customer_name ?? `Customer ${inv.customer_id}`} · {formatIDRFromCents(inv.receivable_cents)}
                </option>
              ))}
            </select>
          </label>

          <label className="field">
            <span className="field__label">Refund Method *</span>
            <select className="input" value={refundMethod} onChange={(e) => setRefundMethod(e.target.value)}>
              {REFUND_METHODS.map((m) => (
                <option key={m.value} value={m.value}>
                  {m.label}
                </option>
              ))}
            </select>
          </label>
        </div>

        <label className="field">
          <span className="field__label">Reason</span>
          <textarea
            className="input"
            rows={2}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Reason for return..."
          />
        </label>

        <div className="entrytab__detail">
          <div className="entrytab__detail-title">
            Returned Items *{loadingInvoice ? " — loading invoice lines..." : ""}
          </div>
          {overReturned.length > 0 && (
            <div className="field-warning" role="alert">
              Return qty exceeds invoiced qty: {overReturned.join("; ")}.
            </div>
          )}
          <div
            className="detail-grid detail-grid--cn"
            style={{ gridTemplateColumns: "2fr 0.8fr 0.8fr 1fr 1fr 1fr 1fr 32px" }}
          >
            <div className="detail-grid__head">
              <div>Item</div>
              <div className="right">Invoiced</div>
              <div>Qty</div>
              <div className="right">Unit Price</div>
              <div className="right">Unit Cost</div>
              <div className="right">Return Total</div>
              <div className="right">COGS Reversed</div>
              <div aria-hidden="true" />
            </div>
            {lines.map((line) => (
              <div className="detail-grid__row" key={line.id}>
                <div className="detail-grid__cell">
                  <StaticCombobox
                    value={line.itemId || null}
                    onChange={(_value, option) => setItem(line.id, option)}
                    options={itemOptions}
                    placeholder={invoiceId ? "Choose item..." : "Pick an invoice first"}
                    disabled={!invoiceId || invoiceLines.length === 0}
                  />
                </div>
                <div className="detail-grid__cell right">
                  <span className="ledger-table__amount">{line.invoicedQty || ""}</span>
                </div>
                <div className="detail-grid__cell">
                  <input
                    type="number"
                    className="input"
                    min={0}
                    max={line.invoicedQty || undefined}
                    step="0.001"
                    value={line.qty || ""}
                    onChange={(e) => setQty(line.id, parseFloat(e.target.value) || 0)}
                    placeholder="0"
                  />
                </div>
                <div className="detail-grid__cell right">
                  <input
                    type="text"
                    className="input"
                    inputMode="numeric"
                    value={formatCentsInput(line.unitPriceCents)}
                    onChange={(e) => setPrice(line.id, parseRupiahToCents(e.target.value))}
                  />
                </div>
                <div className="detail-grid__cell right">
                  <input
                    type="text"
                    className="input"
                    inputMode="numeric"
                    value={formatCentsInput(line.unitCostCents)}
                    onChange={(e) => setCost(line.id, parseRupiahToCents(e.target.value))}
                    placeholder="auto"
                  />
                </div>
                <div className="detail-grid__cell right">{formatIDRFromCents(line.lineTotalCents)}</div>
                <div className="detail-grid__cell right">{formatIDRFromCents(line.cogsReversedCents)}</div>
                <div className="detail-grid__action">
                  <button type="button" className="icon-btn" onClick={() => removeLine(line.id)} title="Remove line">
                    ×
                  </button>
                </div>
              </div>
            ))}
          </div>
          <button type="button" className="btn-link" onClick={addLine}>
            + Add line
          </button>
        </div>

        <div className="entrytab__summary">
          <div className="entrytab__total">
            <span className="entrytab__total-label">Return Total (Dr 4201)</span>
            <span className="entrytab__total-value">{formatIDRFromCents(totalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">COGS Reversed (Cr 5101)</span>
            <span className="entrytab__total-value">{formatIDRFromCents(totalCOGSReversed)}</span>
          </div>
          {savedId !== null && (
            <NextStepsBar number={number}>
              <button type="button" className="next-steps__btn" onClick={() => workbench.close(tabId)}>
                Close
              </button>
            </NextStepsBar>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {savedId === null && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <DiskIcon />
              <span>{saving ? "Saving..." : "Save CN"}</span>
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
    id: crypto.randomUUID(),
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 0,
    unitPriceCents: 0,
    unitCostCents: 0,
    lineTotalCents: 0,
    cogsReversedCents: 0,
    invoicedQty: 0,
  };
}

function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/** Cents -> whole-rupiah text for money inputs (input holds rupiah). */
function formatCentsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(Math.round(cents / 100));
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="currentColor" />
      <path d="M12 7v5l3 2" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}
