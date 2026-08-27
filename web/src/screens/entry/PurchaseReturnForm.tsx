import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { SupplierInvoiceListItem, PurchaseReturnLineInput } from "../../types";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface Line {
  id: string;
  itemId: string;
  qty: number;
  unitPriceCents: number;
  lineTotalCents: number;
}

const REFUND_METHODS: { value: string; label: string }[] = [
  { value: "deduct", label: "Deduct from AP" },
  { value: "refund", label: "Cash refund" },
  { value: "credit_balance", label: "Credit balance" },
];

/**
 * Purchase Return (Retur Pembelian) entry form. Posts a return journal:
 *  - Dr 2101 Accounts Payable (total + vat_reversed) — AP goes back up
 *  - Cr 1301 Inventory (total) — reduce inventory
 *  - Cr 1203 Input VAT (vat_reversed) — reverse input VAT
 * No COGS reversal (purchase returns don't reverse COGS).
 */
export function PurchaseReturnForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [returnDate, setReturnDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("purchase-return-entry"));
  const [invoiceId, setInvoiceId] = useState("");
  const [refundMethod, setRefundMethod] = useState("deduct");
  const [reason, setReason] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [invoices, setInvoices] = useState<SupplierInvoiceListItem[]>([]);
  const [selectedInvoice, setSelectedInvoice] = useState<SupplierInvoiceListItem | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, returnDate, number, invoiceId, refundMethod, reason, lines, workbench]);

  useEffect(() => {
    void api.listSupplierInvoices().then((list) => {
      // Only show ISSUED or PARTIALLY_PAID invoices — can't return what isn't owed.
      setInvoices(list.filter((i) => i.status === "ISSUED" || i.status === "PARTIALLY_PAID"));
    });
  }, []);

  const totalCents = useMemo(() => lines.reduce((s, l) => s + l.lineTotalCents, 0), [lines]);

  // VAT reversal: input VAT rate is 11% (Indonesian PPN). Reversed on the returned DPP.
  const vatReversedCents = useMemo(() => {
    if (!selectedInvoice || selectedInvoice.dpp_cents <= 0) return 0;
    const rate = selectedInvoice.vat_cents / selectedInvoice.dpp_cents;
    if (rate <= 0) return 0;
    return Math.round(totalCents * rate);
  }, [totalCents, selectedInvoice]);

  const apDeductedCents = totalCents + vatReversedCents;

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, qty, lineTotalCents: l.qty > 0 ? Math.round(qty * l.unitPriceCents) : 0 }
          : l,
      ),
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, unitPriceCents, lineTotalCents: l.qty > 0 ? Math.round(l.qty * unitPriceCents) : 0 } : l)),
    );
  };

  const addLine = () => setLines((cur) => [...cur, seedLine()]);
  const removeLine = (id: string) => setLines((cur) => (cur.length > 1 ? cur.filter((l) => l.id !== id) : cur));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!invoiceId) {
      setError("Please select a supplier invoice.");
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
        supplier_id: selectedInvoice?.supplier_id ?? 0,
        return_date: returnDate,
        refund_method: refundMethod as "deduct" | "refund" | "credit_balance",
        reason,
        lines: lines
          .filter((l) => l.qty > 0)
          .map((l): PurchaseReturnLineInput => ({
            item_id: parseInt(l.itemId, 10),
            qty: l.qty,
            unit_price_cents: l.unitPriceCents,
          })),
      };
      const pr = await api.createPurchaseReturn(payload);
      workbench.replaceDraft(tabId, pr.number, pr.status);
      workbench.markUnsaved(tabId, false);
      setNumber(pr.number);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create purchase return.");
    } finally {
      setSaving(false);
    }
  };

  if (entryId) {
    return (
      <div className="entrytab">
        <header className="entrytab__head">
          <span className="entrytab__number">{number}</span>
          <span className="kind-mark is-muted">Purchase Return</span>
        </header>
        <div className="entrytab__body">
          <p className="tab-placeholder__sub">View existing purchase return (read-only).</p>
        </div>
      </div>
    );
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <header className="entrytab__head">
        <span className="entrytab__number">{number}</span>
        <span className="kind-mark is-muted">Purchase Return</span>
      </header>

      <div className="entrytab__body">
        <div className="entrytab__fields">
          <label className="field">
            <span className="field__label">Return Date *</span>
            <input type="date" className="input" value={returnDate} onChange={(e) => setReturnDate(e.target.value)} required />
          </label>

          <label className="field">
            <span className="field__label">Supplier Invoice *</span>
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
                  {inv.number} · {inv.supplier_name ?? `Supplier ${inv.supplier_id}`} · {formatIDR(inv.payable_cents)}
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
              <div className="right">Line Total</div>
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
                <div className="detail-grid__cell right">{formatIDR(line.lineTotalCents)}</div>
                <div className="detail-grid__cell">
                  <Button
                    variant="text"
                    size="sm"
                    onClick={() => removeLine(line.id)}
                    aria-label="Remove line"
                  >
                    ×
                  </Button>
                </div>
              </div>
            ))}
          </div>
          <Button
            variant="text"
            onClick={addLine}
            style={{ marginTop: 8 }}
          >
            + Add line
          </Button>
        </div>

        <div className="entrytab__totals">
          <div className="entrytab__total">
            <span className="entrytab__total-label">Return Total (Cr 1301 Inventory)</span>
            <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">VAT Reversed (Cr 1203 Input VAT)</span>
            <span className="entrytab__total-value">{formatIDR(vatReversedCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">AP Increase (Dr 2101 Accounts Payable)</span>
            <span className="entrytab__total-value">{formatIDR(apDeductedCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            <DiskIcon />
            <span>{saving ? "Saving..." : "Save Return"}</span>
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
    qty: 0,
    unitPriceCents: 0,
    lineTotalCents: 0,
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
