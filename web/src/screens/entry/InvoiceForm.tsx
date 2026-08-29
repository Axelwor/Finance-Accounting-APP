import { useCallback, useEffect, useMemo, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { parseRupiahToCents, formatIDRFromCents, todayISO } from "../../lib/format";
import { useToast } from "../../components/Toast";
import { Icon } from "../../components/m3/Icon";
import { CurrencyRatePicker } from "../../components/CurrencyRatePicker";
import type { Customer, Item, SalesOrderListItem, Invoice, InvoicePayment } from "../../types";
import type { PrefillRef } from "../../workbench/types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
  prefill?: PrefillRef;
}

interface Line {
  id: string;
  itemId: string;
  itemCode: string;
  itemName: string;
  qty: number;
  unitPriceCents: number;
  /** Raw rupiah text as typed by the user; cents derived via parseRupiahToCents. */
  priceInput: string;
  discountPct: number;
  lineTotalCents: number;
}

let lineSeq = 0;
function seedLine(): Line {
  lineSeq += 1;
  return {
    id: `inv-l-${Date.now()}-${lineSeq}`,
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 1,
    unitPriceCents: 0,
    priceInput: "",
    discountPct: 0,
    lineTotalCents: 0,
  };
}

export function InvoiceForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isDetail = entryId !== undefined;

  const [activeSubTab, setActiveSubTab] = useState<"items" | "shipping" | "tax" | "journal">("items");
  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [dueDate, setDueDate] = useState("");
  const [number, setNumber] = useState(initialTitle ?? "INV-2026/DRAFT");
  const [customerId, setCustomerId] = useState("");
  const [salesOrderId, setSalesOrderId] = useState("");
  const [notes, setNotes] = useState("");
  const [shippingAddress, setShippingAddress] = useState("");
  const [currencyCode, setCurrencyCode] = useState("IDR");
  const [exchangeRate, setExchangeRate] = useState(1);
  const [lines, setLines] = useState<Line[]>([seedLine()]);

  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [salesOrders, setSalesOrders] = useState<SalesOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [status, setStatus] = useState(isDetail ? "APPROVED" : "DRAFT");
  const toast = useToast();

  // Posted-invoice data (drives the Pelunasan panel + header/footer totals).
  const [savedId, setSavedId] = useState<number | null>(null);
  const [invoice, setInvoice] = useState<Invoice | null>(null);
  const [payments, setPayments] = useState<InvoicePayment[]>([]);

  // Receive-payment form (Pelunasan / Receive Payment).
  const [cashAccounts, setCashAccounts] = useState<{ id: number; name: string }[]>([]);
  const [payAccountId, setPayAccountId] = useState(0);
  const [payAmountInput, setPayAmountInput] = useState("");
  const [payDate, setPayDate] = useState(todayISO());
  const [payMemo, setPayMemo] = useState("");
  const [payError, setPayError] = useState<string | null>(null);
  const [postingPay, setPostingPay] = useState(false);

  const loadMasterData = useCallback(() => {
    void Promise.all([
      api.listCustomers().then(setCustomers),
      api.listItems().then(setItems),
      api.listSalesOrders("CLOSED").then(setSalesOrders),
    ]).finally(() => setLoading(false));
  }, []);
  useEffect(() => {
    loadMasterData();
  }, [loadMasterData]);
  useTabRefresh(loadMasterData);

  // Cash/bank options for the receive-payment form.
  useEffect(() => {
    let cancelled = false;
    void api.listAccounts().then((accs) => {
      if (cancelled) return;
      const mapped = accs.map((a) => ({
        id: Number(a.id),
        name: `${a.code ? `${a.code} - ` : ""}${a.name}`,
        type: (a.account_type ?? "").toUpperCase(),
      }));
      const cashish = mapped.filter((a) => a.type === "CASH" || a.type === "BANK");
      const options = cashish.length > 0 ? cashish : mapped;
      setCashAccounts(options);
      if (options.length > 0) setPayAccountId(options[0].id);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const loadInvoiceData = async (id: number) => {
    try {
      const inv = await api.getInvoice(id);
      setInvoice(inv);
      setStatus(inv.status);
      if (inv.number) setNumber(inv.number);
      setDate(inv.invoice_date);
      setDueDate(inv.due_date ?? "");
      setCustomerId(String(inv.customer_id));
      setNotes(inv.notes ?? "");
      setLines(
        inv.lines.map((l) => ({
          id: `inv-d-${l.id}`,
          itemId: String(l.item_id),
          itemCode: l.item_code ?? "",
          itemName: l.item_name ?? "",
          qty: Number(l.qty),
          unitPriceCents: l.unit_price_cents,
          priceInput: l.unit_price_cents ? String(l.unit_price_cents / 100) : "",
          discountPct:
            l.unit_price_cents > 0
              ? Math.round((l.discount_cents / l.unit_price_cents) * 100)
              : 0,
          lineTotalCents: l.line_total_cents,
        })),
      );
    } catch {
      // Detail fetch failed — keep whatever the form already shows.
    }
    try {
      setPayments(await api.listInvoicePayments(id));
    } catch {
      setPayments([]);
    }
  };

  const invoiceId = isDetail ? Number(entryId) : savedId;

  useEffect(() => {
    if (invoiceId) void loadInvoiceData(invoiceId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invoiceId]);

  const subtotal = useMemo(
    () => lines.reduce((sum, l) => sum + l.lineTotalCents, 0),
    [lines]
  );
  const ppnCents = Math.round(subtotal * 0.11);
  const grandTotal = subtotal + ppnCents;

  // Draft totals while editing; authoritative server values once loaded/posted.
  const subtotalCents = invoice?.sub_total_cents ?? subtotal;
  const ppnDisplayCents = invoice?.tax_total_cents ?? ppnCents;
  const totalDisplayCents = invoice?.total_cents ?? grandTotal;

  // Pelunasan visibility: any posted (non-draft) invoice with a backend record.
  const isPosted = status !== "DRAFT";
  const showPaymentPanel = isPosted && !!invoiceId;
  const paidAppliedCents = payments
    .filter((p) => p.status !== "REVERSED")
    .reduce((sum, p) => sum + p.ar_applied_cents, 0);
  const outstandingCents = invoice ? invoice.receivable_cents : 0;
  const canReceiveMore =
    status !== "PAID" && status !== "VOID" && outstandingCents > 0;

  const cashAccountLabel = (id: number) => {
    const acc = cashAccounts.find((a) => a.id === id);
    return acc ? acc.name : `Akun #${id}`;
  };

  // Keyboard shortcuts: Ctrl+S to save/post, Esc to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (workbench.activeNested?.id && workbench.activeNested.id !== tabId) return;

      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (!saving && !saved && !isDetail && grandTotal > 0) {
          void handlePost();
        }
      } else if (e.key === "Escape") {
        if (!saving) {
          workbench.close(tabId);
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [workbench.activeNested?.id, saving, saved, isDetail, tabId, grandTotal, lines, date, customerId, salesOrderId, notes, shippingAddress]);

  const addLine = () => setLines((prev) => [...prev, seedLine()]);

  const updateLineItem = (id: string, itemId: string) => {
    const itm = items.find((i) => String(i.id) === itemId);
    setLines((prev) =>
      prev.map((l) => {
        if (l.id !== id) return l;
        const price = (itm as any)?.selling_price_cents ?? (itm as any)?.unit_price_cents ?? 0;
        return {
          ...l,
          itemId,
          itemCode: itm?.code ?? "",
          itemName: itm?.name ?? "",
          unitPriceCents: price,
          priceInput: price ? String(price / 100) : "",
          lineTotalCents: Math.round(l.qty * price * (1 - l.discountPct / 100)),
        };
      })
    );
  };

  const updateLineQty = (id: string, qty: number) => {
    setLines((prev) =>
      prev.map((l) => {
        if (l.id !== id) return l;
        const q = Math.max(1, qty);
        return {
          ...l,
          qty: q,
          lineTotalCents: Math.round(q * l.unitPriceCents * (1 - l.discountPct / 100)),
        };
      })
    );
  };

  const updateLinePrice = (id: string, text: string) => {
    setLines((prev) =>
      prev.map((l) => {
        if (l.id !== id) return l;
        const price = parseRupiahToCents(text);
        return {
          ...l,
          priceInput: text,
          unitPriceCents: price,
          lineTotalCents: Math.round(l.qty * price * (1 - l.discountPct / 100)),
        };
      })
    );
  };

  const removeLine = (id: string) => {
    if (lines.length <= 1) return;
    setLines((prev) => prev.filter((l) => l.id !== id));
  };

  const handlePost = async () => {
    if (!customerId) {
      setError("Pilih pelanggan untuk faktur penjualan.");
      return;
    }
    if (lines.some((l) => !l.itemId || l.lineTotalCents <= 0)) {
      setError("Pastikan semua baris item memiliki produk dan harga yang valid.");
      return;
    }

    setError(null);
    setSaving(true);
    try {
      const payload = {
        customer_id: Number(customerId),
        invoice_date: date,
        due_date: dueDate || date,
        notes: notes.trim() || undefined,
        sales_order_id: salesOrderId ? Number(salesOrderId) : undefined,
        currency_code: currencyCode,
        exchange_rate: currencyCode === "IDR" ? 1 : exchangeRate,
        lines: lines.map((l) => ({
          item_id: Number(l.itemId),
          qty: l.qty,
          unit_price_cents: l.unitPriceCents,
          discount_cents: Math.round(l.unitPriceCents * (l.discountPct / 100)),
          tax_rate: 11,
        })),
      };

      const res = await api.createInvoice(payload);
      setSaved(true);
      setSavedId(res.id);
      setStatus(res.status && res.status !== "DRAFT" ? res.status : "APPROVED");
      if (res.number) setNumber(res.number);
      // Convert the draft tab into the persisted record: title = invoice number,
      // unsaved dot cleared, entryId recorded so reloads reuse this tab.
      workbench.replaceDraft(tabId, res.number ?? number, res.status ?? "APPROVED", res.id);
      workbench.markUnsaved(tabId, false);
      toast.success(`✓ Faktur terbit — ${res.number}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menerbitkan faktur penjualan.");
    } finally {
      setSaving(false);
    }
  };

  const handlePostPayment = async () => {
    if (!invoiceId || !invoice) return;
    setPayError(null);
    const amountCents = parseRupiahToCents(payAmountInput);
    if (amountCents <= 0) {
      setPayError("Jumlah pembayaran harus lebih besar dari nol.");
      return;
    }
    if (amountCents > outstandingCents) {
      setPayError(
        `Jumlah pembayaran melebihi sisa tagihan ${formatIDRFromCents(outstandingCents)}.`
      );
      return;
    }
    if (!payAccountId) {
      setPayError("Pilih akun kas/bank penerima pembayaran.");
      return;
    }
    setPostingPay(true);
    try {
      const res = await api.createInvoicePayment(invoiceId, {
        cash_account_id: payAccountId,
        amount_cents: amountCents,
        payment_date: payDate,
        description: payMemo.trim() || undefined,
        // SET-001: settlement rate for FC invoices (backend computes FX
        // gain/loss against the invoice's posting rate).
        exchange_rate: currencyCode === "IDR" ? 1 : exchangeRate,
      });
      toast.success(`✓ Pembayaran diterima — ${res.number}`);
      setPayAmountInput("");
      setPayMemo("");
      await loadInvoiceData(invoiceId);
    } catch (err) {
      // Backend (2402 dsb.) adalah sumber kebenaran untuk penolakan bisnis.
      setPayError(err instanceof Error ? err.message : "Gagal mencatat penerimaan pembayaran.");
    } finally {
      setPostingPay(false);
    }
  };

  if (loading) return <LoadingState label="Memuat form faktur..." />;

  const selectedCustomer = customers.find((c) => String(c.id) === customerId);

  return (
    <div className="enterprise-form">
      {/* Zone 1: Sticky Header */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="receipt" size={20} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Faktur Penjualan (Sales Invoice)</h1>
              <span className="form-header__doc-number">{number}</span>
              <span className={`form-header__status-badge status-${status.toLowerCase()}`}>
                {status}
              </span>
            </div>
            <p className="text-xs text-muted mt-0.5">Penjualan barang dagang & jasa komersial</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Faktur (Print)"
          >
            <Icon name="print" size={16} />
          </button>
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => workbench.close(tabId)}
            title="Tutup Tab"
          >
            <Icon name="close" size={16} />
          </button>
        </div>
      </header>

      {/* Zone 2: Dynamic Form Body with Sub-Tabs */}
      <main className="form-zone-2">
        {error && <FormError message={error} />}

        {/* 2.A Primary Entity & Meta */}
        <div className="form-card form-grid-2col">
          <div className="flex flex-col gap-3">
            <div className="auth-field">
              <label>Pelanggan (Customer) *</label>
              <select
                className="input-base font-semibold"
                value={customerId}
                disabled={isDetail || saved}
                onChange={(e) => setCustomerId(e.target.value)}
              >
                <option value="">-- Pilih Pelanggan --</option>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.code ? `${c.code} - ` : ""}{c.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="auth-field">
              <label>Referensi Sales Order (SO)</label>
              <select
                className="input-base"
                value={salesOrderId}
                disabled={isDetail || saved}
                onChange={(e) => setSalesOrderId(e.target.value)}
              >
                <option value="">-- Tidak Terkait SO (Langsung) --</option>
                {salesOrders.map((so) => (
                  <option key={so.id} value={so.id}>
                    {so.number} &bull; {so.customer_name} ({formatIDRFromCents(so.total_cents)})
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="flex flex-col gap-3">
            <div className="grid-2col gap-3">
              <div className="auth-field">
                <label>Tanggal Faktur *</label>
                <input
                  type="date"
                  className="input-base font-mono"
                  value={date}
                  disabled={isDetail || saved}
                  onChange={(e) => setDate(e.target.value)}
                />
              </div>
              <div className="auth-field">
                <label>Jatuh Tempo (Due Date)</label>
                <input
                  type="date"
                  className="input-base font-mono"
                  value={dueDate}
                  disabled={isDetail || saved}
                  onChange={(e) => setDueDate(e.target.value)}
                />
              </div>
            </div>

            <div className="auth-field">
              <CurrencyRatePicker
                value={currencyCode}
                rate={exchangeRate}
                onChange={(code, rate) => {
                  setCurrencyCode(code);
                  setExchangeRate(rate);
                }}
                docDate={date}
                disabled={isDetail || saved}
              />
            </div>

            <div className="auth-field">
              <label>Alamat Penagihan</label>
              <input
                type="text"
                className="input-base"
                readOnly
                value={selectedCustomer ? (selectedCustomer.address || "Alamat belum disetel") : "Pilih pelanggan terlebih dahulu"}
              />
            </div>
          </div>
        </div>

        {/* Multi-Tab Navigation for Complex Documents */}
        <div className="flex items-center gap-2 border-b border-subtle pb-1">
          <button
            type="button"
            className={`tabpill ${activeSubTab === "items" ? "is-active" : ""}`}
            onClick={() => setActiveSubTab("items")}
          >
            <Icon name="package" size={14} />
            <span>Rincian Barang / Jasa</span>
          </button>
          <button
            type="button"
            className={`tabpill ${activeSubTab === "shipping" ? "is-active" : ""}`}
            onClick={() => setActiveSubTab("shipping")}
          >
            <Icon name="building" size={14} />
            <span>Pengiriman & Catatan</span>
          </button>
          <button
            type="button"
            className={`tabpill ${activeSubTab === "journal" ? "is-active" : ""}`}
            onClick={() => setActiveSubTab("journal")}
          >
            <Icon name="book_open" size={14} />
            <span>Pratinjau Jurnal (PSAK)</span>
          </button>
        </div>

        {activeSubTab === "items" && (
          <div className="form-card">
            <div className="flex-between mb-3">
              <h2 className="text-sm font-bold text-primary">Line Items Transaksi Penjualan</h2>
              {!isDetail && !saved && (
                <button
                  type="button"
                  className="btn-dash-secondary text-xs"
                  onClick={addLine}
                >
                  <Icon name="plus" size={14} />
                  <span>+ Tambah Baris Produk</span>
                </button>
              )}
            </div>

            <div className="datatable-wrapper">
              <table className="datatable">
                <thead>
                  <tr>
                    <th style={{ width: "35%" }}>Item / Produk *</th>
                    <th className="num" style={{ width: "10%" }}>Qty</th>
                    <th className="num" style={{ width: "20%" }}>Harga Satuan (Rp)</th>
                    <th className="num" style={{ width: "12%" }}>Diskon %</th>
                    <th className="num" style={{ width: "18%" }}>Total Baris (Rp)</th>
                    {!isDetail && !saved && <th style={{ width: "40px" }}>Aksi</th>}
                  </tr>
                </thead>
                <tbody>
                  {lines.map((line) => (
                    <tr key={line.id}>
                      <td>
                        <select
                          className="input-base text-xs w-full font-semibold"
                          value={line.itemId}
                          disabled={isDetail || saved}
                          onChange={(e) => updateLineItem(line.id, e.target.value)}
                        >
                          <option value="">-- Pilih Produk/Jasa --</option>
                          {items.map((itm) => (
                            <option key={itm.id} value={itm.id}>
                              {itm.code} - {itm.name} ({itm.unit || "Pcs"})
                            </option>
                          ))}
                        </select>
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          min="1"
                          className="input-base text-xs text-right font-mono w-full"
                          value={line.qty}
                          disabled={isDetail || saved}
                          onChange={(e) => updateLineQty(line.id, Number(e.target.value))}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="text"
                          inputMode="numeric"
                          className="input-base text-xs text-right font-mono font-semibold w-full"
                          value={line.priceInput}
                          disabled={isDetail || saved}
                          onChange={(e) => updateLinePrice(line.id, e.target.value)}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          className="input-base text-xs text-right font-mono w-full"
                          value={line.discountPct}
                          disabled={isDetail || saved}
                          onChange={(e) => {
                            const disc = Math.min(100, Math.max(0, Number(e.target.value)));
                            setLines((prev) =>
                              prev.map((l) =>
                                l.id === line.id
                                  ? {
                                      ...l,
                                      discountPct: disc,
                                      lineTotalCents: Math.round(l.qty * l.unitPriceCents * (1 - disc / 100)),
                                    }
                                  : l
                              )
                            );
                          }}
                        />
                      </td>
                      <td className="num font-mono font-bold text-primary">
                        {formatIDRFromCents(line.lineTotalCents)}
                      </td>
                      {!isDetail && !saved && (
                        <td className="text-center">
                          <button
                            type="button"
                            className="topbar__icon-btn text-danger"
                            disabled={lines.length <= 1}
                            onClick={() => removeLine(line.id)}
                            title="Hapus baris"
                          >
                            <Icon name="trash" size={14} />
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr className="total-rule-top total-double">
                    <td colSpan={4} className="text-right font-bold text-xs">
                      Subtotal Penjualan:
                    </td>
                    <td className="num font-mono font-bold text-primary text-sm">
                      {formatIDRFromCents(subtotalCents)}
                    </td>
                    {!isDetail && !saved && <td />}
                  </tr>
                  <tr>
                    <td colSpan={4} className="text-right font-semibold text-xs text-muted">
                      PPN 11%:
                    </td>
                    <td className="num font-mono font-semibold text-secondary text-xs">
                      {formatIDRFromCents(ppnDisplayCents)}
                    </td>
                    {!isDetail && !saved && <td />}
                  </tr>
                  <tr className="total-double">
                    <td colSpan={4} className="text-right font-bold text-xs text-brand">
                      Total Tagihan (Grand Total):
                    </td>
                    <td className="num font-mono font-bold text-brand text-base">
                      {formatIDRFromCents(totalDisplayCents)}
                    </td>
                    {!isDetail && !saved && <td />}
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        )}

        {activeSubTab === "shipping" && (
          <div className="form-card form-grid-2col">
            <div className="auth-field">
              <label>Alamat Kirim & Detail Ekspedisi</label>
              <textarea
                className="input-base"
                rows={4}
                placeholder="Alamat gudang / rincian nomor resi pengiriman..."
                value={shippingAddress}
                disabled={isDetail || saved}
                onChange={(e) => setShippingAddress(e.target.value)}
              />
            </div>
            <div className="auth-field">
              <label>Catatan Syarat Pembayaran & Memo</label>
              <textarea
                className="input-base"
                rows={4}
                placeholder="Contoh: Transfer ke rekening BCA 123456789 a/n PT Maju Bersama..."
                value={notes}
                disabled={isDetail || saved}
                onChange={(e) => setNotes(e.target.value)}
              />
            </div>
          </div>
        )}

        {activeSubTab === "journal" && (
          <div className="form-card bg-surface-secondary">
            <div className="flex items-center gap-2 mb-2">
              <Icon name="security" size={16} className="text-brand" />
              <h3 className="text-xs font-bold text-primary uppercase">Dampak Jurnal Akuntansi (Live General Ledger Impact)</h3>
            </div>
            <div className="text-xs font-mono space-y-1">
              <p className="text-indigo-700 font-semibold">
                (Dr) 1103 - Piutang Usaha (AR): <strong>{formatIDRFromCents(totalDisplayCents)}</strong>
              </p>
              <p className="text-slate-600 pl-4">
                (Cr) 4101 - Pendapatan Penjualan: {formatIDRFromCents(subtotalCents)}
              </p>
              <p className="text-slate-600 pl-4">
                (Cr) 2202 - Hutang PPN Keluaran (11%): {formatIDRFromCents(ppnDisplayCents)}
              </p>
            </div>
          </div>
        )}

        {/* Pelunasan / Receive Payment — only for posted invoices */}
        {showPaymentPanel && invoice && (
          <div className="form-card" id="invoice-payment-panel">
            <div className="flex-between mb-3">
              <h2 className="text-sm font-bold text-primary">Pelunasan / Receive Payment</h2>
              <span className="text-xs text-muted">
                Sisa Outstanding:{" "}
                <strong className={`font-mono ${outstandingCents > 0 ? "text-brand" : "text-secondary"}`}>
                  {formatIDRFromCents(outstandingCents)}
                </strong>
              </span>
            </div>

            <div className="text-xs text-muted mb-3">
              Total Tagihan: <span className="font-mono">{formatIDRFromCents(totalDisplayCents)}</span>{" "}
              &bull; Sudah Diterima: <span className="font-mono">{formatIDRFromCents(paidAppliedCents)}</span>
              {(invoice.dp_applied_cents ?? 0) > 0 && (
                <>
                  {" "}&bull; DP Teraplikasi:{" "}
                  <span className="font-mono">{formatIDRFromCents(invoice.dp_applied_cents)}</span>
                </>
              )}
            </div>

            <div className="datatable-wrapper mb-3">
              <table className="datatable">
                <thead>
                  <tr>
                    <th>Nomor Bukti</th>
                    <th>Tanggal</th>
                    <th>Akun Kas/Bank</th>
                    <th className="num">Jumlah</th>
                    <th>Memo</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {payments.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="text-center text-xs text-muted">
                        Belum ada pembayaran yang diterima untuk faktur ini.
                      </td>
                    </tr>
                  ) : (
                    payments.map((pmt) => (
                      <tr key={pmt.id}>
                        <td className="font-mono font-semibold text-xs">{pmt.number}</td>
                        <td className="font-mono text-xs">{pmt.payment_date}</td>
                        <td className="text-xs">{cashAccountLabel(pmt.cash_account_id)}</td>
                        <td className="num font-mono font-bold text-primary text-xs">
                          {formatIDRFromCents(pmt.amount_cents)}
                        </td>
                        <td className="text-xs text-muted">{pmt.description || "-"}</td>
                        <td>
                          <span
                            className={`status-badge ${
                              pmt.status === "RECEIVED" ? "status-balanced" : "status-unbalanced"
                            } text-xs`}
                          >
                            {pmt.status}
                          </span>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {canReceiveMore ? (
              <>
                <div className="form-grid-2col">
                  <div className="auth-field">
                    <label htmlFor="inv-pay-account">Akun Kas/Bank *</label>
                    <select
                      id="inv-pay-account"
                      className="input-base"
                      value={payAccountId}
                      disabled={postingPay || cashAccounts.length === 0}
                      onChange={(e) => setPayAccountId(Number(e.target.value))}
                    >
                      {cashAccounts.map((a) => (
                        <option key={a.id} value={a.id}>
                          {a.name}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="auth-field">
                    <label htmlFor="inv-pay-amount">Jumlah Diterima (Rp) *</label>
                    <input
                      id="inv-pay-amount"
                      type="text"
                      inputMode="numeric"
                      placeholder="0"
                      className="input-base text-right font-mono font-semibold"
                      value={payAmountInput}
                      disabled={postingPay}
                      onChange={(e) => setPayAmountInput(e.target.value)}
                    />
                  </div>
                  <div className="auth-field">
                    <label htmlFor="inv-pay-date">Tanggal Pembayaran *</label>
                    <input
                      id="inv-pay-date"
                      type="date"
                      className="input-base font-mono"
                      value={payDate}
                      disabled={postingPay}
                      onChange={(e) => setPayDate(e.target.value)}
                    />
                  </div>
                  <div className="auth-field">
                    <label htmlFor="inv-pay-memo">Memo / Referensi</label>
                    <input
                      id="inv-pay-memo"
                      type="text"
                      className="input-base"
                      placeholder="Contoh: Transfer BCA 123456789"
                      value={payMemo}
                      disabled={postingPay}
                      onChange={(e) => setPayMemo(e.target.value)}
                    />
                  </div>
                </div>
                <div className="flex items-center justify-end mt-2 gap-3">
                  <span className="text-xs text-muted">
                    Membayar penuh? Isi jumlah sama dengan sisa outstanding.
                  </span>
                  <button
                    type="button"
                    className="btn-dash-primary"
                    disabled={postingPay || cashAccounts.length === 0}
                    onClick={() => void handlePostPayment()}
                  >
                    {postingPay ? (
                      <span>Mencatat Pembayaran...</span>
                    ) : (
                      <>
                        <Icon name="check" size={14} />
                        <span>Terima Pembayaran</span>
                      </>
                    )}
                  </button>
                </div>
              </>
            ) : (
              <p className="text-xs text-muted">
                {status === "PAID"
                  ? "Faktur sudah lunas."
                  : status === "VOID"
                    ? "Faktur dibatalkan — pembayaran ditutup."
                    : "Tidak ada sisa tagihan."}
              </p>
            )}
            {payError && <FormError message={payError} />}
          </div>
        )}

        {/* Official Print Signature Sign-off Box */}
        <div className="print-signoff">
          <div className="print-signoff-box">
            <div className="sign-role">Dibuat Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Bagian Penjualan / Billing )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Diperiksa Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Supervisor / Finance )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Diterima Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Pelanggan / Penerima Barang )</div>
          </div>
        </div>
      </main>

      {/* Zone 3: Sticky Summary & Action Footer */}
      <footer className="form-zone-3">
        <div className="flex items-center gap-4">
          <span className="status-badge status-balanced text-xs">
            <Icon name="check" size={12} /> {isPosted ? "TERPOSTING" : "JURNAL OTOMATIS SIAP POSTING"}
          </span>
          {isPosted ? (
            <span className="text-xs text-muted">
              Terima pembayaran di panel Pelunasan &bull; [Esc] Tutup
            </span>
          ) : (
            <span className="text-xs text-muted">
              [Ctrl+S] Posting Faktur &bull; [Esc] Tutup
            </span>
          )}
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <div className="text-xs text-muted">
              Subtotal: <span className="font-mono">{formatIDRFromCents(subtotalCents)}</span> &bull; PPN 11%: <span className="font-mono">{formatIDRFromCents(ppnDisplayCents)}</span>
            </div>
            <div className="text-sm font-bold text-primary">
              GRAND TOTAL: <span className="font-mono text-xl text-brand">{formatIDRFromCents(totalDisplayCents)}</span>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {!isDetail && !saved && (
              <button
                type="button"
                className="btn-dash-primary"
                disabled={grandTotal <= 0 || saving}
                onClick={handlePost}
              >
                {saving ? (
                  <span>Menerbitkan Faktur...</span>
                ) : (
                  <>
                    <Icon name="check" size={14} />
                    <span>Posting Faktur Penjualan</span>
                    <kbd className="btn-kbd">Ctrl+S</kbd>
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      </footer>
    </div>
  );
}
