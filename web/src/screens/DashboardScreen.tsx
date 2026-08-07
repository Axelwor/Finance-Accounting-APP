import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Card, ErrorState, LoadingState } from "../components/ui";
import { PeriodCard } from "../components/period";
import { TransactionRow } from "../components/transactions";
import { formatIDR, formatDate } from "../lib/format";
import type { DashboardSummary } from "../types";

const wordmark = "Ledgerly";

const todayLong = new Intl.DateTimeFormat("en-US", {
  weekday: "long",
  month: "long",
  day: "numeric",
  year: "numeric",
}).format(new Date());

const todayStamp = new Intl.DateTimeFormat("en-US", {
  day: "2-digit",
  month: "short",
  year: "numeric",
}).format(new Date()).toUpperCase();

interface LedgerRowProps {
  label: string;
  note: string;
  value: string;
  tone?: "positive" | "negative" | "muted";
}

function LedgerRow({ label, note, value, tone }: LedgerRowProps) {
  return (
    <div className="ledger-row">
      <div className="ledger-row__label">
        <span className="ledger-row__label-title">{label}</span>
        <span className="ledger-row__label-note">{note}</span>
      </div>
      <span className={`ledger-row__value${tone ? ` is-${tone}` : ""}`}>{value}</span>
    </div>
  );
}

/** Main summary page after login / onboarding. */
export function DashboardScreen() {
  const { user, business, transactions } = useAppState();
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  const load = useCallback(async () => {
    setError(null);
    try {
      const summary = await api.getDashboard();
      setData(summary);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the summary. Try again.");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load, retryKey, transactions]);

  useEffect(() => {
    document.title = business ? `${business.name} - ${wordmark}` : wordmark;
  }, [business]);

  const businessName = business?.name || user?.businessName || "Your business";

  const profitTone = data && data.monthlyProfitLoss >= 0 ? "positive" : "negative";

  const recent = useMemo(() => data?.recentTransactions ?? [], [data]);

  return (
    <div className="dashboard">
      <header className="page-head">
        <div>
          <p className="page-head__meta">{todayLong} / Vol. 1</p>
          <h1 className="page-title">
            {businessName} <em>ledger</em>
          </h1>
          <p className="page-sub">A ruled summary of the books, stamped at the start of today.</p>
        </div>
        <div className="page-head__actions">
          <Link className="btn btn--secondary" to="/transactions">
            Open ledger
          </Link>
          <Link className="btn btn--primary" to="/record/money-in">
            New entry
          </Link>
        </div>
      </header>

      {error ? (
        <ErrorState message={error} onRetry={() => setRetryKey((k) => k + 1)} />
      ) : !data ? (
        <LoadingState label="Stamping the day..." />
      ) : (
        <>
          <div className="ledger-stamp">
            <div className="ledger-stamp__head">
              <span className="ledger-stamp__label">Cash & Bank — Today's Standing</span>
              <span className="ledger-stamp__date">{todayStamp}</span>
            </div>
            <p className={`ledger-stamp__amount${data.cashAndBankBalance >= 0 ? " is-positive" : " is-negative"}`}>
              {formatIDR(data.cashAndBankBalance)}
            </p>
            <div className="ledger-stamp__sub">
              <span>Money in this month</span>
              <strong>{formatIDR(data.monthlyProfitLoss >= 0 ? data.monthlyProfitLoss : 0)}</strong>
            </div>
          </div>

          <div className="ledger-rows">
            <LedgerRow
              label="This month's Profit/Loss"
              note={data.monthlyProfitLoss >= 0 ? "Income minus expense" : "Expense exceeds income"}
              value={formatIDR(data.monthlyProfitLoss)}
              tone={profitTone}
            />
            <LedgerRow
              label="Due bills"
              note="Open receivables on the books"
              value={String(data.dueBills)}
              tone={data.dueBills > 0 ? "negative" : "muted"}
            />
            <LedgerRow
              label="Low stock items"
              note="Reorder before the next count"
              value={String(data.lowStock)}
              tone={data.lowStock > 0 ? "negative" : "muted"}
            />
          </div>

          <section className="section">
            <div className="section-head">
              <h2 className="section-head__title">Latest entries</h2>
              <p className="section-head__meta">{recent.length} recent</p>
              {recent.length > 5 ? (
                <Link className="section-head__action" to="/transactions">
                  View full ledger
                </Link>
              ) : null}
            </div>

            {recent.length === 0 ? (
              <div className="empty-state">
                <h3 className="empty-state__title">No entries yet</h3>
                <p className="empty-state__message">
                  Open the ledger with your first money in or money out. The book is yours to rule.
                </p>
                <Link className="btn btn--primary" to="/record/money-in">
                  Record first entry
                </Link>
              </div>
            ) : (
              <Card>
                <ul className="tx-table">
                  <li className="tx-table__head">
                    <span>Date</span>
                    <span>Description</span>
                    <span>Category</span>
                    <span style={{ textAlign: "right" }}>Amount</span>
                    <span aria-hidden="true" />
                  </li>
                  {recent.slice(0, 5).map((t) => (
                    <TransactionRow key={t.id} transaction={t} />
                  ))}
                </ul>
              </Card>
            )}
          </section>

          <PeriodCard />
        </>
      )}
    </div>
  );
}
