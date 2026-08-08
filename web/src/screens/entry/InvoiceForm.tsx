import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Customer, Item, InvoiceLineInput, SalesOrderListItem, InvoicePayment, CreatePaymentInput } from "../../types";

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

export function InvoiceForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [dueDate, setDueDate] = useState("");
  const [number, setNumber] = useState(initialTitle ?? draftNumber("sales-invoice"));
  const [customerId, setCustomerId] = useState("");
  const [salesOrderId, setSalesOrderId] = useState("");
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [salesOrders, setSalesOrders] = useState<SalesOrderListItem[]>([]);
  const [cashAccounts, setCashAccounts] = useState<{ id: number; name: string }[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [invId, setInvId] = useState<number | null>(typeof entryId === "number" ? entryId : null);
  const [invStatus, setInvStatus] = useState("DRAFT");
  const [total, setTotal] = useState(0);
  const [dpApplied, setDpApplied] = useState(0);
  const [receivable, setReceivable] = useState(0);
  const [payments, setPayments] = useState<InvoicePayment[]>([]);
  const [payAmount, setPayAmount] = useState(0);
  const [payDate, setPayDate] = useState(todayISO());
  const [payCashAccount, setPayCashAccount] = useState(0);
  const [payDesc, setPayDesc] = useState("");
  const [payError, setPayError] = useState<string | null>(null);
  const [postingPay, setPostingPay] = useState(false);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, customerId, salesOrderId, notes, lines, workbench]);

  useEffect(() => {
    void api.listCustomers().then(setCustomers);
    void api.listItems().then(setItems);
    void api.listSalesOrders("CLOSED").then(setSalesOrders);
    void api.listAccounts().then((accs) => {
      setCashAccounts(accs.map((a) => ({ id: Number(a.id), name: a.name })));
      if (accs.length > 0) setPayCashAccount(Number(accs[0].id));
    });
  }, []);

  useEffect(() => {
    if (invId) {
      void loadInv(invId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invId]);

  const loadInv = async (id: number) => {
    try {
      const inv = await api.getInvoice(id);
      setNumber(inv.number);
      setInvStatus(inv.status);
      setTotal(inv.total_cents);
      setDpApplied(inv.dp_applied_cents);
      setReceivable(inv.receivable_cents);
      setDate(inv.invoice_date);
      setDueDate(inv.due_date ?? "");
      setCustomerId(String(inv.customer_id));
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
          lineTotalCents: l.line_total_cents,
        })),
      );
      // Load payments for this invoice.
      try {
        const pmts = await api.listInvoicePayments(id);
        setPayments(pmts);
      } catch {
        setPayments([]);
      }
      workbench.markUnsaved(tabId, false);
    } catch {
      // new invoice or fetch failed
    }
  };

  const totalCents = useMemo(() => lines.reduce((sum, l) => sum + l.lineTotalCents, 0), [lines]);

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
      cur.map((l) => (l.id === id ? { ...l, qty, lineTotalCents: lineTotal(qty, l.unitPriceCents, l.discountCents) } : l)),
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) => (l.id === id ? { ...l, unitPriceCents, lineTotalCents: lineTotal(l.qty, unitPriceCents, l.discountCents) } : l)),
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
      setError("Pick a customer for this invoice.");
      return;
    }
    const payloadLines: InvoiceLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: l.qty > 0 ? l.qty : 1,
        unit_price_cents: l.unitPriceCents,
        discount_cents: l.discountCents,
        tax_rate: 0,
        description: undefined,
      }));
    if (payloadLines.length === 0) {
      setError("Add at least one item line.");
      return;
    }
    setSaving(true);
    try {
      const created = await api.createInvoice({
        customer_id: Number(customerId),
        sales_order_id: salesOrderId ? Number(salesOrderId) : undefined,
        invoice_date: date,
        due_date: dueDate || undefined,
        notes: notes.trim() || undefined,
        lines: payloadLines,
      });
      setInvId(created.id);
      setInvStatus(created.status);
      setTotal(created.total_cents);
      setDpApplied(created.dp_applied_cents);
      setReceivable(created.receivable_cents);
      setNumber(created.number);
      workbench.replaceDraft(tabId, created.number, created.status);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the invoice.");
    } finally {
      setSaving(false);
    }
  };

  const handlePostPayment = async () => {
    if (!invId) return;
    setPayError(null);
    if (payAmount <= 0) {
      setPayError("Payment amount must be greater than zero.");
      return;
    }
    if (!payCashAccount) {
      setPayError("Pick a cash/bank account.");
      return;
    }
    setPostingPay(true);
    try {
      await api.createInvoicePayment(invId, {
        cash_account_id: payCashAccount,
        amount_cents: payAmount,
        payment_date: payDate,
        description: payDesc.trim() || undefined,
      });
      await loadInv(invId);
      setPayAmount(0);
      setPayDesc("");
    } catch (err) {
      setPayError(err instanceof Error ? err.message : "Could not post the payment.");
    } finally {
      setPostingPay(false);
    }
  };

  const isExisting = invId !== null;

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Sales Invoice</span>
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
                <span className="field__label">Customer</span>
                <select className="input" value={customerId} onChange={(e) => setCustomerId(e.target.value)} disabled={isExisting}>
                  <option value="">Choose customer...</option>
                  {customers.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.code} · {c.name}
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
                <span className="field__label">Sales Order</span>
                <select className="input" value={salesOrderId} onChange={(e) => setSalesOrderId(e.target.value)} disabled={isExisting}>
                  <option value="">(none)</option>
                  {salesOrders.map((so) => (
                    <option key={so.id} value={so.id}>
                      {so.number}
                    </option>
                  ))}
                </select>
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
                </div>
              )}
            </div>
          </div>

          <div className="entrytab__total">
            <span className="entrytab__total-label">Total</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? total : totalCents)}</span>
          </div>

          {isExisting && dpApplied > 0 && (
            <div className="entrytab__total" style={{ marginTop: 8 }}>
              <span className="entrytab__total-label">DP Applied</span>
              <span className="entrytab__total-value">{formatIDR(dpApplied)}</span>
            </div>
          )}
          {isExisting && (
            <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--accent)", paddingTop: 8 }}>
              <span className="entrytab__total-label">Receivable</span>
              <span className="entrytab__total-value">{formatIDR(receivable)}</span>
            </div>
          )}

          {isExisting && (
            <div style={{ marginTop: 16, borderTop: "2px solid var(--accent)", paddingTop: 12 }}>
              <div className="entrytab__detail-title" style={{ marginBottom: 8 }}>
                Payments — Received: <strong>{formatIDR(payments.reduce((s, p) => s + p.ar_applied_cents, 0))}</strong> / Receivable: <strong>{formatIDR(receivable)}</strong>
              </div>

              {payments.length > 0 && (
                <div className="ledger-table" style={{ marginBottom: 12 }}>
                  <div className="ledger-table__head">
                    <span>Number</span>
                    <span>Date</span>
                    <span className="right">Amount</span>
                    <span className="right">AR Applied</span>
                    <span>Status</span>
                  </div>
                  {payments.map((pmt) => (
                    <div className="ledger-table__row" key={pmt.id}>
                      <span className="ledger-table__no">{pmt.number}</span>
                      <span className="ledger-table__date">{pmt.payment_date}</span>
                      <span className="ledger-table__amount right">{formatIDR(pmt.amount_cents)}</span>
                      <span className="ledger-table__amount right">{formatIDR(pmt.ar_applied_cents)}</span>
                      <span>
                        <span className={`kind-mark ${pmt.status === "RECEIVED" ? "is-positive" : "is-negative"}`}>{pmt.status}</span>
                      </span>
                    </div>
                  ))}
                </div>
              )}

              {receivable > 0 && invStatus !== "VOID" && (
                <div className="detail-grid detail-grid--quote" style={{ gridTemplateColumns: "1fr 1fr 2fr" }}>
                  <div className="field">
                    <span className="field__label">Amount</span>
                    <input className="amount input" type="text" inputMode="numeric" value={centsInput(payAmount)} onChange={(e) => setPayAmount(parseCents(e.target.value))} placeholder="0" />
                  </div>
                  <div className="field">
                    <span className="field__label">Cash/Bank</span>
                    <select className="input" value={payCashAccount} onChange={(e) => setPayCashAccount(Number(e.target.value))}>
                      {cashAccounts.map((a) => (
                        <option key={a.id} value={a.id}>{a.name}</option>
                      ))}
                    </select>
                  </div>
                  <div className="field">
                    <span className="field__label">Description</span>
                    <input className="input" type="text" value={payDesc} onChange={(e) => setPayDesc(e.target.value)} placeholder="Payment reference..." />
                  </div>
                  <div className="field">
                    <span className="field__label">Payment Date</span>
                    <input className="input" type="date" value={payDate} onChange={(e) => setPayDate(e.target.value)} />
                  </div>
                  <div />
                  <div style={{ display: "flex", alignItems: "flex-end" }}>
                    <button type="button" className="btn btn--primary btn--sm" onClick={() => void handlePostPayment()} disabled={postingPay}>
                      {postingPay ? "Posting..." : "Receive Payment"}
                    </button>
                  </div>
                </div>
              )}
              <FormError message={payError} />
            </div>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving} title="Create invoice (posts revenue + DP realization)">
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
