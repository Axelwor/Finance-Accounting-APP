import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import type { Customer, Item, SalesOrderLineInput, DownPayment, SalesOrder } from "../../types";

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

export function SalesOrderForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber("sales-order-entry"));
  const [customerId, setCustomerId] = useState("");
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [cashAccounts, setCashAccounts] = useState<{ id: number; name: string }[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [orderId, setOrderId] = useState<number | null>(
    typeof entryId === "number" ? entryId : null,
  );
  const [orderStatus, setOrderStatus] = useState<string>(initialTitle ? "" : "DRAFT");
  const [orderTotal, setOrderTotal] = useState(0);
  const [dpReceived, setDpReceived] = useState(0);
  const [downPayments, setDownPayments] = useState<DownPayment[]>([]);
  const [dpAmount, setDpAmount] = useState(0);
  const [dpDate, setDpDate] = useState(todayISO());
  const [dpCashAccount, setDpCashAccount] = useState(0);
  const [dpDesc, setDpDesc] = useState("");
  const [dpError, setDpError] = useState<string | null>(null);
  const [postingDP, setPostingDP] = useState(false);
  const [customerPONumber, setCustomerPONumber] = useState("");
  const [customerPODate, setCustomerPODate] = useState("");
  const [requestedDeliveryDate, setRequestedDeliveryDate] = useState("");
  const [shippingTerms, setShippingTerms] = useState<SalesOrder["shipping_terms"] | undefined>(undefined);
  const [shipToAddress, setShipToAddress] = useState("");
  const [salespersonId, setSalespersonId] = useState("");
  const [taxRate, setTaxRate] = useState(0);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, customerId, notes, lines, taxRate, workbench]);

  useEffect(() => {
    void api.listCustomers().then(setCustomers);
    void api.listItems().then(setItems);
    void api.listAccounts().then((accs) => {
      setCashAccounts(accs.map((a) => ({ id: Number(a.id), name: a.name })));
      if (accs.length > 0) setDpCashAccount(Number(accs[0].id));
    });
  }, []);

  useEffect(() => {
    if (orderId) {
      void loadOrder(orderId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orderId]);

  const loadOrder = async (id: number) => {
    try {
      const order = await api.getSalesOrder(id);
      setNumber(order.number);
      setOrderStatus(order.status);
      setOrderTotal(order.total_cents);
      setDpReceived(order.dp_received_cents);
      setDownPayments(order.down_payments ?? []);
      setDate(order.order_date);
      setCustomerId(String(order.customer_id));
      setNotes(order.notes ?? "");
      setCustomerPONumber(order.customer_po_number ?? "");
      setCustomerPODate(order.customer_po_date ?? "");
      setRequestedDeliveryDate(order.requested_delivery_date ?? "");
      setShippingTerms(order.shipping_terms);
      setShipToAddress(order.ship_to_address ?? "");
      setSalespersonId(order.salesperson_id ? String(order.salesperson_id) : "");
      const firstRate = order.lines.find((l) => Number(l.tax_rate) > 0);
      setTaxRate(firstRate ? Number(firstRate.tax_rate) : Number(order.lines[0]?.tax_rate ?? 0) || 0);
      setLines(
        order.lines.map((l) => ({
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
      workbench.markUnsaved(tabId, false);
    } catch {
      // new order or fetch failed
    }
  };

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
      setError("Pick a customer for this order.");
      return;
    }
    const payloadLines: SalesOrderLineInput[] = lines
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
      const created = await api.createSalesOrder({
        customer_id: Number(customerId),
        order_date: date,
        notes: notes.trim() || undefined,
        customer_po_number: customerPONumber.trim() || undefined,
        customer_po_date: customerPODate || undefined,
        requested_delivery_date: requestedDeliveryDate || undefined,
        salesperson_id: salespersonId ? Number(salespersonId) : undefined,
        ship_to_address: shipToAddress.trim() || undefined,
        shipping_terms: shippingTerms,
        lines: payloadLines,
      });
      setOrderId(created.id);
      setOrderStatus(created.status);
      setOrderTotal(created.total_cents);
      setNumber(created.number);
      workbench.replaceDraft(tabId, created.number, created.status);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save the sales order.");
    } finally {
      setSaving(false);
    }
  };

  const handlePostDP = async () => {
    if (!orderId) return;
    setDpError(null);
    if (dpAmount <= 0) {
      setDpError("Down payment amount must be greater than zero.");
      return;
    }
    if (!dpCashAccount) {
      setDpError("Pick a cash/bank account.");
      return;
    }
    const remaining = orderTotal - dpReceived;
    if (dpAmount > remaining) {
      setDpError(`Down payment exceeds remaining order total (${formatIDR(remaining)}).`);
      return;
    }
    setPostingDP(true);
    try {
      await api.createDownPayment(orderId, {
        cash_account_id: dpCashAccount,
        amount_cents: dpAmount,
        dp_date: dpDate,
        description: dpDesc.trim() || undefined,
      });
      await loadOrder(orderId);
      setDpAmount(0);
      setDpDesc("");
    } catch (err) {
      setDpError(err instanceof Error ? err.message : "Could not post the down payment.");
    } finally {
      setPostingDP(false);
    }
  };

  const handleRefundDP = async (dpId: number) => {
    setDpError(null);
    try {
      await api.refundDownPayment(dpId);
      if (orderId) await loadOrder(orderId);
    } catch (err) {
      setDpError(err instanceof Error ? err.message : "Could not refund the down payment.");
    }
  };

  const isExisting = orderId !== null;
  const remaining = orderTotal - dpReceived;

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Sales Order</span>
          <span className={`entrytab__status ${isExisting ? "" : "entrytab__status--draft"}`}>
            {isExisting ? orderStatus : "DRAFT"}
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
                <span className="field__label">Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} disabled={isExisting} />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">No</span>
                <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
              </label>
              <TaxRateSelector value={taxRate} onChange={setTaxRate} disabled={isExisting} />
            </div>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Terms, delivery instructions, references..." disabled={isExisting} />
          </label>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Referensi Customer</legend>
            <div className="entrytab__detail-title" style={{ marginBottom: "8px" }}>
              <div className="entrytab__header-grid">
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Customer PO Number</span>
                    <input className="input" type="text" value={customerPONumber} onChange={(e) => setCustomerPONumber(e.target.value)} placeholder="PO-12345" disabled={isExisting} />
                  </label>
                </div>
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Customer PO Date</span>
                    <input className="input" type="date" value={customerPODate} onChange={(e) => setCustomerPODate(e.target.value)} disabled={isExisting} />
                  </label>
                </div>
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Requested Delivery Date</span>
                    <input className="input" type="date" value={requestedDeliveryDate} onChange={(e) => setRequestedDeliveryDate(e.target.value)} disabled={isExisting} />
                  </label>
                </div>
              </div>
            </div>
            <div className="entrytab__detail-title" style={{ marginTop: "8px", marginBottom: "8px" }}>
              <div className="entrytab__header-grid">
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Shipping Terms</span>
                    <select className="input" value={shippingTerms ?? ""} onChange={(e) => setShippingTerms(e.target.value ? (e.target.value as NonNullable<SalesOrder["shipping_terms"]>) : undefined)} disabled={isExisting}>
                      <option value="">None</option>
                      <option value="FOB">FOB</option>
                      <option value="CIF">CIF</option>
                      <option value="EXW">EXW</option>
                      <option value="CFR">CFR</option>
                      <option value="DAP">DAP</option>
                    </select>
                  </label>
                </div>
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Salesperson ID</span>
                    <input className="input" type="number" value={salespersonId} onChange={(e) => setSalespersonId(e.target.value)} placeholder="e.g., 1" disabled={isExisting} />
                  </label>
                </div>
              </div>
            </div>
            <div className="field">
              <label className="field">
                <span className="field__label">Ship To Address</span>
                <textarea className="input" rows={2} value={shipToAddress} onChange={(e) => setShipToAddress(e.target.value)} placeholder="Full delivery address..." disabled={isExisting} />
              </label>
            </div>
          </fieldset>

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
            <span className="entrytab__total-label">DPP (Subtotal)</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? subtotalCents : subtotalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">PPN {taxRate > 0 ? `(${taxRate}%)` : ""}</span>
            <span className="entrytab__total-value">{formatIDR(ppnCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--accent)", paddingTop: 8 }}>
            <span className="entrytab__total-label">Total</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? subtotalCents + ppnCents : totalCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving} title="Save sales order (posts no journal)">
              <DiskIcon />
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>

      {isExisting && (
        <div className="entrytab__dp-section" style={{ marginTop: 16, borderTop: "2px solid var(--accent)", paddingTop: 12 }}>
          <div className="entrytab__detail-title" style={{ marginBottom: 8 }}>
            Down Payments — DP Received: <strong>{formatIDR(dpReceived)}</strong> / Remaining: <strong>{formatIDR(remaining)}</strong>
          </div>

          {downPayments.length > 0 && (
            <div className="ledger-table" style={{ marginBottom: 12 }}>
              <div className="ledger-table__head">
                <span>Number</span>
                <span>Date</span>
                <span className="right">Amount</span>
                <span>Status</span>
                <span aria-hidden="true" />
              </div>
              {downPayments.map((dp) => (
                <div className="ledger-table__row" key={dp.id}>
                  <span className="ledger-table__no">{dp.number}</span>
                  <span className="ledger-table__date">{dp.dp_date}</span>
                  <span className="ledger-table__amount right">{formatIDR(dp.amount_cents)}</span>
                  <span>
                    <span className={`kind-mark ${dp.status === "RECEIVED" ? "is-positive" : "is-negative"}`}>{dp.status}</span>
                  </span>
                  <span>
                    {dp.status === "RECEIVED" && (
                      <button type="button" className="btn btn--secondary btn--sm" onClick={() => void handleRefundDP(dp.id)}>
                        Refund
                      </button>
                    )}
                  </span>
                </div>
              ))}
            </div>
          )}

          {orderStatus === "CONFIRMED" && remaining > 0 && (
            <div className="detail-grid detail-grid--quote" style={{ gridTemplateColumns: "1fr 1fr 2fr" }}>
              <div className="field">
                <span className="field__label">Amount</span>
                <input className="amount input" type="text" inputMode="numeric" value={centsInput(dpAmount)} onChange={(e) => setDpAmount(parseCents(e.target.value))} placeholder="0" />
              </div>
              <div className="field">
                <span className="field__label">Cash/Bank</span>
                <select className="input" value={dpCashAccount} onChange={(e) => setDpCashAccount(Number(e.target.value))}>
                  {cashAccounts.map((a) => (
                    <option key={a.id} value={a.id}>{a.name}</option>
                  ))}
                </select>
              </div>
              <div className="field">
                <span className="field__label">Description</span>
                <input className="input" type="text" value={dpDesc} onChange={(e) => setDpDesc(e.target.value)} placeholder="Down payment description..." />
              </div>
              <div className="field">
                <span className="field__label">Date</span>
                <input className="input" type="date" value={dpDate} onChange={(e) => setDpDate(e.target.value)} />
              </div>
              <div />
              <div style={{ display: "flex", alignItems: "flex-end" }}>
                <button type="button" className="btn btn--primary btn--sm" onClick={() => void handlePostDP()} disabled={postingDP}>
                  {postingDP ? "Posting..." : "Receive DP"}
                </button>
              </div>
            </div>
          )}
          <FormError message={dpError} />
        </div>
      )}
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
