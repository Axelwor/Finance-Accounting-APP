import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDR } from "../../lib/format";
import type { AccountItem, CounterLinePayload, EntrySubKind } from "../../types";

interface Props {
  tabId: string;
  subKind: EntrySubKind;
  /** Entry id when editing an existing entry; absent for a draft. */
  entryId?: string | number;
  /** Persisted number when editing an existing entry. */
  initialTitle?: string;
}

interface CounterLine {
  id: string;
  accountId: string;
  amount: string;
}

/**
 * Cash & bank entry form following the Accurate Online layout:
 *
 *   ┌──────────────────────────────────────────────┬──────────────┐
 *   │ Header (2 columns: cash/date/desc · auto#)   │              │
 *   │                                              │              │
 *   │ Description / Keterangan                     │ Action rail  │
 *   │                                              │   Save       │
 *   │ Cari/Pilih Akun Perkiraan...                 │   Save & New │
 *   │                                              │   Document   │
 *   │ Rincian [Type] *                             │   Attach     │
 *   │   Akun │ Nama Akun │ Nilai                   │   More       │
 *   │   ···  ···        ···                        │              │
 *   │                                              │              │
 *   │ Total: 0                          Nilai 0    │              │
 *   └──────────────────────────────────────────────┴──────────────┘
 *
 * The "cash side" of the journal is taken from the header (Cash/Bank, or
 * From for transfers); the multi-line grid below carries the counter
 * side. For money-in/out the counter is 1+ rows; the running cash
 * amount equals the sum of counter amounts. For transfers the grid is
 * not used — the From + To + Amount come straight from the header.
 */
