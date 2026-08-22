import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import { Icon } from "../../components/m3/Icon";
import type { Customer, Item, SalesOrderLineInput, DownPayment, SalesOrder } from "../../types";
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
  discountCents: number;
  lineTotalCents: number;
}

let lineSeq = 0;
function seedLine(): Line {
  lineSeq += 1;
  return {
    id: `so-ln-${Date.now()}-${lineSeq}`,
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 1,
    unitPriceCents: 0,
    discountCents: 0,
    lineTotalCents: 0,
  };
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round((qty > 0 ? qty : 1) * unitPriceCents) - discountCents;
}

export function SalesOrderForm({ tabId, entryId, initialTitle, prefill }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [activeTab, setActiveTab] = useState<"header" | "items" | "additional">("items");
  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [number, setNumber] = useState(initialTitle ?? draftNumber("sales-order-entry"));
  const [customerId, setCustomerId] = useState("");
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [orderId, setOrderId] = useState<number | null>(
    typeof entryId === "number" ? entryId : null
  );
  const [orderStatus, setOrderStatus] = useState<string>(initialTitle ? "" : "DRAFT");
  const [orderTotal, setOrderTotal] = useState(0);
  const [customerPONumber, setCustomerPONumber] = useState("");
  const [customerPODate, setCustomerPODate] = useState("");
  const [requestedDeliveryDate, setRequestedDeliveryDate] = useState("");
  const [shippingTerms, setShippingTerms] = useState<SalesOrder["shipping_terms"] | undefined>(undefined);
  const [shipToAddress, setShipToAddress] = useState("");
  const [salespersonId, setSalespersonId] = useState("");
  const [taxRate, setTaxRate] = useState(0);
  const [quotationId, setQuotationId] = useState<number | null>(null);

  const isExisting = orderId !== null;

  useEffect(() => {
    void Promise.all([
      api.listCustomers().then(setCustomers),
      api.listItems().then(setItems),
    ]).finally(() => setLoading(false));
  }, []);

  // Workflow chain: pre-fill from quotation
  useEffect(() => {
    if (!prefill || prefill.kind !== "quotation" || entryId) return;
    let cancelled = false;
    void api
      .getQuotation(prefill.id)
      .then((q) => {
        if (cancelled) return;
        setQuotationId(q.id);
        setCustomerId(String(q.customer_id));
        setNotes(q.number ? `Berdasarkan Penawaran ${q.number}` : "");
        setLines(
          q.lines.length > 0
            ? q.lines.map((l) => ({
                id: `ln-src-${l.id}`,
                itemId: String(l.item_id ?? ""),
                itemCode: l.item_code ?? "",
                itemName: l.item_name ?? "",
                qty: Number(l.qty),
                unitPriceCents: l.unit_price_cents,
                discountCents: l.discount_cents,
                lineTotalCents: l.line_total_cents,
              }))
            : [seedLine()]
        );
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [prefill, entryId]);

  // Load existing sales order
  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    void api
      .getSalesOrder(id)
      .then((so) => {
        setOrderId(so.id);
        setNumber(so.number);
        setOrderStatus(so.status);
        setDate(so.order_date);
        setCustomerId(String(so.customer_id));
        setNotes(so.notes ?? "");
        setOrderTotal(so.total_cents);
        setCustomerPONumber(so.customer_po_number ?? "");
        setCustomerPODate(so.customer_po_date ?? "");
        setRequestedDeliveryDate(so.requested_delivery_date ?? "");
        setShippingTerms(so.shipping_terms ?? undefined);
        setShipToAddress(so.ship_to_address ?? "");
        setSalespersonId(so.salesperson_id ? String(so.salesperson_id) : "");
        setLines(
          so.lines.map((l) => ({
            id: `ln-${l.id}`,
            itemId: String(l.item_id),
            itemCode: l.item_code ?? "",
            itemName: l.item_name ?? "",
            qty: Number(l.qty),
            unitPriceCents: l.unit_price_cents,
            discountCents: l.discount_cents,
            lineTotalCents: l.line_total_cents,
          }))
        );
        workbench.markUnsaved(tabId, false);
      })
      .catch(() => {});
  }, [entryId, tabId, workbench]);

  const subtotalCents = useMemo(() => lines.reduce((sum, l) => sum + l.lineTotalCents, 0), [lines]);
  const ppnCents = useMemo(
    () => lines.reduce((sum, l) => sum + taxForLine(l.lineTotalCents, taxRate), 0),
    [lines, taxRate]
  );
  const totalCents = isExisting ? orderTotal : subtotalCents + ppnCents;

  const setItem = (id: string, itemId: string) => {
    const item = items.find((i) => String(i.id) === itemId);
    const price = (item as any)?.selling_price_cents ?? (item as any)?.unit_price_cents ?? 0;
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? {
              ...l,
              itemId,
              itemCode: item?.code ?? "",
              itemName: item?.name ?? "",
              unitPriceCents: price,
              lineTotalCents: lineTotal(l.qty, price, l.discountCents),
            }
          : l
      )
    );
  };

  const setPrice = (id: string, unitPriceCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, unitPriceCents, lineTotalCents: lineTotal(l.qty, unitPriceCents, l.discountCents) }
          : l
      )
    );
  };

  const setQty = (id: string, qty: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, qty: qty > 0 ? qty : 1, lineTotalCents: lineTotal(qty, l.unitPriceCents, l.discountCents) }
          : l
      )
    );
  };

  const setDiscount = (id: string, discountCents: number) => {
    setLines((cur) =>
      cur.map((l) =>
        l.id === id
          ? { ...l, discountCents, lineTotalCents: lineTotal(l.qty, l.unitPriceCents, discountCents) }
          : l
      )
    );
  };

  const addLine = () => setLines((prev) => [...prev, seedLine()]);
  const removeLine = (id: string) => {
    if (lines.length <= 1) return;
    setLines((cur) => cur.filter((l) => l.id !== id));
  };

  const handleSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setError(null);
    if (isExisting) return;
    if (!customerId) {
      setError("Pilih pelanggan untuk sales order ini.");
      setActiveTab("header");
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
      setError("Tambahkan minimal 1 baris item produk.");
      setActiveTab("items");
      return;
    }
    setSaving(true);
    try {
      const created = await api.createSalesOrder({
        customer_id: Number(customerId),
        order_date: date,
        notes: notes.trim() || undefined,
        lines: payloadLines,
        customer_po_number: customerPONumber.trim() || undefined,
        customer_po_date: customerPODate || undefined,
        requested_delivery_date: requestedDeliveryDate || undefined,
        shipping_terms: shippingTerms,
        ship_to_address: shipToAddress.trim() || undefined,
        salesperson_id: salespersonId ? Number(salespersonId) : undefined,
        quotation_id: quotationId ?? undefined,
      });
      workbench.replaceDraft(tabId, created.number, "CONFIRMED");
      workbench.markUnsaved(tabId, false);
      setOrderId(created.id);
      setOrderStatus("CONFIRMED");
      setNumber(created.number);
      setOrderTotal(created.total_cents);
      toast.success(`✓ Sales Order ${created.number} Berhasil Dikonfirmasi`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan Sales Order.");
    } finally {
      setSaving(false);
    }
  };

  const handleCreateDelivery = () => {
    if (!orderId) return;
    workbench.openEntryDraftFromParent("delivery-order-entry", { kind: "sales-order", id: orderId });
  };

  const handleCreateInvoice = () => {
    if (!orderId) return;
    workbench.openEntryDraftFromParent("sales-invoice", { kind: "sales-order", id: orderId });
  };

  // Keyboard shortcuts: Ctrl+S to save/post, Esc to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (workbench.activeNested?.id && workbench.activeNested.id !== tabId) return;

      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (!saving && !isExisting && totalCents > 0) {
          void handleSubmit();
        }
      } else if (e.key === "Escape") {
        if (!saving) {
          workbench.close(tabId);
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [workbench.activeNested?.id, saving, isExisting, tabId, totalCents, lines, date, customerId, notes, taxRate]);

  if (loading) return <LoadingState label="Memuat formulir Sales Order..." />;

  const statusLabel = isExisting ? orderStatus || "CONFIRMED" : "DRAFT";
  const selectedCustomer = customers.find((c) => String(c.id) === customerId);

  return (
    <div className="enterprise-form">
      {/* Zone 1: Sticky Document Header */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="shopping_cart" size={20} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Pesanan Penjualan (Sales Order)</h1>
              <span className="form-header__doc-number">{number}</span>
              <span className={`form-header__status-badge status-${statusLabel.toLowerCase()}`}>
                {statusLabel}
              </span>
            </div>
            <p className="text-xs text-muted mt-0.5">
              Penerimaan & konfirmasi pesanan komersial resmi dari pelanggan
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Sales Order (Print)"
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

      {/* Zone 2: Dynamic Form Body with Corporate Multi-Tab Navigation */}
      <main className="form-zone-2">
        {error && <FormError message={error} />}

        {/* Corporate Form Navigation Tabs */}
        <div className="flex items-center gap-2 border-b border-subtle pb-1">
          <button
            type="button"
            className={`tabpill ${activeTab === "header" ? "is-active" : ""}`}
            onClick={() => setActiveTab("header")}
          >
            <Icon name="building" size={14} />
            <span>Tab 1: Informasi Header & Pihak Pemesan</span>
          </button>
          <button
            type="button"
            className={`tabpill ${activeTab === "items" ? "is-active" : ""}`}
            onClick={() => setActiveTab("items")}
          >
            <Icon name="package" size={14} />
            <span>Tab 2: Rincian Item Barang / Jasa</span>
            <span className="text-xs font-mono font-bold ml-1 opacity-75">({lines.filter(l => l.itemId).length})</span>
          </button>
          <button
            type="button"
            className={`tabpill ${activeTab === "additional" ? "is-active" : ""}`}
            onClick={() => setActiveTab("additional")}
          >
            <Icon name="receipt" size={14} />
            <span>Tab 3: PO Pelanggan, Pengiriman & Catatan</span>
          </button>
        </div>

        {/* TAB 1: INFORMASI HEADER & PELANGGAN */}
        {activeTab === "header" && (
          <div className="form-card form-grid-2col">
            <div className="flex flex-col gap-3">
              <div className="auth-field">
                <label>Pelanggan (Customer) *</label>
                <select
                  className="input-base font-semibold"
                  value={customerId}
                  disabled={isExisting}
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
                <label>Alamat Penagihan Pelanggan</label>
                <input
                  type="text"
                  className="input-base input-computed"
                  readOnly
                  value={selectedCustomer ? (selectedCustomer.address || "Alamat belum disetel") : "Pilih pelanggan untuk melihat alamat"}
                />
              </div>

              <div className="auth-field">
                <label>Kontak / Telepon</label>
                <input
                  type="text"
                  className="input-base input-computed"
                  readOnly
                  value={selectedCustomer ? (selectedCustomer.phone || selectedCustomer.email || "—") : "—"}
                />
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <div className="grid-2col gap-3">
                <div className="auth-field">
                  <label>Tanggal Order *</label>
                  <input
                    type="date"
                    className="input-base font-mono"
                    value={date}
                    disabled={isExisting}
                    onChange={(e) => setDate(e.target.value)}
                  />
                </div>
                <div className="auth-field">
                  <label>Target Tanggal Pengiriman</label>
                  <input
                    type="date"
                    className="input-base font-mono"
                    value={requestedDeliveryDate}
                    disabled={isExisting}
                    onChange={(e) => setRequestedDeliveryDate(e.target.value)}
                  />
                </div>
              </div>

              <div className="auth-field">
                <TaxRateSelector
                  value={taxRate}
                  onChange={setTaxRate}
                  disabled={isExisting}
                  label="Skema Pajak Pertambahan Nilai (PPN)"
                />
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: RINCIAN ITEM BARANG / JASA */}
        {activeTab === "items" && (
          <div className="form-card">
            <div className="flex-between mb-3">
              <div>
                <h2 className="text-sm font-bold text-primary">Line Items Pesanan Penjualan</h2>
                <p className="text-xs text-muted">Daftar item produk yang dipesan beserta kuantitas, harga jual, dan diskon.</p>
              </div>
              {!isExisting && (
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
                    <th className="num" style={{ width: "15%" }}>Diskon (Rp)</th>
                    <th className="num" style={{ width: "16%" }}>Total Baris (Rp)</th>
                    {!isExisting && <th style={{ width: "40px" }}>Aksi</th>}
                  </tr>
                </thead>
                <tbody>
                  {lines.map((line) => (
                    <tr key={line.id}>
                      <td>
                        <select
                          className="input-base text-xs w-full font-semibold"
                          value={line.itemId}
                          disabled={isExisting}
                          onChange={(e) => setItem(line.id, e.target.value)}
                        >
                          <option value="">-- Pilih Produk/Jasa --</option>
                          {items.map((i) => (
                            <option key={i.id} value={i.id}>
                              {i.code} - {i.name} ({i.unit || "Pcs"})
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
                          disabled={isExisting}
                          onChange={(e) => setQty(line.id, Number(e.target.value))}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          className="input-base text-xs text-right font-mono font-semibold w-full"
                          value={line.unitPriceCents}
                          disabled={isExisting}
                          onChange={(e) => setPrice(line.id, parseCents(e.target.value))}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          className="input-base text-xs text-right font-mono w-full"
                          value={line.discountCents}
                          disabled={isExisting}
                          onChange={(e) => setDiscount(line.id, parseCents(e.target.value))}
                        />
                      </td>
                      <td className="num font-mono font-bold text-primary">
                        {formatIDR(line.lineTotalCents)}
                      </td>
                      {!isExisting && (
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
                      Dasar Pengenaan Pajak (DPP Subtotal):
                    </td>
                    <td className="num font-mono font-bold text-primary text-sm">
                      {formatIDR(subtotalCents)}
                    </td>
                    {!isExisting && <td />}
                  </tr>
                  <tr>
                    <td colSpan={4} className="text-right font-semibold text-xs text-muted">
                      PPN {taxRate > 0 ? `(${taxRate}%)` : ""}:
                    </td>
                    <td className="num font-mono font-semibold text-secondary text-xs">
                      {formatIDR(ppnCents)}
                    </td>
                    {!isExisting && <td />}
                  </tr>
                  <tr className="total-double">
                    <td colSpan={4} className="text-right font-bold text-xs text-brand">
                      Total Pesanan (Grand Total):
                    </td>
                    <td className="num font-mono font-bold text-brand text-base">
                      {formatIDR(totalCents)}
                    </td>
                    {!isExisting && <td />}
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        )}

        {/* TAB 3: PO PELANGGAN, PENGIRIMAN & INFORMASI TAMBAHAN */}
        {activeTab === "additional" && (
          <div className="form-card form-grid-2col">
            <div className="flex flex-col gap-3">
              <div className="auth-field">
                <label>Nomor Purchase Order Pelanggan (Customer PO)</label>
                <input
                  type="text"
                  className="input-base font-mono"
                  placeholder="Contoh: PO-CUST-8891"
                  value={customerPONumber}
                  disabled={isExisting}
                  onChange={(e) => setCustomerPONumber(e.target.value)}
                />
              </div>
              <div className="auth-field">
                <label>Tanggal PO Pelanggan</label>
                <input
                  type="date"
                  className="input-base font-mono"
                  value={customerPODate}
                  disabled={isExisting}
                  onChange={(e) => setCustomerPODate(e.target.value)}
                />
              </div>
              <div className="auth-field">
                <label>Catatan & Instruksi Khusus Pesanan</label>
                <textarea
                  className="input-base"
                  rows={2}
                  placeholder="Instruksi pengemasan, referensi kontrak, dll..."
                  value={notes}
                  disabled={isExisting}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <div className="auth-field">
                <label>Syarat Pengiriman (Incoterms)</label>
                <select
                  className="input-base"
                  value={shippingTerms ?? ""}
                  disabled={isExisting}
                  onChange={(e) => setShippingTerms(e.target.value ? (e.target.value as NonNullable<SalesOrder["shipping_terms"]>) : undefined)}
                >
                  <option value="">-- Tanpa Ketentuan Khusus --</option>
                  <option value="FOB">FOB (Free On Board)</option>
                  <option value="CIF">CIF (Cost, Insurance & Freight)</option>
                  <option value="EXW">EXW (Ex Works / Ambil di Gudang)</option>
                  <option value="DAP">DAP (Delivered at Place)</option>
                </select>
              </div>
              <div className="auth-field">
                <label>Alamat Pengiriman Barang (Ship-To Address)</label>
                <textarea
                  className="input-base"
                  rows={3}
                  placeholder="Alamat lengkap penerimaan barang..."
                  value={shipToAddress}
                  disabled={isExisting}
                  onChange={(e) => setShipToAddress(e.target.value)}
                />
              </div>
            </div>
          </div>
        )}

        {/* Post-Save Next Steps Actions */}
        {isExisting && orderStatus === "CONFIRMED" && (
          <div className="form-card bg-surface-secondary">
            <NextStepsBar number={number} hint={orderStatus}>
              <button
                type="button"
                className="btn-dash-primary"
                onClick={handleCreateDelivery}
              >
                <Icon name="package" size={16} />
                <span>Buat Surat Jalan (Delivery Order)</span>
              </button>
              <button
                type="button"
                className="btn-dash-secondary"
                onClick={handleCreateInvoice}
              >
                <Icon name="receipt" size={16} />
                <span>Terbitkan Faktur Penjualan (Sales Invoice)</span>
              </button>
              <button
                type="button"
                className="btn-dash-secondary"
                onClick={() => window.print()}
              >
                <Icon name="print" size={16} />
                <span>Cetak Sales Order</span>
              </button>
            </NextStepsBar>
          </div>
        )}

        {/* Official Print Signature Sign-off Box */}
        <div className="print-signoff">
          <div className="print-signoff-box">
            <div className="sign-role">Diterima Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Bagian Penjualan / Sales Admin )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Disetujui Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Manajer Penjualan / Sales Head )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Pemesan (Customer)</div>
            <div className="sign-space" />
            <div className="sign-name">( Penanggung Jawab Order )</div>
          </div>
        </div>
      </main>

      {/* Zone 3: Sticky Summary & Action Footer */}
      <footer className="form-zone-3">
        <div className="flex items-center gap-4">
          <span className="status-badge status-draft text-xs">
            <Icon name="check" size={12} /> DOKUMEN PESANAN PENJUALAN (SALES ORDER)
          </span>
          <span className="text-xs text-muted">
            [Ctrl+S] Simpan Order &bull; [Esc] Tutup
          </span>
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <div className="text-xs text-muted">
              DPP: <span className="font-mono">{formatIDR(subtotalCents)}</span> &bull; PPN: <span className="font-mono">{formatIDR(ppnCents)}</span>
            </div>
            <div className="text-sm font-bold text-primary">
              TOTAL PESANAN: <span className="font-mono text-xl text-brand">{formatIDR(totalCents)}</span>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn-dash-secondary"
              onClick={() => workbench.close(tabId)}
            >
              Tutup
            </button>
            {!isExisting && (
              <button
                type="button"
                className="btn-dash-primary"
                disabled={totalCents <= 0 || saving}
                onClick={() => void handleSubmit()}
              >
                {saving ? (
                  <span>Mengonfirmasi Pesanan...</span>
                ) : (
                  <>
                    <Icon name="check" size={16} />
                    <span>KONFIRMASI SALES ORDER (Ctrl+S)</span>
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
