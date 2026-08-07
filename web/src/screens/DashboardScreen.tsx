import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Card, ErrorState, LoadingState } from "../components/ui";
import { PeriodCard } from "../components/period";
import { TransactionRow } from "../components/transactions";
import { formatIDR } from "../lib/format";
import type { DashboardSummary } from "../types";

const wordmark = "Ledgerly";

interface KpiCardProps {
  label: string;
  value: string;
  note?: string;
}

function KpiCard({ label, value, note }: KpiCardProps) {
  return (
    <section className="kpi-card">
      <p className="kpi-card__label">{label}</p>
      <p className="kpi-card__nilai">{value}</p>
      {note ? <p className="kpi-card__catatan">{note}</p> : null}
    </section>
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

  return (
    <div className="dashboard">
      <header className="page-head">
        <div>
          <h1 className="page-title">Hello, {businessName}</h1>
          <p className="page-sub">A summary of your business today.</p>
        </div>
        <div className="page-head__actions">
          <Link className="btn btn--primary" to="/record/money-in">
            Record transaction
          </Link>
        </div>
      </header>

      {error ? (
        <ErrorState message={error} onRetry={() => setRetryKey((k) => k + 1)} />
      ) : !data ? (
        <LoadingState label="Loading summary..." />
      ) : (
        <>
          <div className="kpi-grid">
            <KpiCard label="Cash & Bank Balance" value={formatIDR(data.cashAndBankBalance)} note="Cash and accounts combined" />
            <KpiCard
              label="This month's Profit/Loss"
              value={formatIDR(data.monthlyProfitLoss)}
              note={data.monthlyProfitLoss >= 0 ? "Money in minus money out" : "Expenses are greater than income"}
            />
            <KpiCard label="Due bills" value={String(data.dueBills)} note="Example: 2 bills waiting" />
            <KpiCard label="Low stock" value={String(data.lowStock)} note="Example: 4 items to restock" />
          </div>

          <Card
            title="Recent transactions"
            description="The latest records from your books."
          >
            {data.recentTransactions.length === 0 ? (
              <div className="empty-state">
                <h3 className="empty-state__title">No records yet</h3>
                <p className="empty-state__message">
                  Start by recording your first money in or money out.
                </p>
                <Link className="btn btn--primary" to="/record/money-in">
                  Record first transaction
                </Link>
              </div>
            ) : (
              <ul className="transaction-list">
                {data.recentTransactions.slice(0, 5).map((t) => (
                  <TransactionRow key={t.id} transaction={t} />
                ))}
              </ul>
            )}
            {data.recentTransactions.length > 5 ? (
              <div className="card__footer">
                <Link className="link-inline" to="/transactions">
                  View all records
                </Link>
              </div>
            ) : null}
          </Card>

          <PeriodCard />
        </>
      )}

    </div>
  );
}
