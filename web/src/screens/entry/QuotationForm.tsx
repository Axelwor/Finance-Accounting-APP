import { useCallback, useEffect, useMemo, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDRFromCents, parseRupiahToCents } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
import { CurrencyRatePicker } from "../../components/CurrencyRatePicker";
import { Icon } from "../../components/m3/Icon";
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

let lineSeq = 0;
function seedLine(): Line {
  lineSeq += 1;
  return {
    id: `ln-${Date.now()}-${lineSeq}`,
    itemId: "",
    itemCode: "",
    itemName: "",
    qty: 1,
    unitPriceCents: 0,
    discountCents: 0,
    lineTotalCents: 0,
  };
}

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round((qty > 0 ? qty : 1) * unitPriceCents) - discountCents;
}

export function QuotationForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [activeTab, setActiveTab] = useState<"items" | "additional">("items");
  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [number, setNumber] = useState(initialTitle ?? draftNumber("sales-quotation-entry"));
  const [customerId, setCustomerId] = useState("");
  const [validUntil, setValidUntil] = useState("");
  const [notes, setNotes] = useState("");
  const [shippingTerms, setShippingTerms] = useState("");
  const [internalRemarks, setInternalRemarks] = useState("");
  const [lines, setLines] = useState<Line[]>([seedLine()]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [taxRate, setTaxRate] = useState(0);
  const [currencyCode, setCurrencyCode] = useState("IDR");
  const [exchangeRate, setExchangeRate] = useState(1);
  const [sending, setSending] = useState(false);
  const [savedId, setSavedId] = useState<number | null>(
    entryId != null && !Number.isNaN(Number(entryId)) ? Number(entryId) : null
  );
  const [savedStatus, setSavedStatus] = useState<string>("");

  const isSaved = savedId !== null;

  const loadMasterData = useCallback(() => {
    void Promise.all([
      api.listCustomers().then(setCustomers),
      api.listItems().then(setItems),
    ]).finally(() => setLoading(false));
  }, []);
  useEffect(() => {
    loadMasterData();
  }, [loadMasterData]);
  useTabRefresh(loadMasterData);

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    void api
      .getQuotation(id)
      .then((q) => {
        setNumber(q.number);
        setSavedStatus(q.status);
        setDate(q.quotation_date);
        setValidUntil(q.valid_until ?? "");
        setCustomerId(String(q.customer_id));
        setNotes(q.notes ?? "");
        setLines(
          q.lines.length > 0
            ? q.lines.map((l) => ({
                id: `ln-${l.id}`,
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
        workbench.markUnsaved(tabId, false);
      })
      .catch(() => {});
  }, [entryId, tabId, workbench]);

  const subtotalCents = useMemo(() => lines.reduce((sum, l) => sum + l.lineTotalCents, 0), [lines]);
  const ppnCents = useMemo(
    () => lines.reduce((sum, l) => sum + taxForLine(l.lineTotalCents, taxRate), 0),
    [lines, taxRate]
  );
  const totalCents = subtotalCents + ppnCents;

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
    if (isSaved) return;
    if (!customerId) {
      setError("Pilih pelanggan untuk penawaran harga ini.");
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
      setError("Tambahkan minimal 1 baris item produk.");
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
        currency_code: currencyCode,
        exchange_rate: currencyCode === "IDR" ? 1 : exchangeRate,
      });
      workbench.replaceDraft(tabId, created.number, "DRAFT");
      workbench.markUnsaved(tabId, false);
      setSavedId(created.id);
      setSavedStatus("DRAFT");
      setNumber(created.number);
      toast.success(`✓ Tersimpan — ${created.number}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan penawaran harga.");
    } finally {
      setSaving(false);
    }
  };

  const handleConvertToSO = () => {
    if (!savedId) return;
    workbench.openEntryDraftFromParent("sales-order-entry", { kind: "quotation", id: savedId });
  };

  const handleSend = async () => {
    if (!savedId) return;
    setSending(true);
    try {
      const res = await api.sendQuotation(savedId);
      setSavedStatus(res.status);
      workbench.replaceDraft(tabId, number, res.status);
      toast.success(`Penawaran ${number} berhasil dikirim ke pelanggan`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Gagal mengirim penawaran.");
    } finally {
      setSending(false);
    }
  };

  // Keyboard shortcuts: Ctrl+S to save/post, Esc to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (workbench.activeNested?.id && workbench.activeNested.id !== tabId) return;

      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (!saving && !isSaved && totalCents > 0) {
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
  }, [workbench.activeNested?.id, saving, isSaved, tabId, totalCents, lines, date, customerId, validUntil, notes, taxRate]);

  if (loading) return <LoadingState label="Memuat formulir penawaran..." />;

  const statusLabel = isSaved ? savedStatus || "DRAFT" : "DRAFT";
  const selectedCustomer = customers.find((c) => String(c.id) === customerId);

  return (
    <div className="enterprise-form">
      {/* Zone 1: Sticky Corporate Bar */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="description" size={16} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Penawaran Penjualan (Sales Quotation)</h1>
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
            title="Cetak Penawaran (Print)"
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

      {/* Zone 2: Main Body */}
      <main className="form-zone-2">
        {error && <FormError message={error} />}

        {/* 2.A Primary Header Card: Customer Selection & Document Dates (Always at Top) */}
        <div className="form-primary-header-card">
          <div className="form-header-grid">
            <div className="auth-field" style={{ gridColumn: "span 2" }}>
              <label>Pelanggan / Mitra Usaha (Customer) *</label>
              <select
                className="input-base font-semibold"
                value={customerId}
                disabled={isSaved}
                onChange={(e) => setCustomerId(e.target.value)}
              >
                <option value="">-- Pilih Pelanggan / Mitra Usaha --</option>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.code ? `${c.code} - ` : ""}{c.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="auth-field">
              <label>Tanggal Dokumen *</label>
              <input
                type="date"
                className="input-base font-mono"
                value={date}
                disabled={isSaved}
                onChange={(e) => setDate(e.target.value)}
              />
            </div>

            <div className="auth-field">
              <label>Masa Berlaku</label>
              <input
                type="date"
                className="input-base font-mono"
                value={validUntil}
                disabled={isSaved}
                onChange={(e) => setValidUntil(e.target.value)}
              />
            </div>

            <div className="auth-field">
              <TaxRateSelector
                value={taxRate}
                onChange={setTaxRate}
                disabled={isSaved}
                label="Skema PPN"
              />
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
                disabled={isSaved}
              />
            </div>
          </div>

          {selectedCustomer && (
            <div className="form-header-meta">
              <span>Alamat: <strong>{selectedCustomer.address || "—"}</strong></span>
              <span>Kontak: <strong>{selectedCustomer.phone || selectedCustomer.email || "—"}</strong></span>
            </div>
          )}
        </div>

        {/* 2.B Side-Tab Icon Rail + Tabbed Content Area */}
        <div className="form-tabbed-body">
          {/* Icon-Only Side Rail */}
          <aside className="form-side-icon-rail">
            <button
              type="button"
              className={`form-side-icon-btn ${activeTab === "items" ? "is-active" : ""}`}
              onClick={() => setActiveTab("items")}
              title="Rincian Item Barang & Jasa"
            >
              <Icon name="package" size={18} />
              <span className="form-side-icon-btn__badge">
                {lines.filter(l => l.itemId).length}
              </span>
            </button>
            <button
              type="button"
              className={`form-side-icon-btn ${activeTab === "additional" ? "is-active" : ""}`}
              onClick={() => setActiveTab("additional")}
              title="Syarat, Ketentuan & Catatan"
            >
              <Icon name="description" size={18} />
            </button>
          </aside>

          {/* Tab Content Panel */}
          <div className="form-tab-content">
            {/* TAB 1: RINCIAN ITEM PRODUK (DEFAULT) */}
            {activeTab === "items" && (
              <div className="form-card-compact p-0 overflow-hidden">
                <div className="form-card-header" style={{ padding: "8px 10px", marginBottom: 0 }}>
                  <h2 className="form-card-title">Daftar Barang &amp; Jasa yang Ditawarkan</h2>
                  {!isSaved && (
                    <button type="button" className="btn-dash-primary" onClick={addLine}>
                      <Icon name="plus" size={12} />
                      <span>Tambah Baris</span>
                    </button>
                  )}
                </div>

                <div className="datatable-wrapper" style={{ border: "none", borderRadius: 0 }}>
                  <table className="datatable">
                    <thead>
                      <tr>
                        <th style={{ width: "40%" }}>Item Produk / Jasa *</th>
                        <th className="num" style={{ width: "9%" }}>Qty</th>
                        <th className="num" style={{ width: "17%" }}>Harga Satuan</th>
                        <th className="num" style={{ width: "14%" }}>Diskon</th>
                        <th className="num" style={{ width: "17%" }}>Subtotal</th>
                        {!isSaved && <th style={{ width: "34px" }} />}
                      </tr>
                    </thead>
                    <tbody>
                      {lines.map((line) => (
                        <tr key={line.id}>
                          <td>
                            <select
                              className="input-base font-semibold"
                              value={line.itemId}
                              disabled={isSaved}
                              onChange={(e) => setItem(line.id, e.target.value)}
                            >
                              <option value="">-- Pilih Item Produk / Jasa --</option>
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
                              disabled={isSaved}
                              onChange={(e) => setQty(line.id, Number(e.target.value))}
                            />
                          </td>
                          <td className="num">
                            <input
                              type="number"
                              className="input-base text-right font-mono font-semibold"
                              min="0"
                              value={line.unitPriceCents / 100}
                              disabled={isSaved}
                              onChange={(e) => setPrice(line.id, parseRupiahToCents(e.target.value))}
                            />
                          </td>
                          <td className="num">
                            <input
                              type="number"
                              className="input-base text-right font-mono"
                              min="0"
                              value={line.discountCents / 100}
                              disabled={isSaved}
                              onChange={(e) => setDiscount(line.id, parseRupiahToCents(e.target.value))}
                            />
                          </td>
                          <td className="num font-mono font-bold text-primary">
                            {formatIDRFromCents(line.lineTotalCents)}
                          </td>
                          {!isSaved && (
                            <td className="text-center">
                              <button
                                type="button"
                                className="topbar__icon-btn text-danger"
                                disabled={lines.length <= 1}
                                onClick={() => removeLine(line.id)}
                                title="Hapus baris"
                              >
                                <Icon name="trash" size={12} />
                              </button>
                            </td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                    <tfoot>
                      <tr className="total-rule-top">
                        <td colSpan={4} className="text-right font-semibold text-secondary">
                          DPP (Subtotal)
                        </td>
                        <td className="num font-mono font-bold text-primary">
                          {formatIDRFromCents(subtotalCents)}
                        </td>
                        {!isSaved && <td />}
                      </tr>
                      <tr>
                        <td colSpan={4} className="text-right font-semibold text-muted">
                          PPN {taxRate > 0 ? `${taxRate}%` : ""}
                        </td>
                        <td className="num font-mono font-semibold text-secondary">
                          {formatIDRFromCents(ppnCents)}
                        </td>
                        {!isSaved && <td />}
                      </tr>
                      <tr className="total-double">
                        <td colSpan={4} className="text-right font-bold text-brand">
                          Total Penawaran
                        </td>
                        <td className="num font-mono font-bold text-brand text-sm">
                          {formatIDRFromCents(totalCents)}
                        </td>
                        {!isSaved && <td />}
                      </tr>
                    </tfoot>
                  </table>
                </div>
              </div>
            )}

            {/* TAB 2: SYARAT & INFORMASI TAMBAHAN */}
            {activeTab === "additional" && (
              <div className="form-card-compact form-grid-2col">
                <div className="auth-field">
                  <label>Syarat &amp; Ketentuan (Terms &amp; Conditions)</label>
                  <textarea
                    className="input-base"
                    rows={4}
                    placeholder="Contoh: Harga franco Jakarta, pembayaran 30 hari..."
                    value={notes}
                    disabled={isSaved}
                    onChange={(e) => setNotes(e.target.value)}
                  />
                </div>

                <div className="flex flex-col gap-2">
                  <div className="auth-field">
                    <label>Ketentuan Pengiriman / Franco</label>
                    <input
                      type="text"
                      className="input-base"
                      placeholder="Contoh: Franco Jakarta"
                      value={shippingTerms}
                      disabled={isSaved}
                      onChange={(e) => setShippingTerms(e.target.value)}
                    />
                  </div>
                  <div className="auth-field">
                    <label>Catatan Internal (tidak dicetak)</label>
                    <textarea
                      className="input-base"
                      rows={2}
                      placeholder="Catatan tim internal..."
                      value={internalRemarks}
                      disabled={isSaved}
                      onChange={(e) => setInternalRemarks(e.target.value)}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Official Print Signature Sign-off Box */}
        <div className="print-signoff">
          <div className="print-signoff-box">
            <div className="sign-role">Dibuat Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Bagian Penjualan / Sales Rep )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Disetujui Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Manajer Pemasaran / Sales Mgr )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Diterima &amp; Disetujui Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Pelanggan / Prospek )</div>
          </div>
        </div>
      </main>

      {/* Zone 3: Sticky Bottom Footer (Subtotal, Total & Primary Actions) */}
      <footer className="form-zone-3">
        <div className="form-zone-3__summary">
          <span className="text-xs text-muted">
            DPP <strong className="font-mono text-secondary">{formatIDRFromCents(subtotalCents)}</strong>
            {"  ·  "}
            PPN <strong className="font-mono text-secondary">{formatIDRFromCents(ppnCents)}</strong>
          </span>
          <div className="form-zone-3__total-block">
            <span className="form-zone-3__total-label">Total</span>
            <span className="form-zone-3__total-val">{formatIDRFromCents(totalCents)}</span>
          </div>
        </div>

        <div className="form-zone-3__actions">
          {!isSaved ? (
            <button
              type="button"
              className="btn-dash-primary"
              disabled={totalCents <= 0 || saving}
              onClick={() => void handleSubmit()}
            >
              {saving ? (
                <span>Menyimpan…</span>
              ) : (
                <>
                  <Icon name="check" size={13} />
                  <span>Simpan Penawaran</span>
                  <kbd className="btn-kbd">Ctrl+S</kbd>
                </>
              )}
            </button>
          ) : (
            <>
              <button
                type="button"
                className="btn-dash-secondary"
                onClick={() => void handleSend()}
                disabled={sending || savedStatus === "SENT" || savedStatus === "CONVERTED"}
              >
                <Icon name="send" size={12} />
                <span>{sending ? "Mengirim…" : "Kirim ke Pelanggan"}</span>
              </button>
              <button type="button" className="btn-dash-primary" onClick={handleConvertToSO}>
                <Icon name="shopping_cart" size={13} />
                <span>Konversi ke Sales Order</span>
              </button>
            </>
          )}
        </div>
      </footer>
    </div>
  );
}
