import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import { Icon } from "../../components/m3/Icon";
import type { Customer, Item, SalesOrderLineInput, SalesOrder } from "../../types";
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
  const [activeTab, setActiveTab] = useState<"items" | "header" | "additional">("items");
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
      {/* Zone 1: Sticky Document Header (Compact) */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="shopping_cart" size={16} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Pesanan Penjualan (Sales Order)</h1>
              <span className="form-header__doc-number">{number}</span>
              <span className={`form-header__status-badge status-${statusLabel.toLowerCase()}`}>
                {statusLabel}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-1.5">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Sales Order (Print)"
          >
            <Icon name="print" size={14} />
          </button>
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => workbench.close(tabId)}
            title="Tutup Tab"
          >
            <Icon name="close" size={14} />
          </button>
        </div>
      </header>

      {/* 3-Column Compact Workspace Layout */}
      <div className="form-workspace-layout">
        {/* Left Column: Side Tabs Rail */}
        <aside className="form-side-nav">
          <button
            type="button"
            className={`form-side-tab ${activeTab === "items" ? "is-active" : ""}`}
            onClick={() => setActiveTab("items")}
          >
            <span className="form-side-tab__label">
              <Icon name="package" size={15} />
              <span>Rincian Item</span>
            </span>
            <span className="form-side-tab__badge">
              {lines.filter(l => l.itemId).length}
            </span>
          </button>
          <button
            type="button"
            className={`form-side-tab ${activeTab === "header" ? "is-active" : ""}`}
            onClick={() => setActiveTab("header")}
          >
            <span className="form-side-tab__label">
              <Icon name="building" size={15} />
              <span>Info Pelanggan & Pajak</span>
            </span>
          </button>
          <button
            type="button"
            className={`form-side-tab ${activeTab === "additional" ? "is-active" : ""}`}
            onClick={() => setActiveTab("additional")}
          >
            <span className="form-side-tab__label">
              <Icon name="receipt" size={15} />
              <span>PO, Alamat & Kirim</span>
            </span>
          </button>
        </aside>

        {/* Center Column: Form Panels */}
        <main className="form-center-pane">
          {error && <FormError message={error} />}

          {/* Quick Header Strip */}
          <div className="form-card-compact" style={{ padding: "10px 14px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted">Pelanggan:</span>
              <strong className="text-xs text-primary font-semibold">
                {selectedCustomer ? `${selectedCustomer.code ? selectedCustomer.code + " - " : ""}${selectedCustomer.name}` : "— Belum dipilih —"}
              </strong>
            </div>
            <div className="flex items-center gap-4 text-xs text-muted font-mono">
              <span>Tgl: <strong className="text-secondary">{date}</strong></span>
              <span>Kirim: <strong className="text-secondary">{requestedDeliveryDate || "—"}</strong></span>
            </div>
          </div>

          {/* TAB 1: RINCIAN ITEM BARANG / JASA (DEFAULT) */}
          {activeTab === "items" && (
            <div className="form-card-compact" style={{ padding: "0", overflow: "hidden" }}>
              <div className="form-card-header" style={{ padding: "12px 16px", margin: "0" }}>
                <div>
                  <h2 className="form-card-title">Line Items Pesanan Penjualan</h2>
                </div>
                {!isExisting && (
                  <button
                    type="button"
                    className="btn-dash-primary text-xs"
                    style={{ padding: "4px 10px" }}
                    onClick={addLine}
                  >
                    <Icon name="plus" size={13} />
                    <span>Tambah Item</span>
                  </button>
                )}
              </div>

              <div className="datatable-wrapper" style={{ border: "none", borderRadius: "0" }}>
                <table className="datatable">
                  <thead>
                    <tr>
                      <th style={{ width: "40%" }}>Item Produk / Jasa *</th>
                      <th className="num" style={{ width: "10%" }}>Qty</th>
                      <th className="num" style={{ width: "18%" }}>Harga Satuan (Rp)</th>
                      <th className="num" style={{ width: "14%" }}>Diskon (Rp)</th>
                      <th className="num" style={{ width: "18%" }}>Total Subtotal (Rp)</th>
                      {!isExisting && <th style={{ width: "36px" }} />}
                    </tr>
                  </thead>
                  <tbody>
                    {lines.map((line) => (
                      <tr key={line.id}>
                        <td>
                          <select
                            className="input-base font-semibold"
                            value={line.itemId}
                            disabled={isExisting}
                            onChange={(e) => setItem(line.id, e.target.value)}
                          >
                            <option value="">-- Pilih Produk / Jasa --</option>
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
                            className="input-base text-right font-mono"
                            value={line.qty}
                            disabled={isExisting}
                            onChange={(e) => setQty(line.id, Number(e.target.value))}
                          />
                        </td>
                        <td className="num">
                          <input
                            type="number"
                            className="input-base text-right font-mono font-semibold"
                            value={line.unitPriceCents}
                            disabled={isExisting}
                            onChange={(e) => setPrice(line.id, parseCents(e.target.value))}
                          />
                        </td>
                        <td className="num">
                          <input
                            type="number"
                            className="input-base text-right font-mono"
                            value={line.discountCents}
                            disabled={isExisting}
                            onChange={(e) => setDiscount(line.id, parseCents(e.target.value))}
                          />
                        </td>
                        <td className="num font-mono font-bold text-primary text-xs">
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
                              <Icon name="trash" size={13} />
                            </button>
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* TAB 2: INFORMASI HEADER & PELANGGAN */}
          {activeTab === "header" && (
            <div className="form-card-compact form-grid-2col">
              <div className="flex flex-col gap-2.5">
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

              <div className="flex flex-col gap-2.5">
                <div className="grid-2col gap-2.5">
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
                    <label>Target Pengiriman</label>
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

          {/* TAB 3: PO PELANGGAN, PENGIRIMAN & INFORMASI TAMBAHAN */}
          {activeTab === "additional" && (
            <div className="form-card-compact form-grid-2col">
              <div className="flex flex-col gap-2.5">
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
                    placeholder="Instruksi pengemasan, referensi kontrak..."
                    value={notes}
                    disabled={isExisting}
                    onChange={(e) => setNotes(e.target.value)}
                  />
                </div>
              </div>

              <div className="flex flex-col gap-2.5">
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

        {/* Right Column: Action & Summary Sidebar */}
        <aside className="form-action-sidebar">
          {/* Summary Card */}
          <div className="form-summary-card">
            <div className="flex-between text-xs pb-1 border-b border-subtle">
              <span className="text-muted">DPP Subtotal:</span>
              <strong className="font-mono text-primary">{formatIDR(subtotalCents)}</strong>
            </div>
            <div className="flex-between text-xs pb-1 border-b border-subtle">
              <span className="text-muted">PPN {taxRate > 0 ? `(${taxRate}%)` : ""}:</span>
              <strong className="font-mono text-secondary">{formatIDR(ppnCents)}</strong>
            </div>

            <div className="form-summary-total">
              <span className="form-summary-total__label">Total Pesanan</span>
              <span className="form-summary-total__val">{formatIDR(totalCents)}</span>
            </div>

            {/* Action Buttons */}
            <div className="form-action-stack mt-2">
              {!isExisting && (
                <button
                  type="button"
                  className="form-action-btn-primary"
                  disabled={totalCents <= 0 || saving}
                  onClick={() => void handleSubmit()}
                >
                  {saving ? (
                    <span>Mengonfirmasi...</span>
                  ) : (
                    <>
                      <Icon name="check" size={15} />
                      <span>Konfirmasi Order</span>
                    </>
                  )}
                </button>
              )}

              {isExisting && orderStatus === "CONFIRMED" && (
                <>
                  <button
                    type="button"
                    className="form-action-btn-primary"
                    onClick={handleCreateDelivery}
                  >
                    <Icon name="package" size={15} />
                    <span>Buat Surat Jalan</span>
                  </button>
                  <button
                    type="button"
                    className="form-action-btn-secondary"
                    onClick={handleCreateInvoice}
                  >
                    <Icon name="receipt" size={14} />
                    <span>Terbitkan Faktur</span>
                  </button>
                </>
              )}

              <button
                type="button"
                className="form-action-btn-secondary"
                onClick={() => window.print()}
              >
                <Icon name="print" size={14} />
                <span>Cetak Lembar SO</span>
              </button>

              <button
                type="button"
                className="form-action-btn-secondary text-muted"
                onClick={() => workbench.close(tabId)}
              >
                <span>Tutup Tab Form</span>
              </button>
            </div>

            <div className="text-[11px] text-muted text-center mt-2 leading-tight">
              Pintasan: <strong>Ctrl+S</strong> simpan &bull; <strong>Esc</strong> tutup
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
