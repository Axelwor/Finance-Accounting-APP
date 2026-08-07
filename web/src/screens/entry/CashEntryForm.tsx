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
  description: string;
  amount: string;
}

/**
 * Cash & bank entry form.
 *
 * Header carries the cash account (the single side that is always one
 * account) and bookkeeping metadata. The lines grid below renders the
 * counter side of the journal: one or more rows, each with its own
 * account, description, and amount. The sum of all counter amounts must
 * equal the cash amount; the grid shows a running diff.
 *
 * Transfers are special: the header has From and To accounts, and the
 * grid renders two locked rows (one for each) that share a single amount.
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
  const [memo, setMemo] = useState("");
  const [payee, setPayee] = useState("");
  const [cashAccount, setCashAccount] = useState("");
  const [counterAccount, setCounterAccount] = useState("");
  const [counterLines, setCounterLines] = useState<CounterLine[]>([seedCounterLine()]);
  const [transferAmount, setTransferAmount] = useState("");

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
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, memo, payee, cashAccount, counterAccount, counterLines, transferAmount, workbench]);

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

  const countersTotals = useMemo(() => {
    let total = 0;
    for (const line of counterLines) total += parseCents(line.amount);
    return total;
  }, [counterLines]);

  const diff = cashAmountCents - countersTotals;

  const updateCounter = (lineId: string, patch: Partial<CounterLine>) => {
    setCounterLines((current) => current.map((line) => (line.id === lineId ? { ...line, ...patch } : line)));
  };
  const removeCounter = (lineId: string) => {
    setCounterLines((current) => (current.length > 1 ? current.filter((line) => line.id !== lineId) : current));
  };
  const addCounter = () => {
    setCounterLines((current) => [...current, seedCounterLine()]);
  };

  const validate = (): string | null => {
    if (cashAmountCents <= 0) {
      return "Enter a positive amount.";
    }
    if (diff !== 0) {
      return "Debits and credits must balance before posting.";
    }
    if (isTransfer) {
      if (!cashAccount || !counterAccount || cashAccount === counterAccount) {
        return "Pick two different accounts to transfer between.";
      }
    } else {
      if (!cashAccount) return "Pick a cash/bank account.";
      if (counterLines.length === 0) return "Add at least one counter line.";
      for (const line of counterLines) {
        if (!line.accountId) return "Every counter line needs an account.";
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
          description: memo,
          from_account_id: Number(cashAccount),
          to_account_id: Number(counterAccount),
          amount_cents: cashAmountCents,
        });
      } else {
        const counter_lines: CounterLinePayload[] = counterLines.map((line) => ({
          account_id: Number(line.accountId),
          amount_cents: parseCents(line.amount),
          description: line.description,
        }));
        const method = subKind === "money-in" ? api.postCashIn : api.postCashOut;
        await method({
          entry_date: date,
          description: memo,
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

  const titleLabel =
    subKind === "money-in" ? "Other Receipt" : subKind === "money-out" ? "Other Payment" : "Bank Transfer";
  const status = saved ? "POSTED" : "DRAFT";

  if (loading) return <LoadingState label="Loading masters..." />;
  if (loadError) return <ErrorState message={loadError} onRetry={() => window.location.reload()} />;

  const accountOptions = accounts.map((a) => ({ value: String(a.id), label: a.name }));
  const cashAccountName = accountByID.get(cashAccount)?.name ?? "Cash/Bank";
  const counterAccountName = accountByID.get(counterAccount)?.name ?? "Counter account";

  return (
    <form className="entrytab" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{titleLabel}</span>
          <span className={`entrytab__status ${saved ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {status}
          </span>
          <span className="entrytab__number">{number}</span>
        </div>
        <div className="entrytab__actions">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => workbench.close(tabId)}>
            Close
          </button>
          <button type="submit" className="btn btn--ink btn--sm" disabled={saving || saved}>
            {saving ? "Posting..." : saved ? "Posted ✓" : "Post"}
          </button>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__section">
          <div className="entrytab__section-title">Header</div>

          <label className="field">
            <span className="field__label">Date</span>
            <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
          </label>

          <label className="field">
            <span className="field__label">Number</span>
            <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
          </label>

          <label className="field">
            <span className="field__label">{isTransfer ? "From account" : "Cash / Bank account"}</span>
            <select className="input" value={cashAccount} onChange={(e) => setCashAccount(e.target.value)}>
              <option value="">Select...</option>
              {accountOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </label>

          {isTransfer ? (
            <label className="field">
              <span className="field__label">To account</span>
              <select className="input" value={counterAccount} onChange={(e) => setCounterAccount(e.target.value)}>
                <option value="">Select...</option>
                {accountOptions
                  .filter((opt) => opt.value !== cashAccount)
                  .map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
              </select>
            </label>
          ) : null}

          <label className="field">
            <span className="field__label">{isTransfer ? "Memo" : "Payee / Payer"}</span>
            <input
              className="input"
              value={payee}
              onChange={(e) => setPayee(e.target.value)}
              placeholder={isTransfer ? "Reference" : "Name"}
            />
          </label>

          <label className="field">
            <span className="field__label">Memo</span>
            <input className="input" value={memo} onChange={(e) => setMemo(e.target.value)} placeholder="Short description" />
          </label>
        </div>

        <div className="entrytab__section">
          <div className="entrytab__section-title">{isTransfer ? "Transfer lines" : "Account lines"}</div>

          <div className="entry-grid">
            <div className="entry-grid__head">
              <div>#</div>
              <div>Account</div>
              <div>Description</div>
              <div className="right">Debit</div>
              <div className="right">Credit</div>
              <div aria-hidden="true" />
            </div>

            {/* Cash / From row (locked debit) */}
            <div className="entry-grid__row entry-grid__row--locked">
              <div className="entry-grid__num">1</div>
              <div>
                <input
                  type="text"
                  value={cashAccountName}
                  readOnly
                  aria-readonly="true"
                  className="entry-grid__readonly"
                  title="Locked from Header"
                />
              </div>
              <div>
                <input
                  type="text"
                  value={memo || (isTransfer ? "Source" : "Cash / Bank")}
                  onChange={(e) => setMemo(e.target.value)}
                  placeholder="Line memo"
                />
              </div>
              <div>
                <input
                  className="amount"
                  type="text"
                  inputMode="numeric"
                  value={formatAmountInput(String(cashAmountCents))}
                  readOnly
                  aria-readonly="true"
                />
              </div>
              <div>
                <input
                  className="amount"
                  type="text"
                  inputMode="numeric"
                  value=""
                  readOnly
                  aria-readonly="true"
                  placeholder="—"
                />
              </div>
              <div aria-hidden="true" />
            </div>

            {isTransfer ? (
              /* Transfer: single locked credit row that mirrors the cash amount. */
              <div className="entry-grid__row entry-grid__row--locked">
                <div className="entry-grid__num">2</div>
                <div>
                  <input
                    type="text"
                    value={counterAccountName}
                    readOnly
                    aria-readonly="true"
                    className="entry-grid__readonly"
                    title="Locked from Header"
                  />
                </div>
                <div>
                  <input
                    type="text"
                    value={memo || "Destination"}
                    onChange={(e) => setMemo(e.target.value)}
                    placeholder="Line memo"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value=""
                    readOnly
                    aria-readonly="true"
                    placeholder="—"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={formatAmountInput(String(cashAmountCents))}
                    readOnly
                    aria-readonly="true"
                  />
                </div>
                <div aria-hidden="true" />
              </div>
            ) : (
              /* Counter lines: one or more rows whose sum equals the cash amount. */
              counterLines.map((line, idx) => (
                <div className="entry-grid__row" key={line.id}>
                  <div className="entry-grid__num">{idx + 2}</div>
                  <div>
                    <select
                      value={line.accountId}
                      onChange={(e) => updateCounter(line.id, { accountId: e.target.value })}
                    >
                      <option value="">Select account...</option>
                      {accountOptions
                        .filter((opt) => opt.value !== cashAccount)
                        .map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                    </select>
                  </div>
                  <div>
                    <input
                      type="text"
                      value={line.description}
                      onChange={(e) => updateCounter(line.id, { description: e.target.value })}
                      placeholder="Line memo"
                    />
                  </div>
                  <div>
                    <input
                      className="amount"
                      type="text"
                      inputMode="numeric"
                      value=""
                      readOnly
                      aria-readonly="true"
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
                      className="entry-grid__remove"
                      onClick={() => removeCounter(line.id)}
                      aria-label="Remove counter line"
                      disabled={counterLines.length === 1}
                    >
                      ×
                    </button>
                  </div>
                </div>
              ))
            )}

            {/* Add counter line + transfer amount */}
            {!isTransfer ? (
              <div className="entry-grid__row entry-grid__row--add">
                <div className="entry-grid__num">+</div>
                <div>
                  <button type="button" className="entry-grid__add" onClick={addCounter}>
                    + Add counter line
                  </button>
                </div>
                <div />
                <div />
                <div />
                <div />
              </div>
            ) : (
              <div className="entry-grid__row entry-grid__row--add">
                <div className="entry-grid__num">+</div>
                <div>
                  <label className="field field--inline">
                    <span className="field__label">Transfer amount</span>
                    <input
                      className="input"
                      type="text"
                      inputMode="numeric"
                      value={formatAmountInput(transferAmount)}
                      onChange={(e) => {
                        const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                        setTransferAmount(digits);
                      }}
                      placeholder="0"
                    />
                  </label>
                </div>
                <div />
                <div />
                <div />
                <div />
              </div>
            )}
          </div>

          <div className="entry-grid__totals">
            <span>Totals</span>
            <span>
              D <strong>{formatIDR(cashAmountCents)}</strong>
            </span>
            <span>
              C <strong>{formatIDR(countersTotals)}</strong>
            </span>
            <span className={diff === 0 ? "" : "is-off"}>
              Diff <strong>{formatIDR(Math.abs(diff))}</strong>
            </span>
          </div>
        </div>

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

function seedCounterLine(): CounterLine {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: "",
    description: "",
    amount: "",
  };
}

function draftNumber(subKind: EntrySubKind): string {
  switch (subKind) {
    case "money-in":
      return "OR-DRAFT";
    case "money-out":
      return "OP-DRAFT";
    case "cash-transfer":
      return "BT-DRAFT";
    default:
      return "DRAFT";
  }
}
