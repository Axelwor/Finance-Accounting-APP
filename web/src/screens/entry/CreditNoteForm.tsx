import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { InvoiceListItem, CreditNoteLineInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
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
}

const REFUND_METHODS: { value: string; label: string }[] = [
  { value: "deduct", label: "Deduct from AR" },
  { value: "refund", label: "Cash refund" },
];

/**
 * Credit Note (CN) entry form. Posts a return journal:
 *  - Dr 4201 Sales Returns / Cr 1201 AR (or Cash for refund method)
 *  - Dr 1301 Inventory / Cr 5101 COGS (reverse cost of returned goods)
 */
export function CreditNoteForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [cnDate, setCnDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("credit-note-entry"));
  const [invoiceId, setInvoiceId] = useState("");
  const [refundMethod, setRefundMethod] = useState("deduct");
  const [reason, setReason] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [invoices, setInvoices] = useState<InvoiceListItem[]>([]);
  const [selectedInvoice, setSelectedInvoice] = useState<InvoiceListItem | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, cnDate, number, invoiceId, refundMethod, reason, lines, workbench]);

  useEffect(() => {
    void api.listInvoices().then((list) => {
      // Only show ISSUED or PARTIALLY_PAID invoices — can't return what hasn't been invoiced.
      setInvoices(list.filter((i) => i.status === "ISSUED" || i.status === "PARTIALLY_PAID"));
    });
  }, []);

  const totalCents = useMemo(() => lines.reduce((s, l) => s + l.lineTotalCents, 0), [lines]);
  const totalCOGSReversed = useMemo(() => lines.reduce((s, l) => s + l.cogsReversedCents, 0), [lines]);

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              qty,
              lineTotalCents: l.qty > 0 ? Math.round(qty * l.unitPriceCents) : 0,
              cogsReversedCents: l.qty > 0 ? Math.round(qty * l.unitCostCents) : 0,
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

  const addLine = () => setLines((cur) => [...cur, seedLine()]);
  const removeLine = (id: string) => setLines((cur) => (cur.length > 1 ? cur.filter((l) => l.id !== id) : cur));

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

    setSaving(true);
    try {
      const payload = {
        invoice_id: parseInt(invoiceId, 10),
        customer_id: selectedInvoice?.customer_id ?? 0,
        cn_date: cnDate,
        refund_method: refundMethod as "deduct" | "refund",
        reason,
        lines: lines
          .filter((l) => l.qty > 0)
          .map((l) => ({
            item_id: parseInt(l.itemId, 10),
            qty: l.qty,
            unit_price_cents: l.unitPriceCents,
            unit_cost_cents: l.unitCostCents,
          })),
      };
      const cn = await api.createCreditNote(payload);
      workbench.replaceDraft(tabId, cn.number, cn.status);
      workbench.markUnsaved(tabId, false);
      setNumber(cn.number);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create credit note.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="entry-tab" onSubmit={handleSubmit}>
      <header className="entry-tab__header">
        <span className="entry-tab__number">{number}</span>
        <span className="kind-mark is-muted">Credit Note</span>
      </header>

      <div className="entry-tab__body">
        <div className="entry-tab__fields">
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
                  {inv.number} · {inv.customer_name ?? `Customer ${inv.customer_id}`} · {formatIDR(inv.receivable_cents)}
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
          <div className="entrytab__detail-title">Returned Items *</div>
          <div className="detail-grid detail-grid--cn">
            <div className="detail-grid__head">
              <div>Item</div>
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
                  <input
                    className="input"
                    placeholder="Item ID"
                    value={line.itemId}
                    onChange={(e) => {
                      const v = e.target.value;
                      setLines((cur) => cur.map((l) => (l.id === line.id ? { ...l, itemId: v } : l)));
                    }}
                  />
                </div>
                <div className="detail-grid__cell">
                  <input
                    type="number"
                    className="input"
                    min={0}
                    step="0.001"
                    value={line.qty || ""}
                    onChange={(e) => setQty(line.id, parseFloat(e.target.value) || 0)}
                  />
                </div>
                <div className="detail-grid__cell right">
                  <input
                    type="text"
                    className="input"
                    value={formatCentsInput(line.unitPriceCents)}
                    onChange={(e) => setPrice(line.id, parseCents(e.target.value))}
                  />
                </div>
                <div className="detail-grid__cell right">
                  <input
                    type="text"
                    className="input"
                    value={formatCentsInput(line.unitCostCents)}
                    onChange={(e) => setCost(line.id, parseCents(e.target.value))}
                  />
                </div>
                <div className="detail-grid__cell right">{formatIDR(line.lineTotalCents)}</div>
                <div className="detail-grid__cell right">{formatIDR(line.cogsReversedCents)}</div>
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

        <div className="entry-tab__summary">
          <div className="entrytab__total">
            <span className="entrytab__total-label">Return Total (Dr 4201)</span>
            <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">COGS Reversed (Cr 5101)</span>
            <span className="entrytab__total-value">{formatIDR(totalCOGSReversed)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            <DiskIcon />
            <span>{saving ? "Saving..." : "Save CN"}</span>
          </button>
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
  };
}

function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function formatCentsInput(cents: number): string {
  if (!cents) return "";
  return String(cents);
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="currentColor" />
      <path d="M12 7v5l3 2" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}
