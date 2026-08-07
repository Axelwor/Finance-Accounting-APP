import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api";
import { useAppState } from "../../state";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, LoadingState } from "../../components/ui";
import { PeriodCard } from "../../components/period";
import { TransactionRow } from "../../components/transactions";
import { formatIDR } from "../../lib/format";
import type { DashboardSummary } from "../../types";

const todayStamp = new Intl.DateTimeFormat("en-US", {
  weekday: "long",
  day: "2-digit",
  month: "long",
  year: "numeric",
}).format(new Date());

const shortDateStamp = new Intl.DateTimeFormat("en-US", {
  day: "2-digit",
  month: "short",
  year: "numeric",
}).format(new Date()).toUpperCase();

function Spark({ values, tone }: { values: number[]; tone: "pos" | "neg" | "acc" }) {
  if (values.length < 2) return null;
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
}) {
  const dotClass =
    tone === "pos" ? "" : tone === "neg" ? "dot--neg" : tone === "warn" ? "dot--warn" : tone === "acc" ? "dot--acc" : "";
  const valueCls =
    tone === "pos" ? "is-positive" : tone === "neg" ? "is-negative" : tone === "warn" ? "is-warning" : "";
  return (
    <div className={`status-cell${lead ? " status-cell--lead" : ""}`}>
      <div className="status-cell__label">
        <span className={`dot ${dotClass}`} aria-hidden="true" />
        <span>{label}</span>
      </div>
      <p className={`status-cell__value${valueCls ? " " + valueCls : ""}`}>{value}</p>
      <div className="status-cell__delta">
        <span>{delta}</span>
        <strong className={`is-${deltaTone}`}>{meta}</strong>
      </div>
      {spark && sparkTone ? <Spark values={spark} tone={sparkTone} /> : null}
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
      setError(err instanceof Error ? err.message : "Failed to load the console. Try again.");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load, retryKey, transactions]);

  const businessName = business?.name || user?.businessName || "Your business";
  const recent = data?.recentTransactions ?? [];

  const spark = useMemo<number[]>(() => {
    if (!data) return [];
    const base = Math.max(1, Math.abs(data.cashAndBankBalance) / 1_000_000);
    return Array.from({ length: 14 }, (_, i) => {
      const drift = Math.sin((i + businessName.length) * 0.5) * base * 0.4;
      const linear = (data.cashAndBankBalance / 14) * i;
      return Math.max(0, base + linear + drift);
    });
  }, [data, businessName]);

  const entriesSpark = useMemo<number[]>(() => {
    const seed = transactions.length || 1;
    return Array.from({ length: 14 }, (_, i) =>
      Math.max(0, Math.round(seed + Math.sin(i * 0.9) * 3 + i * 0.4)),
    );
  }, [transactions.length]);

  return (
    <div className="dashboard">
      <header className="page-head">
        <div className="page-head__left">
          <p className="page-head__meta">
            <strong>{todayStamp}</strong>
          </p>
          <h1 className="page-title">
            <span>{businessName}</span>
            <span className="page-title__sep">/</span>
            <span className="page-title__sub">today's ledger</span>
          </h1>
          <p className="page-sub">
            A ruled summary of the books. Read positions, rule entries, close periods — all from a single source.
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
        <LoadingState label="Loading console..." />
      ) : (
        <>
          <section className="status-rail" aria-label="Position overview">
            <StatusCell
              lead
              label="Cash & Bank"
              value={formatIDR(data.cashAndBankBalance)}
              delta="Cash + accounts combined"
              deltaTone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              tone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              spark={spark}
              sparkTone={data.cashAndBankBalance >= 0 ? "pos" : "neg"}
              meta={`${shortDateStamp} END-OF-DAY`}
            />
            <StatusCell
              label="MTD P&L"
              value={formatIDR(data.monthlyProfitLoss)}
              delta="Month to date"
              deltaTone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
              tone={data.monthlyProfitLoss >= 0 ? "pos" : "neg"}
              spark={entriesSpark}
              sparkTone="acc"
              meta="VS LAST CYCLE"
            />
            <StatusCell
              label="Open bills"
              value={String(data.dueBills)}
              delta="Receivables awaiting payment"
              deltaTone={data.dueBills > 0 ? "neg" : "neutral"}
              tone={data.dueBills > 0 ? "warn" : "neutral"}
              meta={data.dueBills > 0 ? "ACTION NEEDED" : "CLEAR"}
            />
            <StatusCell
              label="Low stock"
              value={String(data.lowStock)}
              delta="Items below reorder point"
              deltaTone={data.lowStock > 0 ? "neg" : "neutral"}
              tone={data.lowStock > 0 ? "warn" : "neutral"}
              meta={data.lowStock > 0 ? "RESTOCK" : "STABLE"}
            />
          </section>

          <section className="section">
            <div className="section-head">
              <h2 className="section-head__title">
                <span className="dot dot--pos" aria-hidden="true" />
                Latest entries
              </h2>
              <span className="section-head__meta">{recent.length} total &middot; top 5 shown</span>
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
                <h3 className="empty-state__title">No entries in the book</h3>
                <p className="empty-state__message">
                  Open the book with your first money in or money out. The console reads from every line you rule.
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
                label="This month's P&L"
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
