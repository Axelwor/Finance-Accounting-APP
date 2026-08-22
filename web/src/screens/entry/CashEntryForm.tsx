import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useAppState } from "../../state";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDR } from "../../lib/format";
import { Icon } from "../../components/m3/Icon";
import type { BackendAccount, CounterLinePayload, EntrySubKind, JournalEntryListItem } from "../../types";

interface Props {
  tabId: string;
  subKind: EntrySubKind;
  entryId?: string | number;
  initialTitle?: string;
}

interface CounterLine {
  id: string;
  accountId: string;
  amount: string;
  memo: string;
}

let lineSeq = 0;
function seedLine(): CounterLine {
  lineSeq += 1;
  return { id: `cl-${Date.now()}-${lineSeq}`, accountId: "", amount: "", memo: "" };
}

function parseCents(digits: string): number {
  const clean = digits.replace(/[^\d]/g, "");
  return clean ? parseInt(clean, 10) : 0;
}

export function CashEntryForm({ tabId, subKind, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  let business = null;
  try {
    const appState = useAppState();
    business = appState?.business ?? null;
  } catch {
    // optional outside AppStateProvider (e.g. tests)
  }
  const isDetail = entryId !== undefined;
  const isReceipt = subKind === "money-in";

  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [recentEntries, setRecentEntries] = useState<JournalEntryListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [draftRestored, setDraftRestored] = useState(false);

  // Form Fields
  const [date, setDate] = useState(mockHelpers.today());
  const [cashAccountId, setCashAccountId] = useState("");
  const [counterparty, setCounterparty] = useState("");
  const [description, setDescription] = useState("");
  const [reference, setReference] = useState("");
  const [lines, setLines] = useState<CounterLine[]>([seedLine()]);
  const [docNumber, setDocNumber] = useState(initialTitle ?? (isReceipt ? "BKM-2026/DRAFT" : "BKK-2026/DRAFT"));
  const [status, setStatus] = useState(isDetail ? "POSTED" : "DRAFT");

  const draftKey = `ledgerly.draft.${tabId}`;
  const isInitialMount = useRef(true);

  // 1. Restore draft on mount if not detail
  useEffect(() => {
    if (!isDetail) {
      try {
        const savedDraft = localStorage.getItem(draftKey);
        if (savedDraft) {
          const parsed = JSON.parse(savedDraft);
          if (parsed.date) setDate(parsed.date);
          if (parsed.cashAccountId) setCashAccountId(parsed.cashAccountId);
          if (parsed.counterparty) setCounterparty(parsed.counterparty);
          if (parsed.description) setDescription(parsed.description);
          if (parsed.reference) setReference(parsed.reference);
          if (Array.isArray(parsed.lines) && parsed.lines.length > 0) setLines(parsed.lines);
          setDraftRestored(true);
          setTimeout(() => setDraftRestored(false), 3500);
        }
      } catch (e) {
        console.warn("Failed to restore draft", e);
      }
    }
  }, [isDetail, draftKey]);

  // 2. Debounced draft autosave
  useEffect(() => {
    if (isDetail || saved) return;
    if (isInitialMount.current) {
      isInitialMount.current = false;
      return;
    }

    const timer = setTimeout(() => {
      try {
        const payload = { date, cashAccountId, counterparty, description, reference, lines };
        localStorage.setItem(draftKey, JSON.stringify(payload));
      } catch (e) {
        console.warn("Failed to autosave draft", e);
      }
    }, 800);

    return () => clearTimeout(timer);
  }, [date, cashAccountId, counterparty, description, reference, lines, isDetail, saved, draftKey]);

  useEffect(() => {
    void loadInitialData();
  }, []);

  const loadInitialData = async () => {
    setLoading(true);
    try {
      const [accs, recent] = await Promise.all([
        api.listBackendAccounts(),
        api.listRecentJournalEntries(100).catch(() => []),
      ]);
      const active = accs.filter((a) => a.is_active && !a.is_group);
      setAccounts(active);
      setRecentEntries(recent);
      const cashAcc = active.find((a) => a.account_type === "CASH" || a.account_type === "BANK");
      if (cashAcc && !cashAccountId) setCashAccountId(String(cashAcc.id));
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Gagal memuat data formulir kas.");
    } finally {
      setLoading(false);
    }
  };

  // Keyboard shortcut: Ctrl/Cmd+S to save, Escape to close (only for active nested tab)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Gate to ensure only the currently active nested tab handles the event
      if (workbench.activeNested?.id && workbench.activeNested.id !== tabId) return;

      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (!saving && !saved && !isDetail) {
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
  }, [workbench.activeNested?.id, saving, saved, isDetail, tabId, cashAccountId, lines, date, description, counterparty, reference]);

  const cashAccounts = useMemo(
    () => accounts.filter((a) => a.account_type === "CASH" || a.account_type === "BANK" || a.code.startsWith("11")),
    [accounts]
  );

  const expenseOrRevenueAccounts = useMemo(
    () => accounts.filter((a) => !cashAccounts.some((c) => c.id === a.id)),
    [accounts, cashAccounts]
  );

  const totalAmount = useMemo(
    () => lines.reduce((sum, l) => sum + parseCents(l.amount), 0),
    [lines]
  );

  // Period-lock warning
  const periodLockWarning = useMemo(() => {
    if (!date) return null;
    const entryYear = new Date(date).getFullYear();
    const currentYear = new Date().getFullYear();
    const fiscalStartYear = business?.fiscalYearStart ? new Date(business.fiscalYearStart).getFullYear() : currentYear;

    if (entryYear !== fiscalStartYear && entryYear !== currentYear) {
      return `Tanggal di luar periode buku aktif (FY ${fiscalStartYear || currentYear}) — periksa kembali.`;
    }
    return null;
  }, [date, business]);

  // Duplicate reference check
  const duplicateRefWarning = useMemo(() => {
    if (!reference || !reference.trim()) return null;
    const trimmed = reference.trim().toLowerCase();
    const found = recentEntries.find(
      (entry) =>
        (entry.source_ref && entry.source_ref.trim().toLowerCase() === trimmed) ||
        (entry.number && entry.number.trim().toLowerCase() === trimmed)
    );
    if (found) {
      return `No. Referensi "${reference}" sudah pernah digunakan pada jurnal ${found.number}.`;
    }
    return null;
  }, [reference, recentEntries]);

  const addLine = () => setLines((prev) => [...prev, seedLine()]);

  const updateLine = (id: string, field: keyof CounterLine, val: string) => {
    setLines((prev) =>
      prev.map((l) => (l.id === id ? { ...l, [field]: val } : l))
    );
  };

  const removeLine = (id: string) => {
    if (lines.length <= 1) return;
    setLines((prev) => prev.filter((l) => l.id !== id));
  };

  const handlePost = async () => {
    if (!cashAccountId) {
      setError("Pilih rekening Kas/Bank utama.");
      return;
    }
    if (totalAmount <= 0) {
      setError("Nominal transaksi harus lebih dari Rp 0.");
      return;
    }
    const invalidLines = lines.some((l) => !l.accountId || parseCents(l.amount) <= 0);
    if (invalidLines) {
      setError("Pastikan semua baris alokasi memiliki akun dan nominal valid.");
      return;
    }

    setError(null);
    setSaving(true);
    try {
      const counterLines: CounterLinePayload[] = lines.map((l) => ({
        account_id: Number(l.accountId),
        amount_cents: parseCents(l.amount),
        description: l.memo.trim() || description.trim() || "Alokasi kas",
      }));

      let res;
      if (isReceipt) {
        res = await api.postCashIn({
          entry_date: date,
          description: description.trim() || (counterparty ? `Kas Masuk dari ${counterparty}` : "Kas Masuk Operasional"),
          cash_account_id: Number(cashAccountId),
          counter_account_id: Number(counterLines[0].account_id),
          amount_cents: totalAmount,
          counter_lines: counterLines,
        });
      } else {
        res = await api.postCashOut({
          entry_date: date,
          description: description.trim() || (counterparty ? `Kas Keluar kepada ${counterparty}` : "Kas Keluar Operasional"),
          cash_account_id: Number(cashAccountId),
          counter_account_id: Number(counterLines[0].account_id),
          amount_cents: totalAmount,
          counter_lines: counterLines,
        });
      }

      setSaved(true);
      setStatus("POSTED");
      if (res?.number) setDocNumber(res.number);
      try {
        localStorage.removeItem(draftKey);
      } catch (e) {
        console.warn("Failed to clear autosaved draft", e);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mem-posting transaksi kas.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <LoadingState label="Memuat formulir kas..." />;
  if (loadError) return <ErrorState message={loadError} onRetry={loadInitialData} />;

  const cashAccountName = accounts.find((a) => String(a.id) === cashAccountId)?.name || "Kas/Bank Utama";

  return (
    <div className="enterprise-form">
      {/* Zone 1: Sticky Document Header */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon
              name={isReceipt ? "arrow_down_left" : "arrow_up_right"}
              size={20}
              className={isReceipt ? "text-success" : "text-danger"}
            />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">
                {isReceipt ? "Kas Masuk Operasional (Other Receipt)" : "Kas Keluar Operasional (Other Payment)"}
              </h1>
              <span className="form-header__doc-number">{docNumber}</span>
              <span className={`form-header__status-badge status-${status.toLowerCase()}`}>
                {status}
              </span>
              {draftRestored && (
                <span className="status-badge-inline status-open font-semibold text-xs">
                  ✓ Draft Dipulihkan
                </span>
              )}
            </div>
            <p className="text-xs text-muted mt-0.5">
              {isReceipt ? "Pencatatan penerimaan kas non-faktur (pendapatan lain, bunga, setoran)" : "Pencatatan beban & pengeluaran kas non-faktur"}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Bukti Kas"
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

      {/* Zone 2: Dynamic Form Body */}
      <main className="form-zone-2">
        {periodLockWarning && (
          <div
            style={{
              padding: "10px 14px",
              backgroundColor: "var(--color-warning-bg)",
              border: "1px solid var(--color-warning-border)",
              borderRadius: "var(--radius-sm)",
              color: "var(--color-warning-text)",
              fontSize: "12.5px",
              display: "flex",
              alignItems: "center",
              gap: "8px",
            }}
          >
            <Icon name="warning" size={16} />
            <span>{periodLockWarning}</span>
          </div>
        )}

        {error && <FormError message={error} />}

        {/* 2.A Primary Entity & Header Fields */}
        <div className="form-card form-grid-2col">
          {/* Kolom Kiri: Pihak Utama & Rekening */}
          <div className="flex flex-col gap-3">
            <div className="auth-field">
              <label>Rekening Kas / Bank Sumber Transaksi *</label>
              <select
                className="input-base font-semibold"
                value={cashAccountId}
                disabled={isDetail || saved}
                onChange={(e) => setCashAccountId(e.target.value)}
              >
                {cashAccounts.map((acc) => (
                  <option key={acc.id} value={acc.id}>
                    {acc.code} - {acc.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="auth-field">
              <label>{isReceipt ? "Diterima Dari (Pihak Pembayar)" : "Dibayarkan Kepada (Penerima Kas)"}</label>
              <input
                type="text"
                className="input-base"
                placeholder={isReceipt ? "Nama pelanggan / mitra / sumber dana" : "Nama vendor / staf / penerima pembayaran"}
                value={counterparty}
                disabled={isDetail || saved}
                onChange={(e) => setCounterparty(e.target.value)}
              />
            </div>
          </div>

          {/* Kolom Kanan: Tanggal & Referensi */}
          <div className="flex flex-col gap-3">
            <div className="auth-field">
              <label>Tanggal Transaksi *</label>
              <input
                type="date"
                className="input-base font-mono"
                value={date}
                disabled={isDetail || saved}
                onChange={(e) => setDate(e.target.value)}
              />
            </div>

            <div className="auth-field">
              <div className="flex-between">
                <label>No. Referensi / No. Bukti Fisik</label>
                {duplicateRefWarning && (
                  <span className="text-xs text-amber-600 font-semibold flex items-center gap-1">
                    <Icon name="warning" size={12} /> Referensi duplikat
                  </span>
                )}
              </div>
              <input
                type="text"
                className="input-base font-mono"
                placeholder="Contoh: REF-BCA-8891 / KWT-002"
                value={reference}
                disabled={isDetail || saved}
                onChange={(e) => setReference(e.target.value)}
              />
              {duplicateRefWarning && (
                <p className="text-xs text-amber-600 mt-1">{duplicateRefWarning}</p>
              )}
            </div>
          </div>
        </div>

        {/* 2.B Line Items Table Engine (Multi-Account Allocation Grid) */}
        <div className="form-card">
          <div className="flex-between mb-3">
            <div>
              <h2 className="text-sm font-bold text-primary">
                {isReceipt ? "Alokasi Akun Penerimaan / Pendapatan" : "Alokasi Akun Pengeluaran / Beban"}
              </h2>
              <p className="text-xs text-muted">Tentukan satu atau beberapa akun lawan untuk pencatatan transaksi ini.</p>
            </div>
            {!isDetail && !saved && (
              <button
                type="button"
                className="btn-dash-secondary text-xs"
                onClick={addLine}
              >
                <Icon name="plus" size={14} />
                <span>+ Tambah Akun Lawan (Enter)</span>
              </button>
            )}
          </div>

          <div className="datatable-wrapper">
            <table className="datatable">
              <thead>
                <tr>
                  <th style={{ width: "40%" }}>Akun Lawan Transaksi *</th>
                  <th>Keterangan / Memo Baris</th>
                  <th className="num" style={{ width: "22%" }}>Nominal (Rp) *</th>
                  {!isDetail && !saved && <th style={{ width: "50px" }}>Aksi</th>}
                </tr>
              </thead>
              <tbody>
                {lines.map((line, idx) => (
                  <tr key={line.id}>
                    <td>
                      <select
                        className="input-base text-xs w-full"
                        value={line.accountId}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "accountId", e.target.value)}
                      >
                        <option value="">-- Pilih Akun ({idx + 1}) --</option>
                        {expenseOrRevenueAccounts.map((acc) => (
                          <option key={acc.id} value={acc.id}>
                            {acc.code} - {acc.name} ({acc.account_type})
                          </option>
                        ))}
                      </select>
                    </td>
                    <td>
                      <input
                        type="text"
                        className="input-base text-xs w-full"
                        placeholder="Contoh: Pembayaran internet kantor bulan ini"
                        value={line.memo}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "memo", e.target.value)}
                      />
                    </td>
                    <td className="num">
                      <input
                        type="number"
                        className="input-base text-xs text-right font-mono font-semibold w-full"
                        placeholder="0"
                        value={line.amount}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "amount", e.target.value)}
                      />
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
                  <td colSpan={2} className="font-bold text-right text-xs">
                    Total Nominal:
                  </td>
                  <td className="num font-mono font-bold text-primary text-sm">
                    {formatIDR(totalAmount)}
                  </td>
                  {!isDetail && !saved && <td />}
                </tr>
              </tfoot>
            </table>
          </div>
        </div>

        {/* 2.C Live Journal Preview */}
        <div className="form-card bg-surface-secondary">
          <div className="flex items-center gap-2 mb-2">
            <Icon name="security" size={16} className="text-brand" />
            <h3 className="text-xs font-bold text-primary uppercase">Pratinjau Otomatis Jurnal Buku Besar (Live Journal Effect)</h3>
          </div>
          <div className="text-xs font-mono space-y-1">
            {isReceipt ? (
              <>
                <p className="text-emerald-700 font-semibold">
                  (Dr) {cashAccountName}: <strong>{formatIDR(totalAmount)}</strong>
                </p>
                {lines.map((l, i) => {
                  const acc = accounts.find((a) => String(a.id) === l.accountId);
                  return (
                    <p key={i} className="text-slate-600 pl-4">
                      (Cr) {acc ? `${acc.code} - ${acc.name}` : "[Pilih Akun Lawan]"}: {formatIDR(parseCents(l.amount))}
                    </p>
                  );
                })}
              </>
            ) : (
              <>
                {lines.map((l, i) => {
                  const acc = accounts.find((a) => String(a.id) === l.accountId);
                  return (
                    <p key={i} className="text-slate-700 font-semibold">
                      (Dr) {acc ? `${acc.code} - ${acc.name}` : "[Pilih Akun Lawan]"}: {formatIDR(parseCents(l.amount))}
                    </p>
                  );
                })}
                <p className="text-rose-700 font-semibold pl-4">
                  (Cr) {cashAccountName}: <strong>{formatIDR(totalAmount)}</strong>
                </p>
              </>
            )}
          </div>
        </div>
        {/* 2.D Official Print Signature Sign-off Box */}
        <div className="print-signoff">
          <div className="print-signoff-box">
            <div className="sign-role">Dibuat Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Staf Kasir / Pembuat )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Diperiksa Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Supervisor / Akuntan )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Disetujui Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Manajer Keuangan )</div>
          </div>
        </div>
      </main>

      {/* Zone 3: Sticky Summary & Action Footer */}
      <footer className="form-zone-3">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold text-secondary">Integritas Transaksi:</span>
            {totalAmount > 0 ? (
              <span className="status-badge status-balanced text-xs">
                <Icon name="check" size={12} /> SEIMBANG (DEBIT = KREDIT)
              </span>
            ) : (
              <span className="status-badge status-draft text-xs">
                Draft Kosong
              </span>
            )}
          </div>
          <div className="text-xs text-muted">
            <span>Shortcut: [Ctrl+S] Posting Kas &bull; [Esc] Batal</span>
          </div>
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <span className="text-xs text-muted block">TOTAL TRANSAKSI KAS:</span>
            <span className="font-mono text-xl font-bold text-primary total-double inline-block">{formatIDR(totalAmount)}</span>
          </div>

          <div className="flex items-center gap-2">
            {!isDetail && !saved && (
              <button
                type="button"
                className="btn-dash-primary"
                disabled={totalAmount <= 0 || saving}
                onClick={handlePost}
              >
                {saving ? (
                  <span>Menyimpan ke Buku Kas...</span>
                ) : (
                  <>
                    <Icon name="check" size={14} />
                    <span>Posting Transaksi Kas</span>
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

