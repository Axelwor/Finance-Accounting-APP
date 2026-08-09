import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { BackendAccount, GeneralLedgerResult } from "../../types";

/**
 * General Ledger (Buku Besar) — Accountant Mode v1.
 *
 * Pick an account and an optional date range. The screen shows the
 * opening balance (signed debit-credit sum before `from`), each posted
 * movement inside the window with a running balance, and the closing
 * balance.
 */
export function GeneralLedger() {
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [loadingMasters, setLoadingMasters] = useState(true);
  const [accountId, setAccountId] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [result, setResult] = useState<GeneralLedgerResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadMasters();
  }, []);

  const loadMasters = async () => {
    try {
      const data = await api.listBackendAccounts();
      setAccounts(data.filter((a) => a.is_active && !a.is_group));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load accounts.");
    } finally {
      setLoadingMasters(false);
    }
  };

  const load = async () => {
    if (!accountId) {
      setError("Pick an account to view its ledger.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await api.getGeneralLedger({
        account_id: Number(accountId),
        from_date: fromDate || undefined,
        to_date: toDate || undefined,
      });
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the ledger.");
    } finally {
      setLoading(false);
    }
  };

  if (loadingMasters) return <LoadingState label="Loading accounts..." />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>General Ledger</span>
          <small>Per-account movements with opening and closing balances (buku besar).</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Account</span>
            <select
              className="filter-pill__input"
              value={accountId}
              onChange={(e) => setAccountId(e.target.value)}
            >
              <option value="">Choose account...</option>
              {accounts.map((a) => (
                <option key={a.id} value={String(a.id)}>
                  {a.code} · {a.name}
                </option>
              ))}
            </select>
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">From</span>
            <input
              type="date"
              className="filter-pill__input"
              value={fromDate}
              onChange={(e) => setFromDate(e.target.value)}
            />
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">To</span>
            <input
              type="date"
              className="filter-pill__input"
              value={toDate}
              onChange={(e) => setToDate(e.target.value)}
            />
          </label>
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Run
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : loading ? (
          <LoadingState label="Computing ledger..." />
        ) : !result ? (
          <EmptyState
            title="No account selected"
            message="Pick an account above and press Run to view its ledger."
          />
        ) : (
          <>
            <div className="ledger-summary">
              <div className="ledger-summary__head">
                <span>
                  {result.account_code} · {result.account_name}
                </span>
                <span>Opening: {formatIDR(result.opening_balance_cents)}</span>
                <span>Closing: {formatIDR(result.closing_balance_cents)}</span>
              </div>
            </div>
            {result.entries.length === 0 ? (
              <EmptyState title="No movements" message="No posted lines on this account in the selected window." />
            ) : (
              <div className="ledger-table">
                <div className="ledger-table__head">
                  <span>Number</span>
                  <span>Date</span>
                  <span>Description</span>
                  <span className="right">Debit</span>
                  <span className="right">Credit</span>
                  <span className="right">Balance</span>
                </div>
                {result.entries.map((e, i) => (
                  <div className="ledger-table__row" key={`${e.entry_number}-${i}`}>
                    <span className="ledger-table__no">{e.entry_number}</span>
                    <span className="ledger-table__date">{e.entry_date}</span>
                    <span className="ledger-table__cat">{e.description || "—"}</span>
                    <span className="ledger-table__amount right">
                      {e.debit_cents ? formatIDR(e.debit_cents) : "—"}
                    </span>
                    <span className="ledger-table__amount right">
                      {e.credit_cents ? formatIDR(e.credit_cents) : "—"}
                    </span>
                    <span className="ledger-table__amount right">{formatIDR(e.running_balance_cents)}</span>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
