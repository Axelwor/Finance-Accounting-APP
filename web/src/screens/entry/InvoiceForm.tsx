import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { Icon } from "../../components/m3/Icon";
import type { Customer, Item, SalesOrderListItem } from "../../types";
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
  const [lines, setLines] = useState<Line[]>([seedLine()]);

  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [salesOrders, setSalesOrders] = useState<SalesOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [status, setStatus] = useState(isDetail ? "APPROVED" : "DRAFT");

  useEffect(() => {
    void Promise.all([
      api.listCustomers().then(setCustomers),
      api.listItems().then(setItems),
      api.listSalesOrders("CLOSED").then(setSalesOrders),
    ]).finally(() => setLoading(false));
  }, []);

  const subtotal = useMemo(
    () => lines.reduce((sum, l) => sum + l.lineTotalCents, 0),
    [lines]
  );
  const ppnCents = Math.round(subtotal * 0.11);
  const grandTotal = subtotal + ppnCents;

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
        const price = (itm as any)?.selling_price_cents ?? (itm as any)?.unit_price_cents ?? 100000;
        return {
          ...l,
          itemId,
          itemCode: itm?.code ?? "",
          itemName: itm?.name ?? "",
          unitPriceCents: price,
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

  const updateLinePrice = (id: string, price: number) => {
    setLines((prev) =>
      prev.map((l) => {
        if (l.id !== id) return l;
        return {
          ...l,
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
        lines: lines.map((l) => ({
          item_id: Number(l.itemId),
          qty: l.qty,
          unit_price_cents: l.unitPriceCents,
          discount_cents: Math.round(l.unitPriceCents * (l.discountPct / 100)),
          tax_rate: 0.11,
        })),
      };

      const res = await api.createInvoice(payload);
      setSaved(true);
      setStatus("APPROVED");
      if (res?.number) setNumber(res.number);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menerbitkan faktur penjualan.");
    } finally {
      setSaving(false);
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
                    {so.number} &bull; {so.customer_name} ({formatIDR(so.total_cents)})
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
                          type="number"
                          className="input-base text-xs text-right font-mono font-semibold w-full"
                          value={line.unitPriceCents}
                          disabled={isDetail || saved}
                          onChange={(e) => updateLinePrice(line.id, Number(e.target.value))}
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
                        {formatIDR(line.lineTotalCents)}
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
                      {formatIDR(subtotal)}
                    </td>
                    {!isDetail && !saved && <td />}
                  </tr>
                  <tr>
                    <td colSpan={4} className="text-right font-semibold text-xs text-muted">
                      PPN 11%:
                    </td>
                    <td className="num font-mono font-semibold text-secondary text-xs">
                      {formatIDR(ppnCents)}
                    </td>
                    {!isDetail && !saved && <td />}
                  </tr>
                  <tr className="total-double">
                    <td colSpan={4} className="text-right font-bold text-xs text-brand">
                      Total Tagihan (Grand Total):
                    </td>
                    <td className="num font-mono font-bold text-brand text-base">
                      {formatIDR(grandTotal)}
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
                (Dr) 1103 - Piutang Usaha (AR): <strong>{formatIDR(grandTotal)}</strong>
              </p>
              <p className="text-slate-600 pl-4">
                (Cr) 4101 - Pendapatan Penjualan: {formatIDR(subtotal)}
              </p>
              <p className="text-slate-600 pl-4">
                (Cr) 2202 - Hutang PPN Keluaran (11%): {formatIDR(ppnCents)}
              </p>
            </div>
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
            <Icon name="check" size={12} /> JURNAL OTOMATIS SIAP POSTING
          </span>
          <span className="text-xs text-muted">
            [Ctrl+S] Posting Faktur &bull; [Esc] Tutup
          </span>
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <div className="text-xs text-muted">
              Subtotal: <span className="font-mono">{formatIDR(subtotal)}</span> &bull; PPN 11%: <span className="font-mono">{formatIDR(ppnCents)}</span>
            </div>
            <div className="text-sm font-bold text-primary">
              GRAND TOTAL: <span className="font-mono text-xl text-brand">{formatIDR(grandTotal)}</span>
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
