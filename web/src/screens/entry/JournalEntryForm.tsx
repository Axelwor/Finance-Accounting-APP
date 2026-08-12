import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDR } from "../../lib/format";
import type { BackendAccount, JournalEntry, JournalEntryLine } from "../../types";
import { AttachmentPanel } from "../../components/AttachmentPanel";

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

/**
 * Manual Journal Entry form (Accountant Mode v1).
 *
 * Multi-line, double-entry: each line picks an account and either a debit
 * or a credit. The entry must balance (total debit = total credit) before
 * it can be posted. When opened with an entryId the form loads the
 * existing entry as read-only detail (manual journals are immutable once
 * posted — corrections are made via reversal).
 */
export function JournalEntryForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isDetail = entryId !== undefined;
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  // Tracks the posted journal entry id so the Post button cannot fire twice.
  const [journalId, setJournalId] = useState<number | null>(typeof entryId === "number" ? entryId : null);

  const [date, setDate] = useState(mockHelpers.today());
  const [description, setDescription] = useState("");
  const [lines, setLines] = useState<FormLine[]>([seedLine(), seedLine()]);
  const [number, setNumber] = useState(initialTitle ?? "JE-DRAFT");
  const [status, setStatus] = useState(isDetail ? "" : "DRAFT");
  const [detail, setDetail] = useState<JournalEntry | null>(null);

  useEffect(() => {
    void loadMasters();
    if (isDetail && entryId) void loadDetail(Number(entryId));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const accountById = useMemo(() => new Map(accounts.map((a) => [a.id, a])), [accounts]);

  const loadMasters = async () => {
    setLoading(true);
    try {
      const data = await api.listBackendAccounts();
      setAccounts(data.filter((a) => a.is_active && !a.is_group));
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load accounts.");
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: number) => {
    try {
      const entry = await api.getJournalEntry(id);
      if (!entry) {
        setLoadError("Journal entry not found.");
        return;
      }
      setDetail(entry);
      setNumber(entry.number);
      setStatus(entry.status);
      setDate(entry.entry_date);
      setDescription(entry.description);
      setLines(entry.lines.map(lineFromBackend));
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load journal entry.");
    }
  };

  const debitTotal = useMemo(
    () => lines.reduce((sum, l) => sum + parseCents(l.debit), 0),
    [lines],
  );
  const creditTotal = useMemo(
    () => lines.reduce((sum, l) => sum + parseCents(l.credit), 0),
    [lines],
  );
  const balanced = debitTotal > 0 && debitTotal === creditTotal;

  // Once posted (this session or opened as an existing entry) the journal is
  // immutable — this also removes any chance of posting the same entry twice.
  const posted = journalId !== null;
  const readOnly = isDetail || posted;

  const updateLine = (lineId: string, patch: Partial<FormLine>) => {
    setLines((current) => current.map((l) => (l.id === lineId ? { ...l, ...patch } : l)));
  };
  const removeLine = (lineId: string) => {
    setLines((current) => (current.length > 2 ? current.filter((l) => l.id !== lineId) : current));
  };
  const addLine = () => setLines((current) => [...current, seedLine()]);

  const validate = (): string | null => {
    if (!date) return "Entry date is required.";
    if (!description.trim()) return "Description is required.";
    if (lines.length < 2) return "At least two lines are required.";
    for (const line of lines) {
      if (!line.accountId) return "Every line needs an account.";
      const d = parseCents(line.debit);
      const c = parseCents(line.credit);
      if (d < 0 || c < 0) return "Amounts must be non-negative.";
      if (d > 0 && c > 0) return "A line cannot have both debit and credit.";
      if (d === 0 && c === 0) return "A line cannot be zero.";
    }
    if (!balanced) return "Total debit must equal total credit.";
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const validation = validate();
    if (validation) {
      setError(validation);
      return;
    }
    setError(null);
    setSaving(true);
    try {
      const payload = {
        entry_date: date,
        description: description.trim(),
        lines: lines.map((l) => ({
          account_id: Number(l.accountId),
          debit_cents: parseCents(l.debit),
          credit_cents: parseCents(l.credit),
          description: l.description.trim() || undefined,
        })),
      };
      const result = await api.createManualJournal(payload);
      setNumber(result.number);
      setStatus(result.status);
      setJournalId(result.id);
      setSaved(true);
      workbench.replaceDraft(tabId, result.number, result.status, result.id);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post the journal entry.");
    } finally {
      setSaving(false);
    }
  };

  // Open a fresh draft tab to start another journal (the posted one stays open
  // and read-only behind it).
  const handleNewJournal = () => {
    workbench.openEntryDraft("journal-entry");
  };

  if (loading) return <LoadingState label="Loading masters..." />;
  if (loadError) return <ErrorState message={loadError} onRetry={() => window.location.reload()} />;

  return (
    <form className="entrytab entrytab--journal" onSubmit={handleSubmit}>
      <div className="entrytab__head">
        <div className="entrytab__head-title">
          <span className={`entrytab__status ${readOnly ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {posted || isDetail ? status || "POSTED" : status || "DRAFT"}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDateID(date)}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          <div className="entrytab__header-grid">
            <label className="field">
              <span className="field__label">Entry date</span>
              <input
                type="date"
                className="input"
                value={date}
                disabled={readOnly}
                onChange={(e) => setDate(e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">Description</span>
              <input
                type="text"
                className="input"
                placeholder="Memo / description"
                value={description}
                disabled={readOnly}
                onChange={(e) => setDescription(e.target.value)}
              />
            </label>
          </div>

          <div className="entrytab__section">
            <div className="entrytab__section-head">
              <span>Lines</span>
              {!readOnly ? (
                <button type="button" className="btn btn--secondary btn--sm" onClick={addLine}>
                  + Add line
                </button>
              ) : null}
            </div>
            <div className="ledger-table ledger-table--entry">
              <div className="ledger-table__head">
                <span>Account</span>
                <span>Description</span>
                <span className="right">Debit</span>
                <span className="right">Credit</span>
                {!readOnly ? <span /> : null}
              </div>
              {lines.map((line) => (
                <LineRow
                  key={line.id}
                  line={line}
                  accounts={accounts}
                  accountById={accountById}
                  disabled={readOnly}
                  onChange={(patch) => updateLine(line.id, patch)}
                  onRemove={() => removeLine(line.id)}
                />
              ))}
            </div>
          </div>

          <div className="entrytab__totals">
            <span />
            <span />
            <span className="right">
              <small>Debit</small>
              <strong>{formatIDR(debitTotal)}</strong>
            </span>
            <span className="right">
              <small>Credit</small>
              <strong>{formatIDR(creditTotal)}</strong>
            </span>
            {!readOnly ? <span /> : null}
          </div>
          <div className="entrytab__balance">
            {balanced ? (
              <span className="kind-mark is-positive">Balanced</span>
            ) : (
              <span className="kind-mark is-negative">
                Out of balance · {formatIDR(Math.abs(debitTotal - creditTotal))}
              </span>
            )}
          </div>

          {isDetail && detail ? (
            <AttachmentPanel ownerType="journal_entry" ownerId={detail.id} />
          ) : null}
        </div>

        <aside className="entrytab__aside action-rail">
          {!isDetail ? (
            <>
              {posted && (
                <div className="action-rail__hint">
                  <span className="kind-mark is-positive">Posted</span>
                  <p>
                    <small>Journal {number} has been posted and is locked.</small>
                  </p>
                </div>
              )}
              <button
                type="submit"
                className="btn btn--primary btn--full"
                disabled={saving || !balanced || posted}
              >
                {saving ? "Posting..." : posted ? "Posted" : "Post Journal"}
              </button>
              {posted ? (
                <button type="button" className="btn btn--secondary btn--full" onClick={handleNewJournal}>
                  New Journal
                </button>
              ) : (
                <button type="button" className="btn btn--secondary btn--full" disabled>
                  Save & New
                </button>
              )}
            </>
          ) : (
            <div className="action-rail__hint">
              <strong>Detail view</strong>
              <p>
                Manual journals are immutable once posted. Corrections are made by posting a
                reversing entry.
              </p>
              {detail ? (
                <p>
                  <small>
                    Intent: {detail.intent_type}
                    <br />
                    Source: {detail.source_ref || "—"}
                  </small>
                </p>
              ) : null}
            </div>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function LineRow({
  line,
  accounts,
  accountById,
  disabled,
  onChange,
  onRemove,
}: {
  line: FormLine;
  accounts: BackendAccount[];
  accountById: Map<number, BackendAccount>;
  disabled: boolean;
  onChange: (patch: Partial<FormLine>) => void;
  onRemove: () => void;
}) {
  const selected = line.accountId ? accountById.get(Number(line.accountId)) : undefined;
  return (
    <div className="ledger-table__row ledger-table__row--entry">
      <span className="ledger-table__cat">
        <select
          className="input"
          value={line.accountId}
          disabled={disabled}
          onChange={(e) => onChange({ accountId: e.target.value })}
        >
          <option value="">Choose account...</option>
          {accounts.map((a) => (
            <option key={a.id} value={String(a.id)}>
              {a.code} · {a.name}
            </option>
          ))}
        </select>
        {selected ? <small>{selected.account_type}</small> : null}
      </span>
      <span className="ledger-table__memo">
        <input
          type="text"
          className="input"
          placeholder="Line memo"
          value={line.description}
          disabled={disabled}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </span>
      <span className="ledger-table__amount right">
        <input
          type="text"
          inputMode="numeric"
          className="input input--amount"
          placeholder="0"
          value={line.debit}
          disabled={disabled}
          onChange={(e) => onChange({ debit: e.target.value, credit: e.target.value ? "" : line.credit })}
        />
      </span>
      <span className="ledger-table__amount right">
        <input
          type="text"
          inputMode="numeric"
          className="input input--amount"
          placeholder="0"
          value={line.credit}
          disabled={disabled}
          onChange={(e) => onChange({ credit: e.target.value, debit: e.target.value ? "" : line.debit })}
        />
      </span>
      {!disabled ? (
        <span>
          <button type="button" className="btn btn--icon btn--sm" onClick={onRemove} aria-label="Remove line">
            ×
          </button>
        </span>
      ) : null}
    </div>
  );
}

function seedLine(): FormLine {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: "",
    debit: "",
    credit: "",
    description: "",
  };
}

function lineFromBackend(l: JournalEntryLine): FormLine {
  return {
    id: `ln-${l.account_id}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: String(l.account_id),
    debit: l.debit_cents ? String(l.debit_cents) : "",
    credit: l.credit_cents ? String(l.credit_cents) : "",
    description: l.description ?? "",
  };
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function formatDateID(iso: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}
