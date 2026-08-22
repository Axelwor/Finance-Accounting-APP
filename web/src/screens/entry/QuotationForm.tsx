import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import { TaxRateSelector, taxForLine } from "../../components/TaxRateSelector";
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

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round((qty > 0 ? qty : 1) * unitPriceCents) - discountCents;
}

export function QuotationForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [activeTab, setActiveTab] = useState<"header" | "items" | "additional">("items");
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
  const [sending, setSending] = useState(false);
  const [savedId, setSavedId] = useState<number | null>(
    entryId != null && !Number.isNaN(Number(entryId)) ? Number(entryId) : null
  );
  const [savedStatus, setSavedStatus] = useState<string>("");

  const isSaved = savedId !== null;

  useEffect(() => {
    void Promise.all([
      api.listCustomers().then(setCustomers),
      api.listItems().then(setItems),
    ]).finally(() => setLoading(false));
  }, []);

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
      setActiveTab("header");
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
      setActiveTab("items");
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
      {/* Zone 1: Sticky Document Header */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="description" size={20} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Penawaran Penjualan (Sales Quotation)</h1>
              <span className="form-header__doc-number">{number}</span>
              <span className={`form-header__status-badge status-${statusLabel.toLowerCase()}`}>
                {statusLabel}
              </span>
            </div>
            <p className="text-xs text-muted mt-0.5">
              Komitmen penawaran harga & estimasi resmi kepada prospek atau pelanggan
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Penawaran (Print)"
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
            <span>Tab 1: Informasi Header & Pihak Utama</span>
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
            <Icon name="book_open" size={14} />
            <span>Tab 3: Syarat, Ketentuan & Info Tambahan</span>
          </button>
        </div>

        {/* TAB 1: INFORMASI HEADER & PELANGGAN */}
        {activeTab === "header" && (
          <div className="form-card form-grid-2col">
            <div className="flex flex-col gap-3">
              <div className="auth-field">
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
                <label>Alamat Penagihan / Korespondensi</label>
                <input
                  type="text"
                  className="input-base input-computed"
                  readOnly
                  value={selectedCustomer ? (selectedCustomer.address || "Alamat belum disetel") : "Pilih pelanggan untuk melihat alamat"}
                />
              </div>

              <div className="auth-field">
                <label>Kontak / Email Pelanggan</label>
                <input
                  type="text"
                  className="input-base input-computed"
                  readOnly
                  value={selectedCustomer ? (selectedCustomer.email || selectedCustomer.phone || "—") : "—"}
                />
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <div className="grid-2col gap-3">
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
                  <label>Berlaku Hingga (Masa Berlaku)</label>
                  <input
                    type="date"
                    className="input-base font-mono"
                    value={validUntil}
                    disabled={isSaved}
                    onChange={(e) => setValidUntil(e.target.value)}
                  />
                </div>
              </div>

              <div className="auth-field">
                <TaxRateSelector
                  value={taxRate}
                  onChange={setTaxRate}
                  disabled={isSaved}
                  label="Skema Pajak Pertambahan Nilai (PPN)"
                />
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: RINCIAN ITEM PRODUK */}
        {activeTab === "items" && (
          <div className="form-card">
            <div className="flex-between mb-3">
              <div>
                <h2 className="text-sm font-bold text-primary">Rincian Barang / Jasa yang Ditawarkan</h2>
                <p className="text-xs text-muted">Daftar item produk, kuantitas, harga penawaran komersial, dan diskon.</p>
              </div>
              {!isSaved && (
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
                    {!isSaved && <th style={{ width: "40px" }}>Aksi</th>}
                  </tr>
                </thead>
                <tbody>
                  {lines.map((line) => (
                    <tr key={line.id}>
                      <td>
                        <select
                          className="input-base text-xs w-full font-semibold"
                          value={line.itemId}
                          disabled={isSaved}
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
                          disabled={isSaved}
                          onChange={(e) => setQty(line.id, Number(e.target.value))}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          className="input-base text-xs text-right font-mono font-semibold w-full"
                          value={line.unitPriceCents}
                          disabled={isSaved}
                          onChange={(e) => setPrice(line.id, parseCents(e.target.value))}
                        />
                      </td>
                      <td className="num">
                        <input
                          type="number"
                          className="input-base text-xs text-right font-mono w-full"
                          value={line.discountCents}
                          disabled={isSaved}
                          onChange={(e) => setDiscount(line.id, parseCents(e.target.value))}
                        />
                      </td>
                      <td className="num font-mono font-bold text-primary">
                        {formatIDR(line.lineTotalCents)}
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
                    {!isSaved && <td />}
                  </tr>
                  <tr>
                    <td colSpan={4} className="text-right font-semibold text-xs text-muted">
                      PPN {taxRate > 0 ? `(${taxRate}%)` : ""}:
                    </td>
                    <td className="num font-mono font-semibold text-secondary text-xs">
                      {formatIDR(ppnCents)}
                    </td>
                    {!isSaved && <td />}
                  </tr>
                  <tr className="total-double">
                    <td colSpan={4} className="text-right font-bold text-xs text-brand">
                      Total Penawaran (Grand Total):
                    </td>
                    <td className="num font-mono font-bold text-brand text-base">
                      {formatIDR(totalCents)}
                    </td>
                    {!isSaved && <td />}
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        )}

        {/* TAB 3: SYARAT & INFORMASI TAMBAHAN */}
        {activeTab === "additional" && (
          <div className="form-card form-grid-2col">
            <div className="flex flex-col gap-3">
              <div className="auth-field">
                <label>Catatan Syarat & Ketentuan (Terms & Conditions)</label>
                <textarea
                  className="input-base"
                  rows={4}
                  placeholder="Contoh: Harga franco gudang, pembayaran 30 hari setelah invoice, garansi resmi 12 bulan..."
                  value={notes}
                  disabled={isSaved}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <div className="auth-field">
                <label>Ketentuan Pengiriman / Franco</label>
                <input
                  type="text"
                  className="input-base"
                  placeholder="Contoh: Franco Jakarta / Pengiriman via Ekspedisi Rekanan"
                  value={shippingTerms}
                  disabled={isSaved}
                  onChange={(e) => setShippingTerms(e.target.value)}
                />
              </div>
              <div className="auth-field">
                <label>Catatan Internal (Tidak Dicetak pada Penawaran Resmi)</label>
                <textarea
                  className="input-base"
                  rows={2}
                  placeholder="Catatan tim internal mengenai diskon khusus atau histori negosiasi..."
                  value={internalRemarks}
                  disabled={isSaved}
                  onChange={(e) => setInternalRemarks(e.target.value)}
                />
              </div>
            </div>
          </div>
        )}

        {/* Post-Save Workflow Chain Actions */}
        {isSaved && (
          <div className="form-card bg-surface-secondary">
            <NextStepsBar number={number} hint={savedStatus || undefined}>
              <button
                type="button"
                className="btn-dash-primary"
                onClick={handleConvertToSO}
              >
                <Icon name="shopping_cart" size={16} />
                <span>Konversi Menjadi Sales Order (SO)</span>
              </button>
              <button
                type="button"
                className="btn-dash-secondary"
                onClick={() => window.print()}
              >
                <Icon name="print" size={16} />
                <span>Cetak Lembar Penawaran</span>
              </button>
              <button
                type="button"
                className="btn-dash-secondary"
                onClick={() => void handleSend()}
                disabled={sending || savedStatus === "SENT" || savedStatus === "CONVERTED"}
              >
                <Icon name="send" size={16} />
                <span>{sending ? "Mengirim..." : "Kirim ke Pelanggan"}</span>
              </button>
            </NextStepsBar>
          </div>
        )}

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

      {/* Zone 3: Sticky Summary & Action Footer */}
      <footer className="form-zone-3">
        <div className="flex items-center gap-4">
          <span className="status-badge status-draft text-xs">
            <Icon name="check" size={12} /> PENYIMPANAN PENAWARAN (TIDAK MEM-POSTING BUKU BESAR)
          </span>
          <span className="text-xs text-muted">
            [Ctrl+S] Simpan Penawaran &bull; [Esc] Tutup
          </span>
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <div className="text-xs text-muted">
              DPP: <span className="font-mono">{formatIDR(subtotalCents)}</span> &bull; PPN: <span className="font-mono">{formatIDR(ppnCents)}</span>
            </div>
            <div className="text-sm font-bold text-primary">
              TOTAL PENAWARAN: <span className="font-mono text-xl text-brand">{formatIDR(totalCents)}</span>
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
            {!isSaved && (
              <button
                type="button"
                className="btn-dash-primary"
                disabled={totalCents <= 0 || saving}
                onClick={() => void handleSubmit()}
              >
                {saving ? (
                  <span>Menyimpan Penawaran...</span>
                ) : (
                  <>
                    <Icon name="check" size={16} />
                    <span>SIMPAN PENAWARAN HARGA (Ctrl+S)</span>
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
