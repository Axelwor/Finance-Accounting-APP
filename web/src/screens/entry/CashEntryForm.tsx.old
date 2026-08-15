import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useToast } from "../../components/Toast";
import { AccountPicker } from "../../components/AccountPicker";
import { StaticCombobox } from "../../components/Combobox";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDR, formatDate } from "../../lib/format";
import type { AccountItem, Category, CounterLinePayload, EntrySubKind } from "../../types";

interface Props {
  tabId: string;
  subKind: EntrySubKind;
  /** Entry id when viewing an existing entry; absent for a draft. */
  entryId?: string | number;
  /** Persisted number when viewing an existing entry. */
  initialTitle?: string;
}

interface CounterLine {
  id: string;
  accountId: string;
  amount: string;
  memo: string;
}

/** localStorage keys for remembered form preferences. */
const LS_MODE = "cashForm.mode"; // "quick" | "detail"
const LS_LAST_CASH = "cashForm.lastCashAccount"; // account id per subKind
const LS_KEEP_HEADER = "cashForm.keepHeader"; // "1" | "0"

type FormMode = "quick" | "detail";

function loadMode(): FormMode {
  try {
    return localStorage.getItem(LS_MODE) === "detail" ? "detail" : "quick";
  } catch {
    return "quick";
  }
}

function lastCashKey(subKind: EntrySubKind): string {
  return `${LS_LAST_CASH}.${subKind}`;
}

function loadLastCash(subKind: EntrySubKind): string {
  try {
    return localStorage.getItem(lastCashKey(subKind)) ?? "";
  } catch {
    return "";
  }
}

function loadKeepHeader(): boolean {
  try {
    return localStorage.getItem(LS_KEEP_HEADER) !== "0";
  } catch {
    return true;
  }
}

let lineSeq = 0;
function seedCounterLine(): CounterLine {
  lineSeq += 1;
  return { id: `l${Date.now()}-${lineSeq}`, accountId: "", amount: "", memo: "" };
}

function parseCents(digits: string): number {
  const clean = digits.replace(/[^\d]/g, "");
  if (!clean) return 0;
  return parseInt(clean, 10);
}

function formatDigits(value: string): string {
  return value.replace(/[^\d]/g, "").slice(0, 15);
}

/**
 * Cash & bank entry form — dual-mode single page.
 *
 * ┌──────────────────────────────────────────────┬──────────────┐
 * │ Header: Tanggal · Counterparty · Kas/Bank ·  │ Action rail  │
 * │         Jumlah · No Bukti · Referensi        │  Simpan      │
 * │                                              │  Simpan&Baru │
 * │ Mode: [Cepat | Rinci]                        │  Tutup       │
 * │  Cepat  → kategori + catatan (1 baris)       │  Balik&Ganti │
 * │  Rinci → grid multi-akun + memo              │  (view only) │
 * │                                              │              │
 * │ Pratinjau jurnal (live, read-only)           │              │
 * └──────────────────────────────────────────────┴──────────────┘
 *
 * Design rules (from the UX improvement plan):
 *  - The cash-side account picker only lists CASH/BANK accounts.
 *  - One source of truth for the amount: Quick mode & single detail line
 *    sync two-way with the header amount; ≥2 lines make the grid total the
 *    master and the header shows Δ with a one-click "Samakan" fix.
 *  - Existing entries open read-only (journals are immutable) with a
 *    "Balik & ganti" (reverse + duplicate) action.
 *  - Save shows the real journal number from the backend response.
 *  - Save & New keeps date/cash account/counterparty/mode when the user
 *    enabled "Pertahankan header" (default on).
 *  - Keyboard: Ctrl/Cmd+S save, Ctrl/Cmd+Enter save & new, Esc close,
 *    Enter in the last grid row adds a new line.
 */
