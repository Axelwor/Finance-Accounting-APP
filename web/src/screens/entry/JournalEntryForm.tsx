import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useAppState } from "../../state";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDRFromCents, parseRupiahToCents } from "../../lib/format";
import { Icon } from "../../components/m3/Icon";
import type { BackendAccount, JournalEntry, JournalEntryListItem } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface FormLine {
  id: string;
  accountId: string;
  debit: string;
  credit: string;
  description: string;
}

let lineSeq = 0;
function seedLine(): FormLine {
  lineSeq += 1;
  return { id: `jl-${Date.now()}-${lineSeq}`, accountId: "", debit: "", credit: "", description: "" };
}

export function JournalEntryForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  let business = null;
  try {
    const appState = useAppState();
    business = appState?.business ?? null;
  } catch {
    // optional outside AppStateProvider (e.g. tests)
  }
  const isDetail = entryId !== undefined;
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [recentEntries, setRecentEntries] = useState<JournalEntryListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [draftRestored, setDraftRestored] = useState(false);

  const [date, setDate] = useState(mockHelpers.today());
  const [description, setDescription] = useState("");
  const [reference, setReference] = useState("");
  const [lines, setLines] = useState<FormLine[]>([seedLine(), seedLine()]);
  const [number, setNumber] = useState(initialTitle ?? "JE-2026/DRAFT");
  const [status, setStatus] = useState(isDetail ? "POSTED" : "DRAFT");

  const draftKey = `ledgerly.draft.${tabId}`;
  const isInitialMount = useRef(true);
  const tableRef = useRef<HTMLTableElement>(null);

  // Restore draft on mount if not detail
  useEffect(() => {
    if (!isDetail) {
      try {
        const savedDraft = localStorage.getItem(draftKey);
        if (savedDraft) {
          const parsed = JSON.parse(savedDraft);
          if (parsed.date) setDate(parsed.date);
          if (parsed.description) setDescription(parsed.description);
          if (parsed.reference) setReference(parsed.reference);
          if (Array.isArray(parsed.lines) && parsed.lines.length >= 2) setLines(parsed.lines);
          setDraftRestored(true);
          setTimeout(() => setDraftRestored(false), 3500);
        }
      } catch (e) {
        console.warn("Failed to restore draft", e);
      }
    }
  }, [isDetail, draftKey]);

  // Debounced autosave
  useEffect(() => {
    if (isDetail || saved) return;
    if (isInitialMount.current) {
      isInitialMount.current = false;
      return;
    }

    const timer = setTimeout(() => {
      try {
        const payload = { date, description, reference, lines };
        localStorage.setItem(draftKey, JSON.stringify(payload));
      } catch (e) {
        console.warn("Failed to autosave draft", e);
      }
    }, 800);

    return () => clearTimeout(timer);
  }, [date, description, reference, lines, isDetail, saved, draftKey]);

  useEffect(() => {
    void loadMasters();
    if (isDetail && entryId) void loadDetail(Number(entryId));
  }, []);

  const loadMasters = async () => {
    setLoading(true);
    try {
      const [accs, recent] = await Promise.all([
        api.listBackendAccounts(),
        api.listRecentJournalEntries(100).catch(() => []),
      ]);
      setAccounts(accs.filter((a) => a.is_active && !a.is_group));
      setRecentEntries(recent);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Gagal memuat bagan akun.");
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: number) => {
    try {
      const entry = await api.getJournalEntry(id);
      if (!entry) return;
      setNumber(entry.number);
      setStatus(entry.status);
      setDate(entry.entry_date);
      setDescription(entry.description);
      if (entry.source_ref) setReference(entry.source_ref);
      setLines(
        entry.lines.map((l, idx) => ({
          id: `line-${idx}`,
          accountId: String(l.account_id),
          // Backend stores cents; the input shows whole rupiah.
          debit: l.debit_cents ? String(l.debit_cents / 100) : "",
          credit: l.credit_cents ? String(l.credit_cents / 100) : "",
          description: l.description || "",
        }))
      );
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Gagal memuat detail jurnal.");
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
  }, [workbench.activeNested?.id, saving, saved, isDetail, tabId, lines, date, description, reference]);

  // Inputs hold whole rupiah; totals below are integer cents (×100).
  const debitTotal = useMemo(
    () => lines.reduce((sum, l) => sum + parseRupiahToCents(l.debit), 0),
    [lines]
  );
  const creditTotal = useMemo(
    () => lines.reduce((sum, l) => sum + parseRupiahToCents(l.credit), 0),
    [lines]
  );
  const difference = debitTotal - creditTotal;
  const balanced = debitTotal > 0 && debitTotal === creditTotal;

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

  // Micro balance gauge ratio
  const maxSide = Math.max(debitTotal, creditTotal, 1);
  const debitRatio = Math.min(100, Math.max(0, (debitTotal / maxSide) * 100));
  const creditRatio = Math.min(100, Math.max(0, (creditTotal / maxSide) * 100));

  const addLine = () => {
    const newLine = seedLine();
    setLines((prev) => [...prev, newLine]);
    return newLine;
  };

  const updateLine = (id: string, field: keyof FormLine, value: string) => {
    setLines((prev) =>
      prev.map((l) => {
        if (l.id !== id) return l;
        const updated = { ...l, [field]: value };
        if (field === "debit" && value) updated.credit = "";
        if (field === "credit" && value) updated.debit = "";
        return updated;
      })
    );
  };

  const removeLine = (id: string) => {
    if (lines.length <= 2) return;
    setLines((prev) => prev.filter((l) => l.id !== id));
  };

  // Grid Keyboard Navigation: Arrow Up/Down & Enter on last cell
  const handleGridKeyDown = (
    e: React.KeyboardEvent<HTMLElement>,
    rowIndex: number,
    colIndex: number
  ) => {
    if (isDetail || saved) return;

    if (e.key === "ArrowUp") {
      if (rowIndex > 0) {
        e.preventDefault();
        const prevInput = tableRef.current?.querySelector<HTMLElement>(
          `[data-row="${rowIndex - 1}"][data-col="${colIndex}"]`
        );
        prevInput?.focus();
      }
    } else if (e.key === "ArrowDown") {
      if (rowIndex < lines.length - 1) {
        e.preventDefault();
        const nextInput = tableRef.current?.querySelector<HTMLElement>(
          `[data-row="${rowIndex + 1}"][data-col="${colIndex}"]`
        );
        nextInput?.focus();
      }
    } else if (e.key === "Enter") {
      if (colIndex === 3) {
        // Last cell in row (Credit input)
        e.preventDefault();
        if (rowIndex === lines.length - 1) {
          addLine();
          setTimeout(() => {
            const nextAcc = tableRef.current?.querySelector<HTMLElement>(
              `[data-row="${rowIndex + 1}"][data-col="0"]`
            );
            nextAcc?.focus();
          }, 50);
        } else {
          const nextAcc = tableRef.current?.querySelector<HTMLElement>(
            `[data-row="${rowIndex + 1}"][data-col="0"]`
          );
          nextAcc?.focus();
        }
      }
    }
  };

  const handleSubmit = async () => {
    if (!balanced) {
      setError("Jurnal harus seimbang (Total Debit = Total Kredit) sebelum dapat di-posting.");
      return;
    }
    setError(null);
    setSaving(true);
    try {
      const payload = {
        entry_date: date,
        description: description.trim(),
        lines: lines
          .filter((l) => l.accountId && (parseRupiahToCents(l.debit) > 0 || parseRupiahToCents(l.credit) > 0))
          .map((l) => ({
            account_id: Number(l.accountId),
            debit_cents: parseRupiahToCents(l.debit),
            credit_cents: parseRupiahToCents(l.credit),
            description: l.description.trim(),
          })),
      };
      const res = await api.createManualJournal(payload);
      setSaved(true);
      setStatus("POSTED");
      if (res?.number) setNumber(res.number);
      try {
        localStorage.removeItem(draftKey);
      } catch (e) {
        console.warn("Failed to clear autosaved draft", e);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mem-posting jurnal umum.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <LoadingState label="Memuat form jurnal..." />;
  if (loadError) return <ErrorState message={loadError} onRetry={loadMasters} />;

  return (
    <div className="enterprise-form">
      {/* Zone 1: Sticky Document Header */}
      <header className="form-zone-1">
        <div className="form-header__title-group">
          <div className="form-header__icon-box">
            <Icon name="book_open" size={20} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="form-header__title">Jurnal Umum Manual (General Journal)</h1>
              <span className="form-header__doc-number">{number}</span>
              <span className={`form-header__status-badge status-${status.toLowerCase()}`}>
                {status}
              </span>
              {draftRestored && (
                <span className="status-badge-inline status-open font-semibold text-xs">
                  ✓ Draft Dipulihkan
                </span>
              )}
            </div>
            <p className="text-xs text-muted mt-0.5">Penyesuaian & ayat jurnal umum PSAK</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => window.print()}
            title="Cetak Bukti Jurnal"
          >
            <Icon name="print" size={16} />
          </button>
          <button
            type="button"
            className="topbar__icon-btn"
            onClick={() => workbench.close(tabId)}
            title="Tutup Form"
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

        {/* Primary Entity & Header Meta */}
        <div className="form-card form-grid-2col">
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
                placeholder="Contoh: ADJ-2026-08/001"
                value={reference}
                disabled={isDetail || saved}
                onChange={(e) => setReference(e.target.value)}
              />
              {duplicateRefWarning && (
                <p className="text-xs text-amber-600 mt-1">{duplicateRefWarning}</p>
              )}
            </div>
          </div>

          <div className="auth-field">
            <label>Keterangan / Memo Jurnal *</label>
            <textarea
              className="input-base"
              style={{ height: "108px", resize: "none", padding: "8px 12px" }}
              placeholder="Contoh: Penyesuaian biaya sewa dibayar di muka bulan Agustus 2026"
              value={description}
              disabled={isDetail || saved}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>

        {/* 2.B Line Items Table Engine */}
        <div className="form-card">
          <div className="flex-between mb-3">
            <h2 className="text-sm font-bold text-primary">Rincian Baris Debit & Kredit</h2>
            {!isDetail && !saved && (
              <button
                type="button"
                className="btn-dash-secondary text-xs"
                onClick={addLine}
              >
                <Icon name="plus" size={14} />
                <span>+ Tambah Baris (Enter)</span>
              </button>
            )}
          </div>

          <div className="datatable-wrapper">
            <table className="datatable" ref={tableRef}>
              <thead>
                <tr>
                  <th style={{ width: "35%" }}>Akun Buku Besar *</th>
                  <th>Keterangan Baris</th>
                  <th className="num" style={{ width: "18%" }}>Debit (Rp)</th>
                  <th className="num" style={{ width: "18%" }}>Kredit (Rp)</th>
                  {!isDetail && !saved && <th style={{ width: "50px" }}>Aksi</th>}
                </tr>
              </thead>
              <tbody>
                {lines.map((line, idx) => (
                  <tr key={line.id}>
                    <td>
                      <select
                        className="input-base text-xs w-full"
                        data-row={idx}
                        data-col={0}
                        value={line.accountId}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "accountId", e.target.value)}
                        onKeyDown={(e) => handleGridKeyDown(e, idx, 0)}
                      >
                        <option value="">-- Pilih Akun ({idx + 1}) --</option>
                        {accounts.map((acc) => (
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
                        data-row={idx}
                        data-col={1}
                        placeholder="Memo baris..."
                        value={line.description}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "description", e.target.value)}
                        onKeyDown={(e) => handleGridKeyDown(e, idx, 1)}
                      />
                    </td>
                    <td className="num">
                      <input
                        type="number"
                        className="input-base text-xs text-right font-mono w-full cell-debit"
                        data-row={idx}
                        data-col={2}
                        placeholder="0"
                        value={line.debit}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "debit", e.target.value)}
                        onKeyDown={(e) => handleGridKeyDown(e, idx, 2)}
                      />
                    </td>
                    <td className="num">
                      <input
                        type="number"
                        className="input-base text-xs text-right font-mono w-full cell-credit"
                        data-row={idx}
                        data-col={3}
                        placeholder="0"
                        value={line.credit}
                        disabled={isDetail || saved}
                        onChange={(e) => updateLine(line.id, "credit", e.target.value)}
                        onKeyDown={(e) => handleGridKeyDown(e, idx, 3)}
                      />
                    </td>
                    {!isDetail && !saved && (
                      <td className="text-center">
                        <button
                          type="button"
                          className="topbar__icon-btn text-danger"
                          disabled={lines.length <= 2}
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
                    Total Jurnal:
                  </td>
                  <td className="num font-mono font-bold text-emerald-700 text-sm cell-debit">
                    {formatIDRFromCents(debitTotal)}
                  </td>
                  <td className="num font-mono font-bold text-slate-700 text-sm cell-credit">
                    {formatIDRFromCents(creditTotal)}
                  </td>
                  {!isDetail && !saved && <td />}
                </tr>
              </tfoot>
            </table>
          </div>
        </div>

        {/* 2.C Official Print Signature Sign-off Box */}
        <div className="print-signoff">
          <div className="print-signoff-box">
            <div className="sign-role">Dibuat Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Staf Akuntansi )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Diperiksa Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Supervisor / Chief Accountant )</div>
          </div>
          <div className="print-signoff-box">
            <div className="sign-role">Disetujui Oleh</div>
            <div className="sign-space" />
            <div className="sign-name">( Direktur / Head of Finance )</div>
          </div>
        </div>
      </main>

      {/* Zone 3: Sticky Summary & Action Footer */}
      <footer className="form-zone-3">
        <div className="flex items-center gap-4">
          <div className="flex flex-col gap-1.5">
            {/* Live Balance Micro Gauge */}
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
              <div
                style={{
                  width: "120px",
                  height: "6px",
                  backgroundColor: "var(--bg-surface-tertiary)",
                  borderRadius: "var(--radius-full)",
                  overflow: "hidden",
                  display: "flex",
                }}
                title={`Proporsi Debit: ${debitRatio.toFixed(0)}% vs Kredit: ${creditRatio.toFixed(0)}%`}
              >
                <div
                  style={{
                    width: `${balanced ? 50 : debitRatio / 2}%`,
                    backgroundColor: balanced ? "var(--color-success)" : "var(--color-warning)",
                    height: "100%",
                  }}
                />
                <div
                  style={{
                    width: `${balanced ? 50 : creditRatio / 2}%`,
                    backgroundColor: balanced ? "var(--color-success)" : "var(--color-danger)",
                    height: "100%",
                  }}
                />
              </div>
              <span className="text-xs text-muted font-mono">{debitRatio === creditRatio && debitTotal > 0 ? "1:1" : "Selisih"}</span>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-secondary">Status Keseimbangan:</span>
              {balanced ? (
                <span className="status-badge status-balanced text-xs">
                  <Icon name="check" size={12} /> SEIMBANG (DEBIT = KREDIT)
                </span>
              ) : (
                <span className="status-badge status-unbalanced text-xs">
                  <Icon name="warning" size={12} /> SELISIH: {formatIDRFromCents(Math.abs(difference))}
                </span>
              )}
            </div>
          </div>
          <div className="text-xs text-muted">
            <span>Shortcut: [Ctrl+S] Simpan &bull; [Esc] Batal</span>
          </div>
        </div>

        <div className="flex items-center gap-6">
          <div className="text-right">
            <div className="text-xs text-secondary">
              Total Debit: <strong className="font-mono text-primary">{formatIDRFromCents(debitTotal)}</strong> &bull; Total Kredit: <strong className="font-mono text-primary">{formatIDRFromCents(creditTotal)}</strong>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {!isDetail && !saved && (
              <button
                type="button"
                className="btn-dash-primary"
                disabled={!balanced || saving}
                onClick={handleSubmit}
              >
                {saving ? (
                  <span>Mem-posting Jurnal...</span>
                ) : (
                  <>
                    <Icon name="check" size={14} />
                    <span>Posting Jurnal Umum</span>
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

