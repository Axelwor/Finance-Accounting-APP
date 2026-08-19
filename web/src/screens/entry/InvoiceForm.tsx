import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { openPrintWindow } from "../../lib/print";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import type { Customer, Item, InvoiceLineInput, SalesOrderListItem, InvoicePayment, CreatePaymentInput, Invoice } from "../../types";
import type { PrefillRef } from "../../workbench/types";
import { AttachmentPanel } from "../../components/AttachmentPanel";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
  /** Workflow-chain prefill: {kind:"delivery-order", id} copies DO lines. */
  prefill?: PrefillRef;
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

export function InvoiceForm({ tabId, entryId, initialTitle, prefill }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const paymentsRef = useRef<HTMLDivElement>(null);
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
  const [taxTotal, setTaxTotal] = useState<number | null>(null);
  const [dpApplied, setDpApplied] = useState(0);
  const [receivable, setReceivable] = useState(0);
  const [payments, setPayments] = useState<InvoicePayment[]>([]);
  const [payAmount, setPayAmount] = useState(0);
  const [payDate, setPayDate] = useState(todayISO());
  const [payCashAccount, setPayCashAccount] = useState(0);
  const [payDesc, setPayDesc] = useState("");
  const [payError, setPayError] = useState<string | null>(null);
  const [postingPay, setPostingPay] = useState(false);
  const [taxInvoiceNumber, setTaxInvoiceNumber] = useState("");
  const [salespersonId, setSalespersonId] = useState("");
  const [taxRate, setTaxRate] = useState(0);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, customerId, salesOrderId, notes, lines, taxRate, workbench.markUnsaved]);

  useEffect(() => {
    void api.listCustomers().then(setCustomers);
    void api.listItems().then(setItems);
    void api.listSalesOrders("CLOSED").then(setSalesOrders);
    void api.listAccounts().then((accs) => {
      setCashAccounts(accs.map((a) => ({ id: Number(a.id), name: a.name })));
      if (accs.length > 0) setPayCashAccount(Number(accs[0].id));
    });
  }, []);

  // Workflow chain: pre-fill from a Delivery Order ("Create Invoice"). The
  // DO supplies customer + delivered qty; its sales order supplies prices.
  useEffect(() => {
    if (!prefill || prefill.kind !== "delivery-order" || entryId) return;
    let cancelled = false;
    void (async () => {
      try {
        const deliv = await api.getDeliveryOrder(prefill.id);
        if (cancelled) return;
        // Prices live on the SO, not the DO — resolve them per item.
        let priceByItem = new Map<number, { price: number; discount: number }>();
        if (deliv.sales_order_id) {
          try {
            const so = await api.getSalesOrder(deliv.sales_order_id);
            priceByItem = new Map(
              so.lines.map((l) => [l.item_id, { price: l.unit_price_cents, discount: l.discount_cents }]),
            );
            if (!cancelled) setSalesOrderId(String(so.id));
          } catch {
            // SO unavailable — invoice lines fall back to zero price.
          }
        }
        if (cancelled) return;
        setCustomerId(String(deliv.customer_id));
        setNotes(`Delivery ${deliv.number}`);
        setLines(
          deliv.lines.length > 0
            ? deliv.lines.map((l) => {
                const pricing = priceByItem.get(l.item_id);
                const qty = Number(l.qty) || 1;
                const unitPriceCents = pricing?.price ?? 0;
                const discountCents = pricing?.discount ?? 0;
                return {
                  id: `ln-src-${l.id}`,
                  itemId: String(l.item_id),
                  itemCode: l.item_code ?? "",
                  itemName: l.item_name ?? "",
                  qty,
                  unitPriceCents,
                  discountCents,
                  lineTotalCents: Math.round(qty * unitPriceCents) - discountCents,
                };
              })
            : [seedLine()],
        );
        toast.info(`Loaded ${deliv.lines.length} line(s) from delivery ${deliv.number}`);
      } catch {
        // Leave the draft blank if the delivery cannot be loaded.
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prefill?.kind, prefill?.id, entryId]);

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
      // total_cents is tax-inclusive; tax_total_cents is not populated by the
      // API (always 0), so a 0/absent value means "recompute from the lines".
      setTaxTotal(inv.tax_total_cents || null);
      setDpApplied(inv.dp_applied_cents);
      setReceivable(inv.receivable_cents);
      setDate(inv.invoice_date);
      setDueDate(inv.due_date ?? "");
      setCustomerId(String(inv.customer_id));
      setSalesOrderId(inv.sales_order_id ? String(inv.sales_order_id) : "");
      setNotes(inv.notes ?? "");
      setTaxInvoiceNumber(inv.tax_invoice_number ?? "");
      setSalespersonId(inv.salesperson_id ? String(inv.salesperson_id) : "");
      // Restore the document-level rate from the first line that carries one.
      const firstRate = inv.lines.find((l) => Number(l.tax_rate) > 0);
      setTaxRate(firstRate ? Number(firstRate.tax_rate) : Number(inv.lines[0]?.tax_rate ?? 0) || 0);
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
        tax_rate: taxRate,
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
        tax_invoice_number: taxInvoiceNumber.trim() || undefined,
        salesperson_id: salespersonId ? Number(salespersonId) : undefined,
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
      toast.success(`✓ Saved ${created.number}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the invoice.");
    } finally {
      setSaving(false);
    }
  };

  /** Scroll to the inline payment panel (Receive Payment next step). */
  const handleReceivePayment = () => {
    paymentsRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const handlePrint = () => {
    const customer = customers.find((c) => String(c.id) === customerId);
    openPrintWindow({
      title: `Sales Invoice ${number}`,
      subtitle: isExisting ? invStatus : "DRAFT",
      meta: [
        ["Customer", customer ? `${customer.code} · ${customer.name}` : "-"],
        ["Invoice date", formatDateID(date)],
        ["Due date", dueDate ? formatDateID(dueDate) : "-"],
        ["Tax invoice no", taxInvoiceNumber || "-"],
        ["Notes", notes || "-"],
      ],
      columns: [
        { label: "Item" },
        { label: "Qty", right: true },
        { label: "Unit Price", right: true },
        { label: "Discount", right: true },
        { label: "Line Total", right: true },
      ],
      rows: lines
        .filter((l) => l.itemId)
        .map((l) => [
          `${l.itemCode || l.itemId}${l.itemName ? ` · ${l.itemName}` : ""}`,
          l.qty,
          formatIDR(l.unitPriceCents),
          formatIDR(l.discountCents),
          formatIDR(l.lineTotalCents),
        ]),
      totals: [
        ["Total", formatIDR(isExisting ? total : totalCents)],
        ...(isExisting && dpApplied > 0 ? ([["DP Applied", formatIDR(dpApplied)]] as Array<[string, string]>) : []),
        ...(isExisting ? ([["Receivable", formatIDR(receivable)]] as Array<[string, string]>) : []),
      ],
    });
  };

  /** No email endpoint yet — the print view doubles as the send artifact. */
  const handleSend = () => {
    handlePrint();
    toast.info("Print preview opened — save it as PDF to send the invoice.");
  };

  /** The API has no invoice void; a full credit note is the supported path. */
  const handleVoid = () => {
    if (!invId) return;
    const ok = window.confirm(
      `The API cannot void posted invoices directly. Open a pre-filled credit note for ${number} instead? Saving it reverses the invoice.`,
    );
    if (!ok) return;
    workbench.openEntryDraftFromParent("credit-note-entry", { kind: "invoice", id: invId });
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
      toast.success(`✓ Payment of ${formatIDR(payAmount)} received`);
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

          <div className="entrytab__section" style={{ marginTop: "16px", marginBottom: "16px" }}>
            <div className="entrytab__section-head">
              <div className="entrytab__section-title">Erp Details</div>
            </div>
            <div className="entrytab__detail-title" style={{ marginBottom: "8px" }}>
              <div className="entrytab__header-grid">
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Tax Invoice Number</span>
                    <input className="input" type="text" value={taxInvoiceNumber} onChange={(e) => setTaxInvoiceNumber(e.target.value)} placeholder="Faktur Pajak" disabled={isExisting} />
                  </label>
                </div>
                <div className="entrytab__header-col">
                  <TaxRateSelector value={taxRate} onChange={setTaxRate} disabled={isExisting} />
                </div>
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Salesperson ID</span>
                    <input className="input" type="number" value={salespersonId} onChange={(e) => setSalespersonId(e.target.value)} placeholder="e.g., 1" disabled={isExisting} />
                  </label>
                </div>
              </div>
            </div>
          </div>

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
                    <Button
                      variant="outlined"
                      size="sm"
                      onClick={() => setLines((cur) => [...cur, seedLine()])}
                    >
                      + Add item
                    </Button>
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
            <span className="entrytab__total-value">{formatIDR(isExisting ? total - (taxTotal ?? ppnCents) : subtotalCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8 }}>
            <span className="entrytab__total-label">PPN {taxRate > 0 ? `(${taxRate}%)` : ""}</span>
            <span className="entrytab__total-value">{formatIDR(isExisting ? (taxTotal ?? ppnCents) : ppnCents)}</span>
          </div>
          <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--md-sys-color-primary)", paddingTop: 8 }}>
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
            <div className="entrytab__total" style={{ marginTop: 8, borderTop: "2px solid var(--md-sys-color-primary)", paddingTop: 8 }}>
              <span className="entrytab__total-label">Receivable</span>
              <span className="entrytab__total-value">{formatIDR(receivable)}</span>
            </div>
          )}

          {isExisting && (
            <NextStepsBar number={number} hint={invStatus}>
              {receivable > 0 && invStatus !== "VOID" && (
                <button type="button" className="next-steps__btn next-steps__btn--primary" onClick={handleReceivePayment}>
                  Receive Payment
                </button>
              )}
              <button type="button" className="next-steps__btn" onClick={handlePrint}>
                Print Invoice
              </button>
              <button type="button" className="next-steps__btn" onClick={handleSend}>
                Send
              </button>
              {invStatus !== "VOID" && invStatus !== "PAID" && (
                <button type="button" className="next-steps__btn next-steps__btn--danger" onClick={handleVoid}>
                  Void
                </button>
              )}
              <button type="button" className="next-steps__btn" onClick={() => workbench.close(tabId)}>
                Close
              </button>
            </NextStepsBar>
          )}

          {isExisting && (
            <div ref={paymentsRef} style={{ marginTop: 16, borderTop: "2px solid var(--md-sys-color-primary)", paddingTop: 12 }}>
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
                    <Button
                      variant="filled"
                      size="sm"
                      onClick={() => void handlePostPayment()}
                      disabled={postingPay}
                    >
                      {postingPay ? "Posting..." : "Receive Payment"}
                    </Button>
                  </div>
                </div>
              )}
              <FormError message={payError} />

              {invId && (
                <AttachmentPanel ownerType="invoice" ownerId={invId} />
              )}
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
