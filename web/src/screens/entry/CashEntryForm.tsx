import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, FormError, LoadingState } from "../../components/ui";
import { api, mockHelpers } from "../../api";
import { formatIDR } from "../../lib/format";
import type { AccountItem, EntrySubKind } from "../../types";

interface Props {
  tabId: string;
  subKind: EntrySubKind;
  /** Entry id when editing an existing entry; absent for a draft. */
  entryId?: string | number;
  /** Persisted number when editing an existing entry. */
  initialTitle?: string;
}

interface Line {
  id: string;
  accountId: string;
  accountName: string;
  description: string;
  amount: string;
  /** Side this line lives on; defaults to debit. */
  side: "debit" | "credit";
}

/**
 * Entry form for cash & bank. Header carries date, number, and the
 * appropriate account selector (cash account + counter for receipts /
 * payments; from + to for transfers). The body is a balanced multi-line
 * journal grid: every line picks an account, a description, and either
 * a debit or credit amount; the totals must balance to zero before posting.
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
  const [fromAccount, setFromAccount] = useState("");
  const [toAccount, setToAccount] = useState("");
  const [lines, setLines] = useState<Line[]>(() => seedLines(subKind));

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
  }, [tabId, date, number, memo, payee, fromAccount, toAccount, lines, workbench]);

  const accountByID = useMemo(() => {
    const map = new Map<string, AccountItem>();
    for (const a of accounts) map.set(String(a.id), a);
    return map;
  }, [accounts]);

  const totals = useMemo(() => {
    let debit = 0;
    let credit = 0;
    for (const line of lines) {
      const cents = parseCents(line.amount);
      if (line.side === "debit") debit += cents;
      else credit += cents;
    }
    return { debit, credit, diff: debit - credit };
  }, [lines]);

  const handleAccountChange = (lineId: string, accountId: string) => {
    const account = accountByID.get(accountId);
    setLines((current) =>
      current.map((line) =>
        line.id === lineId ? { ...line, accountId, accountName: account?.name ?? "" } : line,
      ),
    );
  };

  const updateLine = (lineId: string, patch: Partial<Line>) => {
    setLines((current) => current.map((line) => (line.id === lineId ? { ...line, ...patch } : line)));
  };

  const removeLine = (lineId: string) => {
    setLines((current) => (current.length > 2 ? current.filter((line) => line.id !== lineId) : current));
  };

  const addLine = (side: "debit" | "credit") => {
    setLines((current) => [
      ...current,
      {
        id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        accountId: "",
        accountName: "",
        description: "",
        amount: "",
        side,
      },
    ]);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (totals.diff !== 0) {
      setError("Debits and credits must balance before posting.");
      return;
    }
    const totalCents = Math.max(totals.debit, totals.credit);
    if (totalCents <= 0) {
      setError("Enter a positive amount.");
      return;
    }
    setError(null);
    setSaving(true);
    try {
      if (entryId) {
        await api.reverseCash(Number(entryId));
      } else if (subKind === "money-in") {
        await api.postCashIn({
          entry_date: date,
          description: memo,
          cash_account_id: Number(fromAccount),
          counter_account_id: Number(toAccount),
          amount_cents: totalCents,
        });
      } else if (subKind === "money-out") {
        await api.postCashOut({
          entry_date: date,
          description: memo,
          cash_account_id: Number(fromAccount),
          counter_account_id: Number(toAccount),
          amount_cents: totalCents,
        });
      } else {
        await api.postTransfer({
          entry_date: date,
          description: memo,
          from_account_id: Number(fromAccount),
          to_account_id: Number(toAccount),
          amount_cents: totalCents,
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

  return (
    <form className="entrytab" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{titleLabel}</span>
          <span className={`entrytab__status ${saved ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {status}
          </span>
          <span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", color: "var(--ink-muted)" }}>
            {number}
          </span>
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

          {subKind === "cash-transfer" ? (
            <>
              <label className="field">
                <span className="field__label">From account</span>
                <select className="input" value={fromAccount} onChange={(e) => setFromAccount(e.target.value)}>
                  <option value="">Select...</option>
                  {accountOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">To account</span>
                <select className="input" value={toAccount} onChange={(e) => setToAccount(e.target.value)}>
                  <option value="">Select...</option>
                  {accountOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
            </>
          ) : (
            <>
              <label className="field">
                <span className="field__label">Cash / Bank account</span>
                <select className="input" value={fromAccount} onChange={(e) => setFromAccount(e.target.value)}>
                  <option value="">Select...</option>
                  {accountOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Counter account</span>
                <select className="input" value={toAccount} onChange={(e) => setToAccount(e.target.value)}>
                  <option value="">Select...</option>
                  {accountOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
            </>
          )}

          <label className="field">
            <span className="field__label">Payee / Payer</span>
            <input className="input" value={payee} onChange={(e) => setPayee(e.target.value)} placeholder="Name" />
          </label>

          <label className="field">
            <span className="field__label">Memo</span>
            <input className="input" value={memo} onChange={(e) => setMemo(e.target.value)} placeholder="Short description" />
          </label>
        </div>

        <div style={{ padding: 0 }}>
          <div className="entrytab__section-title" style={{ padding: "var(--space-5) var(--space-5) var(--space-2)" }}>
            Account lines
          </div>
          <div className="entry-grid">
            <div className="entry-grid__head">
              <div>#</div>
              <div>Account</div>
              <div>Account name</div>
              <div>Description</div>
              <div className="right">Debit</div>
              <div className="right">Credit</div>
              <div aria-hidden="true" />
            </div>
            {lines.map((line, idx) => (
              <div className="entry-grid__row" key={line.id}>
                <div className="entry-grid__num">{idx + 1}</div>
                <div>
                  <select value={line.accountId} onChange={(e) => handleAccountChange(line.id, e.target.value)}>
                    <option value="">Select...</option>
                    {accountOptions.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <input type="text" value={line.accountName} readOnly style={{ color: "var(--ink-tertiary)" }} />
                </div>
                <div>
                  <input
                    type="text"
                    value={line.description}
                    onChange={(e) => updateLine(line.id, { description: e.target.value })}
                    placeholder="Line memo"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={line.side === "credit" ? "" : formatAmountInput(line.amount)}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateLine(line.id, { amount: digits, side: "debit" });
                    }}
                    placeholder="0"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={line.side === "debit" ? "" : formatAmountInput(line.amount)}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateLine(line.id, { amount: digits, side: "credit" });
                    }}
                    placeholder="0"
                  />
                </div>
                <div>
                  <button
                    type="button"
                    className="entry-grid__remove"
                    onClick={() => removeLine(line.id)}
                    aria-label="Remove line"
                  >
                    ×
                  </button>
                </div>
              </div>
            ))}
            <div className="entry-grid__row">
              <div className="entry-grid__num">+</div>
              <div style={{ display: "flex", gap: "var(--space-2)" }}>
                <button type="button" className="entry-grid__add" onClick={() => addLine("debit")}>
                  + Debit line
                </button>
                <button type="button" className="entry-grid__add" onClick={() => addLine("credit")}>
                  + Credit line
                </button>
              </div>
              <div /><div /><div /><div /><div />
            </div>
          </div>
          <div className="entry-grid__totals">
            <span>Totals</span>
            <span>
              D <strong>{formatIDR(totals.debit)}</strong>
            </span>
            <span>
              C <strong>{formatIDR(totals.credit)}</strong>
            </span>
            <span className={totals.diff === 0 ? "" : "is-off"}>
              Diff <strong>{formatIDR(Math.abs(totals.diff))}</strong>
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

function seedLines(subKind: EntrySubKind): Line[] {
  const base: Line[] = [
    {
      id: "ln-debit",
      accountId: "",
      accountName: "",
      description: "",
      amount: "",
      side: "debit",
    },
    {
      id: "ln-credit",
      accountId: "",
      accountName: "",
      description: "",
      amount: "",
      side: "credit",
    },
  ];
  if (subKind === "money-in") {
    base[0].description = "Receipt";
    base[1].description = "Counter account";
  } else if (subKind === "money-out") {
    base[0].description = "Counter account";
    base[1].description = "Payment";
  } else {
    base[0].description = "Source account";
    base[1].description = "Destination account";
  }
  return base;
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
