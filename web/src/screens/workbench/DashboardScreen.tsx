import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api";
import { useAppState } from "../../state";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, LoadingState } from "../../components/ui";
import { PeriodCard } from "../../components/period";
import { TransactionRow } from "../../components/transactions";
import { formatIDR } from "../../lib/format";
import type { DashboardSummary } from "../../types";

/** Today's date, formatted for the greeting header (computed once per mount). */
function useTodayStamp(): string {
  return useMemo(
    () =>
      new Intl.DateTimeFormat("en-US", {
        weekday: "long",
        day: "2-digit",
        month: "long",
        year: "numeric",
      }).format(new Date()),
    [],
  );
}

function Spark({ values, tone }: { values: number[]; tone: "pos" | "neg" | "acc" }) {
  // Flat CSS-gradient bar when there is not enough data for a real line.
  // Replaces the previous Math.sin() drift — no fabricated shape.
  if (values.length < 2) {
    const cls = tone === "neg" ? "spark spark--neg spark--flat" : tone === "acc" ? "spark spark--acc spark--flat" : "spark spark--flat";
    return <div className={cls} aria-hidden="true" />;
  }
  const width = 220;
  const height = 28;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = width / (values.length - 1);
  const path = values
    .map((v, i) => {
      const x = i * stepX;
      const y = height - ((v - min) / range) * height;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  const cls = tone === "neg" ? "spark spark--neg" : tone === "acc" ? "spark spark--acc" : "spark";
  return (
    <div className={cls}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-hidden="true">
        <path d={path} />
      </svg>
    </div>
  );
}

/** Small inline chart icon (keeps the dashboard free of icon dependencies). */
function ChartIcon() {
  return (
    <svg
      className="status-cell__icon"
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2 12 L6 7 L9 10 L14 4" />
      <path d="M2 14 L14 14" opacity="0.4" />
    </svg>
  );
}

function StatusCell({
  label,
  value,
  delta,
  deltaTone,
  tone,
  spark,
  sparkTone,
  lead,
  meta,
  onDetails,
}: {
  label: string;
  value: string;
  delta: string;
  deltaTone: "pos" | "neg" | "neutral";
  tone: "pos" | "neg" | "acc" | "warn" | "neutral";
  spark?: number[];
  sparkTone?: "pos" | "neg" | "acc";
  lead?: boolean;
  meta: string;
  /** Click handler for the "View details" link; when omitted, no link is shown. */
  onDetails?: () => void;
}) {
  const dotClass =
    tone === "pos" ? "" : tone === "neg" ? "dot--neg" : tone === "warn" ? "dot--warn" : tone === "acc" ? "dot--acc" : "";
  const valueCls =
    tone === "pos" ? "is-positive" : tone === "neg" ? "is-negative" : tone === "warn" ? "is-warning" : "";
  return (
    <div className={`status-cell${lead ? " status-cell--lead" : ""}`}>
      <div className="status-cell__label">
        <ChartIcon />
        <span className={`dot ${dotClass}`} aria-hidden="true" />
        <span>{label}</span>
      </div>
      <p className={`status-cell__value${valueCls ? " " + valueCls : ""}`}>{value}</p>
      <div className="status-cell__delta">
        <span>{delta}</span>
        <strong className={`is-${deltaTone}`}>{meta}</strong>
      </div>
      {spark && sparkTone ? <Spark values={spark} tone={sparkTone} /> : null}
      {onDetails ? (
        <button type="button" className="status-cell__details" onClick={onDetails}>
          View details
        </button>
      ) : null}
    </div>
  );
}

function KpiRow({
  label,
  note,
  value,
  tone,
  dotTone,
}: {
  label: string;
  note: string;
  value: string;
  tone: "pos" | "neg" | "muted";
  dotTone: "pos" | "neg" | "warn" | "neutral";
}) {
  const dotCls =
    dotTone === "pos" ? "is-pos" : dotTone === "neg" ? "is-neg" : dotTone === "warn" ? "is-warn" : "";
  const valueCls = tone ? ` is-${tone}` : "";
  return (
    <div className="kpi-list__row">
      <div className="kpi-list__label">
        <span className="kpi-list__label-title">{label}</span>
        <span className="kpi-list__label-note">{note}</span>
      </div>
      <span className={`kpi-list__value${valueCls}`}>{value}</span>
      <span className={`kpi-list__dot ${dotCls}`} aria-hidden="true" />
    </div>
  );
}

/**
 * Dashboard content. Rendered inside the workbench as the default tab.
 * Owns no chrome — page head lives on the tab pill + section heads.
 */
export function DashboardScreen() {
  const workbench = useWorkbench();
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
      setError(err instanceof Error ? err.message : "Failed to load the dashboard. Try again.");
    }
  }, []);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryKey]); // Only reload when retryKey changes (manual retry)

  const businessName = business?.name || user?.businessName || "Your business";
  const todayStamp = useTodayStamp();
  const recent = data?.recentTransactions ?? [];

  // Real cash trend: build a sparkline from the recent transactions the
  // backend actually returned (running balance). When there is not enough
  // data, Spark renders a flat gradient bar — no fabricated Math.sin() drift.
  const spark = useMemo<number[]>(() => {
    if (!data || recent.length < 2) return [];
    const running: number[] = [];
    let acc = 0;
    // Oldest first so the line reads left-to-right over time.
    for (const trx of [...recent].reverse()) {
      acc += trx.kind === "money-in" ? trx.amount : trx.kind === "money-out" ? -trx.amount : 0;
      running.push(acc);
    }
    // Anchor the end of the sparkline to the real reported balance so the
    // final point reflects the backend figure, not the local sample.
    if (running.length) running[running.length - 1] = data.cashAndBankBalance;
    return running;
  }, [data, recent]);

  // Real entries trend: count of transactions per day over the last 14 days
  // from the local store. No synthetic drift — flat zero when empty.
  const entriesSpark = useMemo<number[]>(() => {
    if (transactions.length === 0) return [];
    const days = 14;
    const buckets = new Array(days).fill(0);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    for (const trx of transactions) {
      const [y, m, d] = trx.date.split("-").map(Number);
      if (!y || !m || !d) continue;
      const t = new Date(y, m - 1, d);
      t.setHours(0, 0, 0, 0);
      const idx = Math.floor((today.getTime() - t.getTime()) / 86_400_000);
      if (idx >= 0 && idx < days) buckets[days - 1 - idx] += 1;
    }
    return buckets;
  }, [transactions]);

  return (
    <div className="dashboard">
      <header className="page-head">
        <div className="page-head__left">
          <p className="page-head__meta">
            <strong>{todayStamp}</strong>
          </p>
          <h1 className="page-title">
            <span>{businessName}</span>
          </h1>
          <p className="page-sub">
            {businessName} financial overview — today's positions and quick access to posting.
          </p>
        </div>
        <div className="page-head__actions">
          <button
            type="button"
            className="btn btn--secondary"
            onClick={() => workbench.openList("cash-other-payment")}
          >
            Open ledger
          </button>
          <button
            type="button"
            className="btn btn--primary"
            onClick={() => workbench.openEntryDraft("money-in")}
          >
            New entry
          </button>
        </div>
      </header>

      {error ? (
        <ErrorState message={error} onRetry={() => setRetryKey((k) => k + 1)} />
      ) : !data ? (
        <LoadingState label="Loading dashboard..." />
      ) : (
        <>
          <section className="status-rail" aria-label="Position overview">
            <StatusCell
              lead
              label="Cash & Bank"
              value={formatIDR(data.cashAndBankBalance)}
              delta="Cash and accounts combined"
              deltaTone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              tone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              spark={spark}
              sparkTone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              meta="End of day"
              onDetails={() => workbench.openList("cash-other-payment")}
            />
            <StatusCell
              label="MTD P&amp;L"
              value={formatIDR(data.monthlyProfitLoss)}
              delta="Month to date"
              deltaTone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
              tone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
              spark={entriesSpark}
              sparkTone="acc"
              meta="vs last cycle"
              onDetails={() => workbench.openList("cash-other-payment")}
            />
            <StatusCell
              label="Open bills"
              value={String(data.dueBills)}
              delta="Receivables awaiting payment"
              deltaTone={data.dueBills > 0 ? "neg" : "neutral"}
              tone={data.dueBills > 0 ? "warn" : "neutral"}
              meta={data.dueBills > 0 ? "Action needed" : "Clear"}
              onDetails={() => workbench.openList("sales-invoice")}
            />
            <StatusCell
              label="Low stock"
              value={String(data.lowStock)}
              delta="Items below reorder point"
              deltaTone={data.lowStock > 0 ? "neg" : "neutral"}
              tone={data.lowStock > 0 ? "warn" : "neutral"}
              meta={data.lowStock > 0 ? "Restock" : "Stable"}
              onDetails={() => workbench.openList("inventory-items")}
            />
          </section>

          <section className="section">
            <div className="section-head">
              <h2 className="section-head__title">
                <span className="dot dot--pos" aria-hidden="true" />
                Latest entries
              </h2>
              <span className="section-head__meta">{recent.length} total — top 5 shown</span>
              {recent.length > 5 ? (
                <button
                  type="button"
                  className="section-head__action"
                  onClick={() => workbench.openList("cash-other-payment")}
                >
                  View ledger
                </button>
              ) : null}
            </div>

            {recent.length === 0 ? (
              <div className="empty-state">
                <h3 className="empty-state__title">No entries in the book yet</h3>
                <p className="empty-state__message">
                  Start with your first money in or money out. The dashboard updates as you rule each line.
                </p>
                <button
                  type="button"
                  className="btn btn--primary"
                  onClick={() => workbench.openEntryDraft("money-in")}
                >
                  Record first entry
                </button>
              </div>
            ) : (
              <div className="ledger-table">
                <div className="ledger-table__head">
                  <span>Date</span>
                  <span>Description</span>
                  <span>Category</span>
                  <span className="right">Amount</span>
                  <span aria-hidden="true" />
                </div>
                {recent.slice(0, 5).map((t) => (
                  <TransactionRow key={t.id} transaction={t} />
                ))}
              </div>
            )}
          </section>

          <section className="section">
            <div className="section-head">
              <h2 className="section-head__title">
                <span className="dot dot--acc" aria-hidden="true" />
                Indicators
              </h2>
              <span className="section-head__meta">Sourced from the same ledger</span>
            </div>
            <div className="kpi-list">
              <KpiRow
                label="This month's P&amp;L"
                note="Income minus expense"
                value={formatIDR(data.monthlyProfitLoss)}
                tone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
                dotTone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
              />
              <KpiRow
                label="Open receivables"
                note="Bills waiting to be collected"
                value={String(data.dueBills)}
                tone={data.dueBills > 0 ? "neg" : "muted"}
                dotTone={data.dueBills > 0 ? "neg" : "neutral"}
              />
              <KpiRow
                label="Items below reorder point"
                note="Counted at the last inventory check"
                value={String(data.lowStock)}
                tone={data.lowStock > 0 ? "neg" : "muted"}
                dotTone={data.lowStock > 0 ? "warn" : "neutral"}
              />
            </div>
          </section>

          <PeriodCard />
        </>
      )}
    </div>
  );
}
