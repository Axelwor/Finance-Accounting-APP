import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { SupplierListItem, GoodsReceivedNoteListItem, Item, SupplierInvoiceLineInput } from "../../types";

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
  taxRate: number;
  lineTotalCents: number;
  vatCents: number;
}

export function SupplierInvoiceForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [dueDate, setDueDate] = useState("");
  const [number, setNumber] = useState(initialTitle ?? draftNumber("supplier-invoice-entry"));
  const [supplierId, setSupplierId] = useState("");
  const [grnId, setGrnId] = useState("");
  const [supplierInvoiceNumber, setSupplierInvoiceNumber] = useState("");
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [suppliers, setSuppliers] = useState<SupplierListItem[]>([]);
  const [grns, setGrNs] = useState<GoodsReceivedNoteListItem[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [invId, setInvId] = useState<number | null>(typeof entryId === "number" ? entryId : null);
  const [invStatus, setInvStatus] = useState("DRAFT");
  const [dpp, setDpp] = useState(0);
  const [vat, setVat] = useState(0);
  const [total, setTotal] = useState(0);
  const [payable, setPayable] = useState(0);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, supplierId, grnId, supplierInvoiceNumber, notes, lines, workbench]);

  useEffect(() => {
    void api.listSuppliers().then(setSuppliers);
    void api.listItems().then(setItems);
    void api.listGRNs("RECEIVED").then(setGrNs).catch(() => setGrNs([]));
  }, []);

  useEffect(() => {
    if (invId) {
      void loadInv(invId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invId]);

  const loadInv = async (id: number) => {
    try {
      const inv = await api.getSupplierInvoice(id);
      setNumber(inv.number);
      setInvStatus(inv.status);
      setDpp(inv.dpp_cents);
      setVat(inv.vat_cents);
      setTotal(inv.total_cents);
      setPayable(inv.payable_cents);
      setDate(inv.invoice_date);
      setDueDate(inv.due_date ?? "");
      setSupplierId(String(inv.supplier_id));
      setGrnId(inv.grn_id ? String(inv.grn_id) : "");
      setSupplierInvoiceNumber(inv.supplier_invoice_number ?? "");
      setNotes(inv.notes ?? "");
      setLines(
        inv.lines.map((l) => ({
          id: `ln-${l.id}`,
          itemId: String(l.item_id),
          itemCode: l.item_code ?? "",
          itemName: l.item_name ?? "",
          qty: Number(l.qty),
          unitPriceCents: l.unit_price_cents,
          discountCents: l.discount_cents,
          taxRate: l.tax_rate,
          lineTotalCents: l.line_total_cents,
          vatCents: Math.round((l.line_total_cents * l.tax_rate) / 100),
        })),
      );
      workbench.markUnsaved(tabId, false);
    } catch {
      // new invoice or fetch failed
    }
  };

  const computedDpp = useMemo(
    () => lines.reduce((sum, l) => sum + l.lineTotalCents, 0),
    [lines],
  );
  const computedVat = useMemo(
    () => lines.reduce((sum, l) => sum + l.vatCents, 0),
    [lines],
  );
  const computedTotal = computedDpp + computedVat;
  const computedPayable = computedTotal; // no DP realization yet

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
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              qty,
              lineTotalCents: lineTotal(qty, l.unitPriceCents, l.discountCents),
              vatCents: vatForLine(lineTotal(qty, l.unitPriceCents, l.discountCents), l.taxRate),
            }
          : l,
      ),
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              unitPriceCents,
              lineTotalCents: lineTotal(l.qty, unitPriceCents, l.discountCents),
              vatCents: vatForLine(lineTotal(l.qty, unitPriceCents, l.discountCents), l.taxRate),
            }
          : l,
      ),
    );
  };

  const setDiscount = (id: string, discountCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              discountCents,
              lineTotalCents: lineTotal(l.qty, l.unitPriceCents, discountCents),
              vatCents: vatForLine(lineTotal(l.qty, l.unitPriceCents, discountCents), l.taxRate),
            }
          : l,
      ),
    );
  };

  const setTaxRate = (id: string, taxRate: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              taxRate,
              vatCents: vatForLine(l.lineTotalCents, taxRate),
            }
          : l,
      ),
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!supplierId) {
      setError("Pick a supplier for this invoice.");
      return;
    }
    const payloadLines: SupplierInvoiceLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: l.qty > 0 ? l.qty : 1,
        unit_price_cents: l.unitPriceCents,
        discount_cents: l.discountCents,
        tax_rate: l.taxRate,
        description: undefined,
      }));
    if (payloadLines.length === 0) {
      setError("Add at least one item line.");
      return;
    }
    setSaving(true);
    try {
      const created = await api.createSupplierInvoice({
        supplier_id: Number(supplierId),
        grn_id: grnId ? Number(grnId) : undefined,
        invoice_date: date,
        due_date: dueDate || undefined,
        supplier_invoice_number: supplierInvoiceNumber.trim() || undefined,
        notes: notes.trim() || undefined,
        lines: payloadLines,
      });
      setInvId(created.id);
      setInvStatus(created.status);
      setDpp(created.dpp_cents);
      setVat(created.vat_cents);
      setTotal(created.total_cents);
      setPayable(created.payable_cents);
      setNumber(created.number);
      workbench.replaceDraft(tabId, created.number, created.status);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the supplier invoice.");
    } finally {
      setSaving(false);
    }
  };

  const isExisting = invId !== null;

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Supplier Invoice</span>
          <span className={`entrytab__status ${isExisting ? "" : "entrytab__status--draft"}`}>
            {isExisting ? invStatus : "DRAFT"}
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
                <span className="field__label">Supplier</span>
                <select className="input" value={supplierId} onChange={(e) => setSupplierId(e.target.value)} disabled={isExisting}>
                  <option value="">Choose supplier...</option>
                  {suppliers.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.code} · {s.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Invoice Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} disabled={isExisting} />
              </label>
              <label className="field">
                <span className="field__label">Due Date</span>
                <input className="input" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} disabled={isExisting} />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">No</span>
                <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">GRN (optional)</span>
                <select className="input" value={grnId} onChange={(e) => setGrnId(e.target.value)} disabled={isExisting}>
                  <option value="">(none)</option>
                  {grns.map((g) => (
                    <option key={g.id} value={g.id}>
                      {g.number}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Supplier Invoice No</span>
                <input className="input" value={supplierInvoiceNumber} onChange={(e) => setSupplierInvoiceNumber(e.target.value)} placeholder="INV-SUP-001" disabled={isExisting} />
              </label>
            </div>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Tax notes, NPWP, references..." disabled={isExisting} />
          </label>

          <div className="entrytab__detail">
            <div className="entrytab__detail-title">Item lines *</div>
            <div className="detail-grid detail-grid--quote">
              <div className="detail-grid__head">
                <div>Item</div>
                <div>Qty</div>
                <div className="right">Unit Price</div>
                <div className="right">Discount</div>
                <div className="right">Tax %</div>
                <div className="right">VAT</div>
                <div className="right">Line Total</div>
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
                    <input className="amount right" type="text" inputMode="numeric" value={centsInput(line.unitPriceCents)} onChange={(e) => setPrice(line.id, parseCents(e.target.value))} placeholder="0" disabled={isExisting} />
                  </div>
                  <div>
                    <input className="amount right" type="text" inputMode="numeric" value={centsInput(line.discountCents)} onChange={(e) => setDiscount(line.id, parseCents(e.target.value))} placeholder="0" disabled={isExisting} />
                  </div>
                  <div>
                    <input className="amount right" type="number" min={0} max={100} step="any" value={line.taxRate || ""} onChange={(e) => setTaxRate(line.id, Number(e.target.value))} placeholder="0" disabled={isExisting} />
                  </div>
                  <div className="right">
                    <span className="ledger-table__amount">{formatIDR(line.vatCents)}</span>
                  </div>
                  <div className="right">
                    <span className="ledger-table__amount">{formatIDR(line.lineTotalCents)}</span>
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
                  <div />
                  <div />
                  <div />
                </div>
              )}
            </div>
          </div>

          <div className="entrytab__total">
            <span className="entrytab__total-label">DPP (Dasar Pengenaan Pajak)</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? dpp : computedDpp)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">VAT (PPN Masukan)</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? vat : computedVat)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--accent)", paddingTop: 8 }}>
            <span className="entrytab__total-label">Total</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? total : computedTotal)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--accent)", paddingTop: 8 }}>
            <span className="entrytab__total-label">Payable (Cr 2101 AP)</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? payable : computedPayable)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Post Invoice"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round((qty > 0 ? qty : 1) * unitPriceCents) - discountCents;
}

function vatForLine(lineTotalCents: number, taxRate: number): number {
  if (taxRate <= 0) return 0;
  return Math.round((lineTotalCents * taxRate) / 100);
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
    taxRate: 0,
    lineTotalCents: 0,
    vatCents: 0,
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