export function CashEntryForm({ tabId, subKind, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const [date, setDate] = useState(mockHelpers.today());
  const [number, setNumber] = useState(initialTitle ?? draftNumber(subKind));
  const [autoNumber, setAutoNumber] = useState(true);
  const [description, setDescription] = useState("");
  const [cashAccount, setCashAccount] = useState("");
  const [counterAccount, setCounterAccount] = useState("");
  const [counterLines, setCounterLines] = useState<CounterLine[]>([seedCounterLine()]);
  const [transferAmount, setTransferAmount] = useState("");
  const [accountSearch, setAccountSearch] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listAccounts();
        if (cancelled) return;
        setAccounts(list);
      } catch (err) {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : "Failed to load accounts.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, description, cashAccount, counterAccount, counterLines, transferAmount, workbench]);

  const accountByID = useMemo(() => {
    const map = new Map<string, AccountItem>();
    for (const a of accounts) map.set(String(a.id), a);
    return map;
  }, [accounts]);

  const isTransfer = subKind === "cash-transfer";

  const cashAmountCents = useMemo(() => {
    if (isTransfer) return parseCents(transferAmount);
    return counterLines.reduce((sum, line) => sum + parseCents(line.amount), 0);
  }, [isTransfer, transferAmount, counterLines]);

  const counterTotalCents = useMemo(
    () => counterLines.reduce((sum, line) => sum + parseCents(line.amount), 0),
    [counterLines],
  );

  const updateCounter = (lineId: string, patch: Partial<CounterLine>) => {
    setCounterLines((current) => current.map((line) => (line.id === lineId ? { ...line, ...patch } : line)));
  };
  const removeCounter = (lineId: string) => {
    setCounterLines((current) => (current.length > 1 ? current.filter((line) => line.id !== lineId) : current));
  };
  const addCounter = () => setCounterLines((current) => [...current, seedCounterLine()]);

  const validate = (): string | null => {
    if (cashAmountCents <= 0) return "Enter a positive amount.";
    if (!isTransfer && counterTotalCents !== cashAmountCents) {
      return "Sum of detail lines must equal the cash amount.";
    }
    if (isTransfer) {
      if (!cashAccount || !counterAccount) return "Pick two accounts to transfer between.";
      if (cashAccount === counterAccount) return "From and To accounts must differ.";
    } else {
      if (!cashAccount) return "Pick a cash/bank account.";
      if (counterLines.length === 0) return "Add at least one detail line.";
      for (const line of counterLines) {
        if (!line.accountId) return "Every detail line needs an account.";
      }
    }
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
      if (isTransfer) {
        await api.postTransfer({
          entry_date: date,
          description,
          from_account_id: Number(cashAccount),
          to_account_id: Number(counterAccount),
          amount_cents: cashAmountCents,
        });
      } else {
        const counter_lines: CounterLinePayload[] = counterLines.map((line) => ({
          account_id: Number(line.accountId),
          amount_cents: parseCents(line.amount),
          description: "",
        }));
        const method = subKind === "money-in" ? api.postCashIn : api.postCashOut;
        await method({
          entry_date: date,
          description,
          cash_account_id: Number(cashAccount),
          counter_account_id: 0, // ignored when counter_lines is non-empty
          amount_cents: cashAmountCents,
          counter_lines,
        });
      }
      workbench.replaceDraft(tabId, number, "POSTED");
      setSaved(true);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post the entry.");
    } finally {
      setSaving(false);
    }
  };

  const handleSaveAndNew = async () => {
    await handleSubmit(new Event("submit") as unknown as React.FormEvent);
    if (!error) {
      // reset the form for the next entry
      setCounterLines([seedCounterLine()]);
      setTransferAmount("");
      setNumber(initialTitle ?? draftNumber(subKind));
      setDescription("");
      setSaved(false);
    }
  };

  if (loading) return <LoadingState label="Loading masters..." />;
  if (loadError) return <ErrorState message={loadError} onRetry={() => window.location.reload()} />;

  const cashAccountName = accountByID.get(cashAccount)?.name ?? "Cash/Bank";
  const counterAccountName = accountByID.get(counterAccount)?.name ?? "Counter account";
  const titleLabel = subKind === "money-in" ? "Other Receipt" : subKind === "money-out" ? "Other Payment" : "Bank Transfer";
  const detailLabel = isTransfer ? "Transfer" : `${titleLabel} detail`;
  const status = saved ? "POSTED" : "DRAFT";

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{titleLabel}</span>
          <span className={`entrytab__status ${saved ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {status}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDateID(date)}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          <div className="entrytab__header-grid">
            <div className="entrytab__header-col">
              <label className="field">
                <span className="field__label">{isTransfer ? "From account" : "Cash / Bank"}</span>
                <select className="input" value={cashAccount} onChange={(e) => setCashAccount(e.target.value)}>
                  <option value="">Choose account...</option>
                  {accounts.map((a) => (
                    <option key={a.id} value={String(a.id)}>{a.name}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field field--inline">
                <span className="field__label">No Bukti</span>
                <input
                  type="checkbox"
                  checked={autoNumber}
                  onChange={(e) => setAutoNumber(e.target.checked)}
                  aria-label="Auto-generate document number"
                />
              </label>
              <input
                className="input"
                value={number}
                onChange={(e) => setNumber(e.target.value)}
                placeholder="Document number"
              />
              <button type="button" className="btn btn--secondary btn--sm entrytab__ambil">
                <span aria-hidden="true">↗</span> Ambil
              </button>
            </div>
          </div>

          {isTransfer ? (
            <label className="field">
              <span className="field__label">To account</span>
              <select className="input" value={counterAccount} onChange={(e) => setCounterAccount(e.target.value)}>
                <option value="">Choose account...</option>
                {accounts.filter((a) => String(a.id) !== cashAccount).map((a) => (
                  <option key={a.id} value={String(a.id)}>{a.name}</option>
                ))}
              </select>
            </label>
          ) : null}

          <label className="field">
            <span className="field__label">Keterangan / Description</span>
            <input
              className="input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Short description"
            />
          </label>

          {!isTransfer ? (
            <>
              <div className="entrytab__search">
                <input
                  type="search"
                  className="input"
                  placeholder="Cari/Pilih Akun Perkiraan..."
                  value={accountSearch}
                  onChange={(e) => setAccountSearch(e.target.value)}
                />
                <span className="entrytab__search-icon" aria-hidden="true">🔍</span>
              </div>

              <div className="entrytab__detail">
                <div className="entrytab__detail-title">{detailLabel} *</div>
                <div className="detail-grid">
                  <div className="detail-grid__head">
                    <div>Akun</div>
                    <div>Nama Akun</div>
                    <div className="right">Nilai</div>
                    <div aria-hidden="true" />
                  </div>
                  {counterLines.map((line) => {
                    const acct = accountByID.get(line.accountId);
                    return (
                      <div className="detail-grid__row" key={line.id}>
                        <div>
                          <select
                            value={line.accountId}
                            onChange={(e) => updateCounter(line.id, { accountId: e.target.value })}
                          >
                            <option value="">Choose account...</option>
                            {accounts
                              .filter((a) => String(a.id) !== cashAccount)
                              .filter((a) => !accountSearch || a.name.toLowerCase().includes(accountSearch.toLowerCase()))
                              .map((a) => (
                                <option key={a.id} value={String(a.id)}>{a.name}</option>
                              ))}
                          </select>
                        </div>
                        <div>
                          <input
                            type="text"
                            value={acct?.name ?? ""}
                            readOnly
                            aria-readonly="true"
                            className="detail-grid__readonly"
                            placeholder="—"
                          />
                        </div>
                        <div>
                          <input
                            className="amount"
                            type="text"
                            inputMode="numeric"
                            value={formatAmountInput(line.amount)}
                            onChange={(e) => {
                              const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                              updateCounter(line.id, { amount: digits });
                            }}
                            placeholder="0"
                          />
                        </div>
                        <div>
                          <button
                            type="button"
                            className="detail-grid__remove"
                            onClick={() => removeCounter(line.id)}
                            aria-label="Remove line"
                            disabled={counterLines.length === 1}
                          >
                            ×
                          </button>
                        </div>
                      </div>
                    );
                  })}
                  <div className="detail-grid__row detail-grid__row--add">
                    <div>
                      <button type="button" className="btn btn--secondary btn--sm" onClick={addCounter}>
                        + Add line
                      </button>
                    </div>
                    <div />
                    <div />
                    <div />
                  </div>
                </div>
              </div>
            </>
          ) : null}

          <div className="entrytab__total">
            <span className="entrytab__total-label">Nilai</span>
            <span className="entrytab__total-value">{formatIDR(cashAmountCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button
            type="submit"
            className="action-rail__btn action-rail__btn--primary"
            disabled={saving || saved}
            title="Save and close"
          >
            <DiskIcon />
            <span>{saving ? "Saving..." : saved ? "Saved" : "Save"}</span>
          </button>
          <button
            type="button"
            className="action-rail__btn action-rail__btn--secondary"
            onClick={handleSaveAndNew}
            disabled={saving}
            title="Save and start a new entry"
          >
            <SavePlusIcon />
            <span>Save &amp; New</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="Duplicate this entry">
            <DocIcon />
            <span>Document</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="Attach a file">
            <AttachIcon />
            <span>Attach</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="More actions">
            <MoreIcon />
            <span>More</span>
          </button>
          <div className="action-rail__hint">
            <strong>Cash:</strong> {cashAccountName}
            {isTransfer ? ` → ${counterAccountName}` : ""}
          </div>
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function formatAmountInput(raw: string): string {
  if (!raw) return "";
  const digits = raw.replace(/[^\d]/g, "");
  if (!digits) return "";
  return new Intl.NumberFormat("en-US").format(parseInt(digits, 10));
}

function formatDateID(iso: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function seedCounterLine(): CounterLine {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: "",
    amount: "",
  };
}

function draftNumber(subKind: EntrySubKind): string {
  switch (subKind) {
    case "money-in": return "OR-DRAFT";
    case "money-out": return "OP-DRAFT";
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

function DocIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M6 3h9l4 4v14a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z" fill="currentColor" />
      <path d="M14 3v5h5" fill="rgba(255,255,255,0.5)" />
    </svg>
  );
}

function AttachIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M21 11l-9 9a5 5 0 0 1-7-7l9-9a3 3 0 0 1 4 4l-9 9a1 1 0 0 1-1.4-1.4l8-8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function MoreIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="5" cy="12" r="1.5" fill="currentColor" />
      <circle cx="12" cy="12" r="1.5" fill="currentColor" />
      <circle cx="19" cy="12" r="1.5" fill="currentColor" />
    </svg>
  );
}
