import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import type { Customer, Item, QuotationLineInput } from "../../types";

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
  discountCents: number;
  lineTotalCents: number;
}

/**
 * Quotation (SQ) entry form. SQ is a commitment — it does NOT post any
 * journal. On save it POSTs /quotations with customers + items picked from
 * the live backend.
 */
export function QuotationForm({ tabId, entryId, initialTitle }: Props) {
  const { markUnsaved, replaceDraft } = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("sales-quotation-entry"));
  const [customerId, setCustomerId] = useState("");
  const [validUntil, setValidUntil] = useState("");
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [taxRate, setTaxRate] = useState(0);

  useEffect(() => {
    markUnsaved(tabId, true);
  }, [tabId, date, number, customerId, validUntil, notes, lines, taxRate, markUnsaved]);

  useEffect(() => {
    void api.listCustomers().then(setCustomers);
    void api.listItems().then(setItems);
  }, []);

  const subtotalCents = useMemo(() => lines.reduce((sum, l) => sum + l.lineTotalCents, 0), [lines]);
  const ppnCents = useMemo(
    () => lines.reduce((sum, l) => sum + taxForLine(l.lineTotalCents, taxRate), 0),
    [lines, taxRate],
  );
  const totalCents = subtotalCents + ppnCents;

  const setItem = (id: string, itemId: string) => {
    const item = items.find((i) => String(i.id) === itemId);
    setLines((cur) =>
      cur.map((l) =>
        l.id === id ? { ...l, itemId, itemCode: item?.code ?? "", itemName: item?.name ?? "" } : l,
      ),
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, unitPriceCents, lineTotalCents: lineTotal(l.qty, unitPriceCents, l.discountCents) } : l)),
    );
  };

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, qty, lineTotalCents: lineTotal(qty, l.unitPriceCents, l.discountCents) } : l)),
    );
  };

  const setDiscount = (id: string, discountCents: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, discountCents, lineTotalCents: lineTotal(l.qty, l.unitPriceCents, discountCents) } : l)),
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!customerId) {
      setError("Pick a customer for this quotation.");
      return;
    }
    const payloadLines: QuotationLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: l.qty > 0 ? l.qty : 1,
        unit_price_cents: l.unitPriceCents,
        discount_cents: l.discountCents,
        tax_rate: taxRate,
        description: undefined,
      }));
    if (payloadLines.length === 0) {
      setError("Add at least one item line.");
      return;
    }
    setSaving(true);
    try {
      const created = await api.createQuotation({
        customer_id: Number(customerId),
        quotation_date: date,
        valid_until: validUntil || undefined,
        notes: notes.trim() || undefined,
        lines: payloadLines,
      });
      replaceDraft(tabId, created.number, "DRAFT");
      markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save the quotation.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Quotation</span>
          <span className="entrytab__status entrytab__status--draft">DRAFT</span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDateID(date)}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          <div className="entrytab__header-grid">
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">Customer</span>
                <select className="input" value={customerId} onChange={(e) => setCustomerId(e.target.value)}>
                  <option value="">Choose customer...</option>
                  {customers.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.code} · {c.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Valid until</span>
                <input className="input" type="date" value={validUntil} onChange={(e) => setValidUntil(e.target.value)} />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">No</span>
                <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
              </label>
              <TaxRateSelector value={taxRate} onChange={setTaxRate} />
            </div>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Terms, validity, references..." />
          </label>

          <div className="entrytab__detail">
            <div className="entrytab__detail-title">Item lines *</div>
            <div className="detail-grid detail-grid--quote">
              <div className="detail-grid__head">
                <div>Item</div>
                <div>Qty</div>
                <div className="right">Unit Price</div>
                <div className="right">Discount</div>
                <div className="right">Line Total</div>
                <div aria-hidden="true" />
              </div>
              {lines.map((line) => (
                <div className="detail-grid__row" key={line.id}>
                  <div>
                    <select className="input" value={line.itemId} onChange={(e) => setItem(line.id, e.target.value)}>
                      <option value="">Choose item...</option>
                      {items.map((i) => (
                        <option key={i.id} value={i.id}>
                          {i.code} · {i.name}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <input
                      className="amount"
                      type="number"
                      min={1}
                      step="any"
                      value={line.qty || ""}
                      onChange={(e) => setQty(line.id, Number(e.target.value))}
                      placeholder="1"
                    />
                  </div>
                  <div>
                    <input
                      className="amount right"
                      type="text"
                      inputMode="numeric"
                      value={centsInput(line.unitPriceCents)}
                      onChange={(e) => setPrice(line.id, parseCents(e.target.value))}
                      placeholder="0"
                    />
                  </div>
                  <div>
                    <input
                      className="amount right"
                      type="text"
                      inputMode="numeric"
                      value={centsInput(line.discountCents)}
                      onChange={(e) => setDiscount(line.id, parseCents(e.target.value))}
                      placeholder="0"
                    />
                  </div>
                  <div className="right">
                    <span className="ledger-table__amount">{formatIDR(line.lineTotalCents)}</span>
                  </div>
                  <div>
                    <button
                      type="button"
                      className="detail-grid__remove"
                      onClick={() => setLines((cur) => (cur.length > 1 ? cur.filter((l) => l.id !== line.id) : cur))}
                      aria-label="Remove line"
                      disabled={lines.length === 1}
                    >
                      ×
                    </button>
                  </div>
                </div>
              ))}
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
                <div />
              </div>
            </div>
          </div>

          <div className="entrytab__total">
            <span className="entrytab__total-label">DPP (Subtotal)</span>
            <span className="entrytab__total-value">{formatIDR(subtotalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">PPN {taxRate > 0 ? `(${taxRate}%)` : ""}</span>
            <span className="entrytab__total-value">{formatIDR(ppnCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--accent)", paddingTop: 8 }}>
            <span className="entrytab__total-label">Total</span>
            <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving} title="Save quotation (posts no journal)">
            <DiskIcon />
            <span>{saving ? "Saving..." : "Save"}</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="Send to customer after saving">
            <PlaneIcon />
            <span>Send</span>
          </button>
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round((qty > 0 ? qty : 1) * unitPriceCents) - discountCents;
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function centsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(cents);
}

function seedLine(): Line {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 1,
    unitPriceCents: 0,
    discountCents: 0,
    lineTotalCents: 0,
  };
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
function PlaneIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M2 12l20-8-6 18-4-7-8-3z" fill="currentColor" />
      <path d="M12 15l10-11" stroke="#fff" strokeWidth="1.2" />
    </svg>
  );
}