export function CashEntryForm({ tabId, subKind, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  /** Journal number + id returned by the backend after a successful save. */
  const [savedJournal, setSavedJournal] = useState<{ id: number; number: string } | null>(null);

  // ── Header state ────────────────────────────────────────────────────────
  const [date, setDate] = useState(mockHelpers.today());
  const [number, setNumber] = useState(initialTitle ?? draftNumber(subKind));
  const [autoNumber, setAutoNumber] = useState(true);
  const [description, setDescription] = useState("");
  const [counterparty, setCounterparty] = useState("");
  const [reference, setReference] = useState("");
  const [cashAccount, setCashAccount] = useState("");
  const [counterAccount, setCounterAccount] = useState(""); // transfer "To"
  const [amountDisplay, setAmountDisplay] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [note, setNote] = useState("");

  // ── Detail grid (Rinci mode) ────────────────────────────────────────────
  const [counterLines, setCounterLines] = useState<CounterLine[]>([seedCounterLine()]);
  /** When the user edits the header amount manually while the grid total
   *  leads, we flag an override and surface Δ + a "Samakan" fix button. */
  const [amountOverride, setAmountOverride] = useState(false);

  // ── Modes & preferences ─────────────────────────────────────────────────
  const [mode, setMode] = useState<FormMode>(loadMode);
  const [keepHeader, setKeepHeader] = useState(loadKeepHeader);

  // ── View existing entry ─────────────────────────────────────────────────
  const [viewing, setViewing] = useState(false);
  const [viewLoading, setViewLoading] = useState(false);
  const [viewAmountCents, setViewAmountCents] = useState(0);

  const isTransfer = subKind === "cash-transfer";
  const isMoneyIn = subKind === "money-in";
  const readOnly = viewing || savedJournal !== null;

  const cashOptions = useMemo(
    () => accounts.filter((a) => a.account_type === "CASH" || a.account_type === "BANK"),
    [accounts],
  );

  const categoryOptions = useMemo(
    () =>
      categories
        .filter((c) => c.kind === subKind)
        .map((c) => ({ value: c.id, label: c.name })),
    [categories, subKind],
  );

  const accountByID = useMemo(() => {
    const map = new Map<string, AccountItem>();
    for (const a of accounts) map.set(String(a.id), a);
    return map;
  }, [accounts]);

  // ── Data loading (accounts + categories; existing entry when editing) ───
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [acctList, catList] = await Promise.all([
          api.listAccounts(),
          api.listCategories(),
        ]);
        if (cancelled) return;
        setAccounts(acctList);
        setCategories(catList);
        // Default the cash account to the last one used for this subKind.
        const last = loadLastCash(subKind);
        if (last && acctList.some((a) => a.id === last)) setCashAccount(last);
      } catch (err) {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : "Gagal memuat akun.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [subKind]);

  /** Full journal entry with lines when viewing an existing entry. */
  const [journalLines, setJournalLines] = useState<{account_id: string; debit_cents: number; credit_cents: number; account_name: string}[]>([]);

  // Load an existing entry for the read-only view.
  useEffect(() => {
    if (!entryId) return;
    let cancelled = false;
    setViewLoading(true);
    (async () => {
      try {
        // First get list to confirm entry exists and fetch basic fields
        const items = await api.listCashEntries({
          kind: isTransfer ? "transfer" : isMoneyIn ? "money-in" : "money-out",
          q: typeof entryId === "string" ? entryId : String(entryId),
          limit: 5,
        });
        if (cancelled) return;
        const match =
          items.find((it) => String(it.id) === String(entryId) || it.number === String(entryId)) ??
          items[0];
        if (!match) {
          setLoadError("Entri tidak ditemukan.");
          setViewLoading(false);
          return;
        }
        setViewing(true);
        setNumber(match.number);
        setDate(match.entry_date);
        setDescription(match.description);
        setCashAccount(String(match.cash_account_id ?? ""));
        if (match.kind === "transfer" && match.from_account_id) {
          setCashAccount(String(match.from_account_id));
          setCounterAccount(String(match.to_account_id ?? ""));
        }
        setViewAmountCents(match.amount_cents);
        
        // Now fetch the full entry with lines for display
        const journalId = match.id;
        console.log('Fetching journal entry with ID:', journalId, 'Type:', typeof journalId); // DEBUG
        if (!journalId) {
          setLoadError("Entri tidak memiliki ID untuk dimuat.");
          setViewLoading(false);
          return;
        }
        
        const fullEntry = await api.getJournalEntry(journalId);
        console.log('API response:', fullEntry); // DEBUG
        if (cancelled || !fullEntry) {
          const msg = cancelled ? "Cancelled" : "Entry not found in backend";
          console.warn(msg, "| Response:", fullEntry); // DEBUG
          if (!cancelled) setLoadError("Gagal memuat rincian entri (tidak ditemukan).");
          setViewLoading(false);
          return;
        }
        
        if (!fullEntry.lines || fullEntry.lines.length === 0) {
          console.warn("No lines found for this entry"); // DEBUG
          setLoadError("Rincian entri kosong.");
          setViewLoading(false);
          return;
        }
        
        // Transform lines to local format
        const transformedLines = fullEntry.lines.map(l => ({
          account_id: String(l.account_id),
          debit_cents: l.debit_cents ?? 0,
          credit_cents: l.credit_cents ?? 0,
          account_name: l.account_name || `#${l.account_id}`
        }));
        console.log('Transformed lines:', transformedLines); // DEBUG
        setJournalLines(transformedLines);
      } catch (err) {
        if (!cancelled) setLoadError(err instanceof Error ? err.message : "Gagal memuat entri.");
      } finally {
        if (!cancelled) setViewLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [entryId]);

  // ── Unsaved tracking ────────────────────────────────────────────────────
  useEffect(() => {
    if (!readOnly) workbench.markUnsaved(tabId, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tabId, date, number, description, cashAccount, counterAccount, counterLines, amountDisplay]);

  // ── Amount sync (single source of truth) ────────────────────────────────
  const cashAmountCents = useMemo(() => parseCents(amountDisplay), [amountDisplay]);

  const counterTotalCents = useMemo(
    () => counterLines.reduce((sum, line) => sum + parseCents(line.amount), 0),
    [counterLines],
  );

  /** Filled detail lines (an empty trailing row is not "filled"). */
  const filledLineCount = useMemo(
    () => counterLines.filter((l) => l.accountId !== "" || parseCents(l.amount) > 0).length,
    [counterLines],
  );

  // Grid total drives the header when ≥2 filled lines and no manual override.
  useEffect(() => {
    if (isTransfer || mode === "quick") return;
    if (filledLineCount >= 2 && !amountOverride && counterTotalCents > 0) {
      setAmountDisplay(counterTotalCents ? String(counterTotalCents) : "");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [counterTotalCents, filledLineCount, amountOverride]);

  const setHeaderAmount = (digits: string) => {
    setAmountDisplay(digits);
    setAmountOverride(filledLineCount >= 2);
    // Single filled row drives the two-way sync in BOTH modes: quick mode's
    // hidden counter line and detail mode's first row mirror the header.
    if (filledLineCount <= 1 && counterLines.length >= 1) {
      setCounterLines((current) =>
        current.map((line, idx) => (idx === 0 ? { ...line, amount: digits } : line)),
      );
    }
  };

  const setLineAmount = (lineId: string, digits: string) => {
    setCounterLines((current) =>
      current.map((line) => (line.id === lineId ? { ...line, amount: digits } : line)),
    );
    // Single filled row drives the header (two-way sync).
    const othersFilled = counterLines.filter((l) => l.id !== lineId && parseCents(l.amount) > 0);
    if (othersFilled.length === 0) {
      setAmountDisplay(digits);
      setAmountOverride(false);
    }
  };

  const matchAmountToTotal = () => {
    setAmountDisplay(counterTotalCents ? String(counterTotalCents) : "");
    setAmountOverride(false);
  };

  // ── Quick mode: category drives the counter account ─────────────────────
  const applyCategory = (value: string | null) => {
    setCategoryId(value ?? "");
    if (!value) return;
    const cat = categories.find((c) => c.id === value);
    if (!cat) return;
    // money-in: Dr cash / Cr category default credit; money-out: Dr default debit / Cr cash.
    const wanted = isMoneyIn ? cat.default_credit_account_id : cat.default_debit_account_id;
    if (wanted) {
      setCounterLines((current) =>
        current.map((line, idx) => (idx === 0 ? { ...line, accountId: String(wanted) } : line)),
      );
    }
  };

  // ── Journal preview (live) ──────────────────────────────────────────────
  const journalPreview = useMemo(() => {
    const rows: { side: "Dr" | "Cr"; account: string; amount: number }[] = [];
    const cashName =
      accountByID.get(cashAccount)?.name ?? (cashAccount ? `#${cashAccount}` : "— kas/bank —");
    const toName =
      accountByID.get(counterAccount)?.name ?? (counterAccount ? `#${counterAccount}` : "— tujuan —");
    if (isTransfer) {
      if (cashAccount) rows.push({ side: "Dr", account: cashName, amount: cashAmountCents });
      if (counterAccount) rows.push({ side: "Cr", account: toName, amount: cashAmountCents });
      return rows;
    }
    if (mode === "quick") {
      const line = counterLines[0];
      const acct = line?.accountId ? accountByID.get(line.accountId) : undefined;
      const name = acct ? acct.name : "— pilih kategori/akun —";
      if (isMoneyIn) {
        rows.push({ side: "Dr", account: cashName, amount: cashAmountCents });
        rows.push({ side: "Cr", account: name, amount: cashAmountCents });
      } else {
        rows.push({ side: "Dr", account: name, amount: cashAmountCents });
        rows.push({ side: "Cr", account: cashName, amount: cashAmountCents });
      }
      return rows;
    }
    for (const line of counterLines) {
      const acct = line.accountId ? accountByID.get(line.accountId) : undefined;
      const cents = parseCents(line.amount);
      if (!acct || cents <= 0) continue;
      if (isMoneyIn) rows.push({ side: "Cr", account: acct.name, amount: cents });
      else rows.push({ side: "Dr", account: acct.name, amount: cents });
    }
    const cashRow = { side: isMoneyIn ? ("Dr" as const) : ("Cr" as const), account: cashName, amount: cashAmountCents };
    if (cashAmountCents > 0) rows.unshift(cashRow);
    return rows;
  }, [isTransfer, isMoneyIn, mode, cashAccount, counterAccount, cashAmountCents, counterLines, accountByID]);

  const previewDr = journalPreview.filter((r) => r.side === "Dr").reduce((s, r) => s + r.amount, 0);
  const previewCr = journalPreview.filter((r) => r.side === "Cr").reduce((s, r) => s + r.amount, 0);
  const previewBalanced = previewDr > 0 && previewDr === previewCr;

  // ── Line ops ────────────────────────────────────────────────────────────
  const updateCounter = (lineId: string, patch: Partial<CounterLine>) => {
    setCounterLines((current) => current.map((line) => (line.id === lineId ? { ...line, ...patch } : line)));
  };
  const removeCounter = (lineId: string) => {
    setCounterLines((current) => (current.length > 1 ? current.filter((line) => line.id !== lineId) : current));
  };
  const addCounter = () => setCounterLines((current) => [...current, seedCounterLine()]);

  // ── Validation (per-field) ──────────────────────────────────────────────
  const validate = (): true | Record<string, string> => {
    const errs: Record<string, string> = {};
    if (cashAmountCents <= 0) errs.amount = "Isi nominal lebih dari 0.";
    if (isTransfer) {
      if (!cashAccount) errs.cashAccount = "Pilih akun sumber (CASH/BANK).";
      if (!counterAccount) errs.counterAccount = "Pilih akun tujuan (CASH/BANK).";
      if (cashAccount && cashAccount === counterAccount) errs.counterAccount = "Akun tujuan harus berbeda dari sumber.";
    } else {
      if (!cashAccount) errs.cashAccount = "Pilih akun kas/bank.";
      if (mode === "quick") {
        const line = counterLines[0];
        if (!line?.accountId) errs.category = "Pilih kategori (atau isi akun di Mode Rinci).";
      } else {
        for (const line of counterLines) {
          if (parseCents(line.amount) > 0 && !line.accountId) errs[line.id] = "Pilih akun untuk baris ini.";
        }
        if (counterTotalCents <= 0) errs.grid = "Isi minimal satu baris rincian.";
        if (counterTotalCents !== cashAmountCents) {
          errs.grid = `Total rincian ${formatIDR(counterTotalCents)} ≠ jumlah ${formatIDR(cashAmountCents)}.`;
        }
      }
    }
    if (!date) errs.date = "Tanggal wajib diisi.";
    return Object.keys(errs).length > 0 ? errs : true;
  };

  /** Composed description: counterparty + reference + note/description. */
  const composedDescription = useCallback((): string => {
    const parts: string[] = [];
    if (counterparty.trim()) {
      parts.push(isTransfer ? counterparty.trim() : `${counterpartyLabel(isMoneyIn)}: ${counterparty.trim()}`);
    }
    if (description.trim()) parts.push(description.trim());
    if (!isTransfer && mode === "quick" && note.trim()) parts.push(note.trim());
    if (reference.trim()) parts.push(`Ref: ${reference.trim()}`);
    return parts.join(" — ");
  }, [counterparty, description, note, reference, isTransfer, mode, isMoneyIn]);

  // ── Save ────────────────────────────────────────────────────────────────
  const doSave = async (): Promise<{ id: number; number: string } | null> => {
    const validation = validate();
    if (validation !== true) {
      setFieldErrors(validation);
      setError(Object.values(validation)[0]);
      return null;
    }
    setFieldErrors({});
    setError(null);
    setSaving(true);
    try {
      const desc = composedDescription();
      let result: { id: number; number: string };
      if (isTransfer) {
        const r = await api.postTransfer({
          entry_date: date,
          description: desc,
          from_account_id: Number(cashAccount),
          to_account_id: Number(counterAccount),
          amount_cents: cashAmountCents,
        });
        result = { id: r.id, number: r.number };
      } else {
        const lines: CounterLinePayload[] =
          mode === "quick"
            ? [
                {
                  account_id: Number(counterLines[0].accountId),
                  amount_cents: cashAmountCents,
                  description: note.trim(),
                },
              ]
            : counterLines
                .filter((l) => l.accountId && parseCents(l.amount) > 0)
                .map((l) => ({
                  account_id: Number(l.accountId),
                  amount_cents: parseCents(l.amount),
                  description: l.memo.trim(),
                }));
        const method = isMoneyIn ? api.postCashIn : api.postCashOut;
        const r = await method({
          entry_date: date,
          description: desc,
          cash_account_id: Number(cashAccount),
          counter_account_id: 0,
          amount_cents: cashAmountCents,
          counter_lines: lines,
        });
        result = { id: r.id, number: r.number };
      }
      // Remember the cash account for the next entry of this subKind.
      try {
        localStorage.setItem(lastCashKey(subKind), cashAccount);
      } catch {
        /* storage unavailable — skip remembering */
      }
      setSavedJournal(result);
      workbench.markUnsaved(tabId, false);
      toast.success(`Tersimpan — Jurnal ${result.number}`);
      return result;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Gagal menyimpan.";
      setError(mapBackendError(message));
      return null;
    } finally {
      setSaving(false);
    }
  };

  /** Reset for the next entry. keepHeader=true retains date, cash account,
   *  counterparty and mode — the common batch-entry case. */
  const resetForNew = () => {
    const keep = keepHeader;
    setNumber(draftNumber(subKind));
    setAutoNumber(true);
    setDescription("");
    if (!keep) {
      setCounterparty("");
      setDate(mockHelpers.today());
      setCashAccount(loadLastCash(subKind) || "");
    }
    setReference("");
    setAmountDisplay("");
    setAmountOverride(false);
    setCategoryId("");
    setNote("");
    setCounterLines([seedCounterLine()]);
    setSavedJournal(null);
    setError(null);
    setFieldErrors({});
    workbench.markUnsaved(tabId, true);
  };

  const handleSave = async () => {
    await doSave();
  };

  const handleSaveAndNew = async () => {
    const result = await doSave();
    if (result) resetForNew();
  };

  const handleSaveAndClose = async () => {
    const result = await doSave();
    if (result) workbench.close(tabId);
  };

  /** View mode: reverse the journal then open a prefilled draft. */
  const handleReverseAndReplace = async () => {
    if (!savedJournal && !entryId) return;
    setSaving(true);
    setError(null);
    try {
      const items = await api.listCashEntries({
        kind: isTransfer ? "transfer" : isMoneyIn ? "money-in" : "money-out",
        q: String(entryId ?? number),
        limit: 5,
      });
      const match =
        items.find((it) => it.number === String(entryId ?? number)) ?? items[0];
      // CashEntryListItem.id IS the journal entry id (the list is built from
      // journal_entries), so reverseCash can take it directly.
      if (match?.id) {
        await api.reverseCash(match.id);
        toast.success(`Jurnal ${match.number} dibalik. Draft baru disiapkan dari data lama.`);
      } else {
        toast.error("Entri asli tidak ditemukan untuk dibalik.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal membalik jurnal.");
    } finally {
      setSaving(false);
    }
  };

  // ── Keyboard shortcuts ──────────────────────────────────────────────────
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void handleSave();
  };

  useEffect(() => {
    if (readOnly) return;
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (!saving) void handleSave();
      } else if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        if (!saving) void handleSaveAndNew();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [saving, readOnly, date, number, description, cashAccount, counterAccount, counterLines, amountDisplay, mode, categoryId, note, reference, counterparty]);

  const firstFieldRef = useRef<HTMLSelectElement | HTMLInputElement | null>(null);
  useEffect(() => {
    if (!loading && !readOnly) firstFieldRef.current?.focus();
  }, [loading, readOnly]);

  // ── Derived labels ──────────────────────────────────────────────────────
  const titleLabel =
    subKind === "money-in"
      ? "Penerimaan Kas Lainnya"
      : subKind === "money-out"
        ? "Pengeluaran Kas Lainnya"
        : "Transfer Kas/Bank";
  const statusLabel = readOnly ? (savedJournal ? "POSTED" : viewing ? "POSTED" : "DRAFT") : "DRAFT";

  const hasData = isTransfer
    ? amountDisplay.trim() !== "" && cashAccount !== "" && counterAccount !== ""
    : amountDisplay.trim() !== "" || counterLines.some((l) => l.accountId !== "" || l.amount.trim() !== "");

  const cashFieldLabel = isTransfer ? "Dari akun" : isMoneyIn ? "Masuk ke" : "Keluar dari";
  const partyLabel = counterpartyLabel(isMoneyIn);

  // ── Render ──────────────────────────────────────────────────────────────
  if (loading) return <LoadingState label="Memuat akun…" />;
  if (loadError && accounts.length === 0) return <ErrorState message={loadError} onRetry={() => window.location.reload()} />;

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{titleLabel}</span>
          <span className={`entrytab__status ${readOnly ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {statusLabel}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDate(date)}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          {/* Success panel replaces the form after save. */}
          {savedJournal ? (
            <div className="entrytab__saved" role="status">
              <div className="entrytab__saved-title">✓ Tersimpan — Jurnal {savedJournal.number}</div>
              <div className="entrytab__saved-actions">
                <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.activate(tabId)}>
                  Lihat daftar
                </button>
                <button type="button" className="btn btn--secondary btn--sm" onClick={resetForNew}>
                  Entri baru
                </button>
                <button type="button" className="btn btn--secondary btn--sm" onClick={() => workbench.close(tabId)}>
                  Tutup tab
                </button>
              </div>
            </div>
          ) : null}

          {savedJournal ? null : viewLoading ? (
            <LoadingState label="Memuat entri…" />
          ) : (
            <>
              <div className="entrytab__header-grid">
                <div className="entrytab__header-col">
                  <label className="field">
                    <span className="field__label">Tanggal *</span>
                    <input
                      ref={(el) => {
                        if (el) firstFieldRef.current = el;
                      }}
                      className={`input${fieldErrors.date ? " input--error" : ""}`}
                      type="date"
                      value={date}
                      onChange={(e) => setDate(e.target.value)}
                      disabled={readOnly}
                      required
                    />
                    {fieldErrors.date && <span className="field__error" role="alert">{fieldErrors.date}</span>}
                  </label>
                  {!isTransfer && (
                    <label className="field">
                      <span className="field__label">{partyLabel}</span>
                      <input
                        className="input"
                        value={counterparty}
                        onChange={(e) => setCounterparty(e.target.value)}
                        placeholder={isMoneyIn ? "Nama pemberi dana" : "Nama penerima"}
                        disabled={readOnly}
                      />
                    </label>
                  )}
                  <label className="field">
                    <span className="field__label">{cashFieldLabel} *</span>
                    <AccountPicker
                      accounts={accounts}
                      value={cashAccount || null}
                      onChange={(v) => setCashAccount(v ?? "")}
                      allowedTypes={["CASH", "BANK"]}
                      placeholder="Ketik kode / nama kas-bank…"
                      disabled={readOnly}
                    />
                    {fieldErrors.cashAccount && (
                      <span className="field__error" role="alert">{fieldErrors.cashAccount}</span>
                    )}
                  </label>
                  {isTransfer && (
                    <label className="field">
                      <span className="field__label">Ke akun *</span>
                      <AccountPicker
                        accounts={accounts}
                        value={counterAccount || null}
                        onChange={(v) => setCounterAccount(v ?? "")}
                        allowedTypes={["CASH", "BANK"]}
                        excludeIds={cashAccount ? [cashAccount] : []}
                        placeholder="Ketik kode / nama kas-bank tujuan…"
                        disabled={readOnly}
                      />
                      {fieldErrors.counterAccount && (
                        <span className="field__error" role="alert">{fieldErrors.counterAccount}</span>
                      )}
                    </label>
                  )}
                  <label className="field">
                    <span className="field__label">Jumlah *</span>
                    <input
                      className={`amount${fieldErrors.amount ? " input--error" : ""}`}
                      type="text"
                      inputMode="numeric"
                      value={amountDisplay ? Number(amountDisplay).toLocaleString("id-ID") : ""}
                      onChange={(e) => setHeaderAmount(formatDigits(e.target.value))}
                      placeholder="0"
                      aria-label="Jumlah"
                      disabled={readOnly}
                    />
                    {fieldErrors.amount && <span className="field__error" role="alert">{fieldErrors.amount}</span>}
                  </label>
                </div>
                <div className="entrytab__header-col">
                  <label className="field field--inline">
                    <span className="field__label">No Bukti</span>
                    <input
                      type="checkbox"
                      checked={autoNumber}
                      onChange={(e) => setAutoNumber(e.target.checked)}
                      aria-label="Nomor otomatis"
                      disabled={readOnly}
                    />
                  </label>
                  <input
                    className="input"
                    value={number}
                    onChange={(e) => setNumber(e.target.value)}
                    placeholder="Nomor bukti"
                    disabled={readOnly || autoNumber}
                  />
                  <label className="field">
                    <span className="field__label">No. Referensi / Cek</span>
                    <input
                      className="input"
                      value={reference}
                      onChange={(e) => setReference(e.target.value)}
                      placeholder="Opsional — disimpan di keterangan"
                      disabled={readOnly}
                    />
                  </label>
                  <label className="field">
                    <span className="field__label">Keterangan</span>
                    <input
                      className="input"
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      placeholder="Deskripsi singkat"
                      disabled={readOnly}
                    />
                  </label>
                </div>
              </div>

              {/* Mode switch — hidden for transfers (no category/grid needed). */}
              {!isTransfer && !readOnly && (
                <div className="entrytab__mode" role="tablist" aria-label="Mode input">
                  <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "quick"}
                    className={`btn btn--sm ${mode === "quick" ? "btn--primary" : "btn--secondary"}`}
                    onClick={() => {
                      setMode("quick");
                      try {
                        localStorage.setItem(LS_MODE, "quick");
                      } catch { /* ignore */ }
                    }}
                  >
                    Mode Cepat
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "detail"}
                    className={`btn btn--sm ${mode === "detail" ? "btn--primary" : "btn--secondary"}`}
                    onClick={() => {
                      setMode("detail");
                      try {
                        localStorage.setItem(LS_MODE, "detail");
                      } catch { /* ignore */ }
                    }}
                  >
                    Mode Rinci
                  </button>
                  {categories.length === 0 && mode === "quick" && (
                    <span className="entrytab__mode-hint">
                      Belum ada kategori — gunakan Mode Rinci.
                    </span>
                  )}
                </div>
              )}

              {/* Quick mode: category + note. */}
              {!isTransfer && mode === "quick" && !readOnly && (
                <div className="entrytab__quick">
                  <label className="field">
                    <span className="field__label">Kategori *</span>
                    <StaticCombobox
                      options={categoryOptions}
                      value={categoryId || null}
                      onChange={(v) => applyCategory(v)}
                      placeholder="Pilih kategori…"
                    />
                    {fieldErrors.category && (
                      <span className="field__error" role="alert">{fieldErrors.category}</span>
                    )}
                  </label>
                  <label className="field">
                    <span className="field__label">Catatan</span>
                    <input
                      className="input"
                      value={note}
                      onChange={(e) => setNote(e.target.value)}
                      placeholder="Opsional — contoh: listrik Agustus"
                    />
                  </label>
                </div>
              )}

              {/* Detail grid (Rinci mode, and Quick mode's read-only view of the counter account). */}
              {!isTransfer && (mode === "detail" || readOnly) && (
                <div className="entrytab__detail">
                  <div className="entrytab__detail-title">
                    {mode === "detail" && !readOnly ? "Rincian alokasi *" : "Rincian alokasi"}
                  </div>
                  <div className="detail-grid">
                    <div className="detail-grid__head">
                      <div>Akun (ketik untuk cari)</div>
                      <div className="right">Nilai</div>
                      <div>Memo</div>
                      <div aria-hidden="true" />
                    </div>
                    {counterLines.map((line, idx) => (
                      <div className="detail-grid__row" key={line.id}>
                        <div>
                          <AccountPicker
                            accounts={accounts}
                            value={line.accountId || null}
                            onChange={(v) => updateCounter(line.id, { accountId: v ?? "" })}
                            excludeIds={cashAccount ? [cashAccount] : []}
                            disabled={readOnly}
                            placeholder={idx === 0 ? "Ketik kode / nama akun…" : "Ketik kode / nama akun…"}
                          />
                          {fieldErrors[line.id] && (
                            <span className="field__error" role="alert">{fieldErrors[line.id]}</span>
                          )}
                        </div>
                        <div>
                          <input
                            className="amount"
                            type="text"
                            inputMode="numeric"
                            value={line.amount ? Number(line.amount).toLocaleString("id-ID") : ""}
                            onChange={(e) => setLineAmount(line.id, formatDigits(e.target.value))}
                            onKeyDown={(e) => {
                              if (
                                e.key === "Enter" &&
                                idx === counterLines.length - 1 &&
                                !readOnly &&
                                line.accountId !== "" &&
                                parseCents(line.amount) > 0
                              ) {
                                e.preventDefault();
                                addCounter();
                              }
                            }}
                            placeholder="0"
                            aria-label={`Nilai baris ${idx + 1}`}
                            disabled={readOnly}
                          />
                        </div>
                        <div>
                          <input
                            className="input"
                            value={line.memo}
                            onChange={(e) => updateCounter(line.id, { memo: e.target.value })}
                            placeholder="Memo baris"
                            aria-label={`Memo baris ${idx + 1}`}
                            disabled={readOnly}
                          />
                        </div>
                        <div>
                          <button
                            type="button"
                            className="detail-grid__remove"
                            onClick={() => removeCounter(line.id)}
                            aria-label="Hapus baris"
                            disabled={readOnly || counterLines.length === 1}
                          >
                            ×
                          </button>
                        </div>
                      </div>
                    ))}
                    {!readOnly && (
                      <div className="detail-grid__row detail-grid__row--add">
                        <div>
                          <button type="button" className="btn btn--secondary btn--sm" onClick={addCounter}>
                            + Tambah baris
                          </button>
                        </div>
                        <div />
                        <div />
                        <div />
                      </div>
                    )}
                  </div>
                  <div className="entrytab__total">
                    <span className="entrytab__total-label">Total rincian</span>
                    <span className="entrytab__total-value">{formatIDR(counterTotalCents)}</span>
                  </div>
                  {amountOverride && counterTotalCents !== cashAmountCents && (
                    <div className="entrytab__delta" role="alert">
                      ⚠ Selisih {formatIDR(Math.abs(counterTotalCents - cashAmountCents))}{" "}
                      <button type="button" className="btn btn--secondary btn--sm" onClick={matchAmountToTotal}>
                        Samakan ke total rincian
                      </button>
                    </div>
                  )}
                  {fieldErrors.grid && <FormError message={fieldErrors.grid} />}
                </div>
              )}

              {/* Journal preview — live (quick/detail modes) or actual lines (view-only) */}
              <div className="entrytab__preview" aria-label="Pratinjau jurnal">
                <div className="entrytab__preview-title">{viewing ? "Rincian jurnal" : "Pratinjau jurnal"}</div>
                {viewing && journalLines.length > 0 ? (
                  <>
                    <table className="entrytab__preview-table">
                      <tbody>
                        {journalLines.map((line, i) => {
                          const hasDebit = line.debit_cents > 0;
                          return (
                            <tr key={i}>
                              <td className="entrytab__preview-side">{hasDebit ? "Dr" : "Cr"}</td>
                              <td>{line.account_name || `#${line.account_id}`}</td>
                              <td className="right">{formatIDR(hasDebit ? line.debit_cents : line.credit_cents)}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                    <div className={`entrytab__preview-balance ${true ? "is-positive" : "is-negative"}`}>
                      ✓ Balance
                    </div>
                  </>
                ) : journalPreview.length === 0 ? (
                  <div className="entrytab__preview-empty">Isi form untuk melihat pratinjau jurnal.</div>
                ) : (
                  <table className="entrytab__preview-table">
                    <tbody>
                      {journalPreview.map((row, i) => (
                        <tr key={`${row.side}-${i}`}>
                          <td className="entrytab__preview-side">{row.side}</td>
                          <td>{row.account}</td>
                          <td className="right">{formatIDR(row.amount)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>

              {error && <FormError message={error} />}
            </>
          )}
        </div>

        <aside className="action-rail" aria-label="Aksi form">
          {!readOnly && (
            <>
              <button
                type="submit"
                className="action-rail__btn action-rail__btn--primary"
                disabled={saving || !hasData}
                title="Simpan (Ctrl+S)"
              >
                <DiskIcon />
                <span>{saving ? "Menyimpan…" : "Simpan"}</span>
              </button>
              <button
                type="button"
                className="action-rail__btn action-rail__btn--secondary"
                onClick={handleSaveAndNew}
                disabled={saving || !hasData}
                title="Simpan & entri baru (Ctrl+Enter)"
              >
                <SavePlusIcon />
                <span>Simpan &amp; Baru</span>
              </button>
              <button
                type="button"
                className="action-rail__btn action-rail__btn--secondary"
                onClick={handleSaveAndClose}
                disabled={saving || !hasData}
                title="Simpan dan tutup tab"
              >
                <CloseIcon />
                <span>Simpan &amp; Tutup</span>
              </button>
              <label className="action-rail__toggle" title="Pertahankan header saat Simpan & Baru">
                <input
                  type="checkbox"
                  checked={keepHeader}
                  onChange={(e) => {
                    setKeepHeader(e.target.checked);
                    try {
                      localStorage.setItem(LS_KEEP_HEADER, e.target.checked ? "1" : "0");
                    } catch { /* ignore */ }
                  }}
                />
                <span>Pertahankan header</span>
              </label>
            </>
          )}
          {readOnly && (
            <>
              <div className="action-rail__number" title="Nomor jurnal backend">
                {savedJournal ? savedJournal.number : number}
              </div>
              {viewing && (
                <button
                  type="button"
                  className="action-rail__btn action-rail__btn--secondary"
                  onClick={handleReverseAndReplace}
                  disabled={saving}
                  title="Balik jurnal lalu buka draft baru dari data lama"
                >
                  <ReverseIcon />
                  <span>{saving ? "Memproses…" : "Balik & Ganti"}</span>
                </button>
              )}
              <button
                type="button"
                className="action-rail__btn"
                onClick={() => workbench.close(tabId)}
                title="Tutup tab (Esc)"
              >
                <CloseIcon />
                <span>Tutup</span>
              </button>
            </>
          )}
        </aside>
      </div>
    </form>
  );
}

/** Best-effort mapping of backend error strings to friendlier messages. */
function mapBackendError(message: string): string {
  if (message.includes("period") && message.toLowerCase().includes("clos")) {
    return "Periode akuntansi sudah ditutup untuk tanggal ini.";
  }
  if (message.includes("counter_lines")) {
    return "Total rincian tidak sama dengan jumlah. Samakan kedua nilai lalu simpan.";
  }
  return message;
}

function counterpartyLabel(isMoneyIn: boolean): string {
  return isMoneyIn ? "Diterima dari" : "Dibayar kepada";
}

function draftNumber(subKind: EntrySubKind): string {
  switch (subKind) {
    case "money-in": return "BM-DRAFT";
    case "money-out": return "BK-DRAFT";
    case "cash-transfer": return "BT-DRAFT";
    default: return "DRAFT";
  }
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="currentColor" />
      <path d="M12 7v5l3 2" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function SavePlusIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <rect x="3" y="4" width="18" height="16" rx="2" fill="currentColor" />
      <path d="M12 9v6m-3-3h6" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <rect x="3" y="4" width="18" height="16" rx="2" fill="currentColor" />
      <path d="M9 9l6 6m0-6l-6 6" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function ReverseIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 8-8" stroke="currentColor" strokeWidth="1.8" fill="none" strokeLinecap="round" />
      <path d="M4 5v4h4" stroke="currentColor" strokeWidth="1.8" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M20 12a8 8 0 0 1-8 8" stroke="currentColor" strokeWidth="1.8" fill="none" strokeLinecap="round" />
      <path d="M20 19v-4h-4" stroke="currentColor" strokeWidth="1.8" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
