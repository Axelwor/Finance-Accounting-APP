import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api";
import { useAppState } from "../../state";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, LoadingState } from "../../components/ui";
import { PeriodCard } from "../../components/period";
import { KpiCard } from "../../components/dashboard/KpiCard";
import { AgingChart } from "../../components/dashboard/AgingChart";
import { RecentTxnsWidget } from "../../components/dashboard/RecentTxnsWidget";
import { LowStockWidget } from "../../components/dashboard/LowStockWidget";
import { TaxSummaryWidget } from "../../components/dashboard/TaxSummaryWidget";
import { formatIDR } from "../../lib/format";
import type {
  AgingSummary,
  JournalEntryListItem,
  LowStockItem,
  PeriodStatusData,
  PPNSummary,
} from "../../types";

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

interface WidgetBundle<T> {
  data: T | null;
  error: boolean;
}

/**
 * Dashboard content. Rendered inside the workbench as the default tab.
 *
 * The dashboard fetches each widget independently via Promise.allSettled so
 * one failing endpoint never collapses the whole screen — the affected card
 * shows an empty state while the rest keep working.
 */
export function DashboardScreen() {
  const workbench = useWorkbench();
  const { user, business, transactions } = useAppState();
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);
  const [loading, setLoading] = useState(true);

  // Per-widget state — each can fail independently.
  const [cashBalance, setCashBalance] = useState<number | null>(null);
  const [profitLoss, setProfitLoss] = useState<number | null>(null);
  const [arAging, setArAging] = useState<AgingSummary | null>(null);
  const [apAging, setApAging] = useState<AgingSummary | null>(null);
  const [recentTxns, setRecentTxns] = useState<JournalEntryListItem[]>([]);
  const [lowStock, setLowStock] = useState<LowStockItem[]>([]);
  const [ppn, setPpn] = useState<PPNSummary | null>(null);
  const [period, setPeriod] = useState<PeriodStatusData | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const results = await Promise.allSettled([
        api.getDashboard(), // cashBalance + profitLoss (existing aggregated call)
        api.getDashboardARAging(),
        api.getDashboardAPAging(),
        api.listRecentJournalEntries(8),
        api.getLowStockItems(),
        api.getDashboardPPNSummary(),
        api.getPeriodStatus(),
      ]);

      // getDashboard returns the cash + P&L summary.
      const dashResult = results[0].status === "fulfilled" ? results[0].value : null;
      setCashBalance(dashResult ? dashResult.cashAndBankBalance : null);
      setProfitLoss(dashResult ? dashResult.monthlyProfitLoss : null);

      setArAging(results[1].status === "fulfilled" ? results[1].value : null);
      setApAging(results[2].status === "fulfilled" ? results[2].value : null);
      setRecentTxns(results[3].status === "fulfilled" ? results[3].value : []);
      setLowStock(results[4].status === "fulfilled" ? results[4].value : []);
      setPpn(results[5].status === "fulfilled" ? results[5].value : null);
      setPeriod(results[6].status === "fulfilled" ? results[6].value : null);

      // If the aggregate dashboard call rejected entirely, surface a soft
      // error banner but keep the widget cards rendering their own states.
      if (results[0].status === "rejected" && results.every((r) => r.status === "rejected")) {
        setError("Failed to load the dashboard. Try again.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the dashboard. Try again.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryKey]);

  const businessName = business?.name || user?.businessName || "Your business";
  const todayStamp = useTodayStamp();

  // Sparkline: running balance from recent transactions (real backend data).
  const spark = useMemo<number[]>(() => {
    if (recentTxns.length < 2) return [];
    const running: number[] = [];
    let acc = 0;
    for (const trx of [...recentTxns].reverse()) {
      acc += trx.total_debit_cents;
      running.push(acc);
    }
    // Anchor the last point to the real reported balance.
    if (running.length && cashBalance !== null) {
      running[running.length - 1] = cashBalance * 100;
    }
    return running;
  }, [recentTxns, cashBalance]);

  // Sparkline: entries per day over 14 days from the local store.
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

  const arTotal = arAging?.total_cents ?? 0;
  const apTotal = apAging?.total_cents ?? 0;

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
      ) : loading ? (
        <LoadingState label="Loading dashboard..." />
      ) : (
        <div className="dashboard-grid">
          {/* KPI row — 4 cards */}
          <div className="dashboard-grid__cell dashboard-grid__cell--span-4">
            <div className="kpi-grid">
              <KpiCard
                lead
                label="Cash & Bank"
                value={cashBalance !== null ? formatIDR(cashBalance) : "—"}
                delta="Cash and accounts combined"
                deltaTone={cashBalance !== null && cashBalance >= 0 ? "pos" : "neg"}
                tone={cashBalance !== null && cashBalance >= 0 ? "pos" : "neg"}
                spark={spark}
                sparkTone={cashBalance !== null && cashBalance >= 0 ? "pos" : "neg"}
                meta="End of day"
                onDetails={() => workbench.openList("cash-other-payment")}
              />
              <KpiCard
                label="MTD P&L"
                value={profitLoss !== null ? formatIDR(profitLoss) : "—"}
                delta="Month to date"
                deltaTone={profitLoss !== null && profitLoss >= 0 ? "pos" : "neg"}
                tone={profitLoss !== null && profitLoss >= 0 ? "pos" : "neg"}
                spark={entriesSpark}
                sparkTone="acc"
                meta="vs last cycle"
                onDetails={() => workbench.openList("cash-other-payment")}
              />
              <KpiCard
                label="AR Outstanding"
                value={formatIDR(arTotal)}
                delta="Receivables awaiting payment"
                deltaTone={arTotal > 0 ? "neg" : "neutral"}
                tone={arTotal > 0 ? "warn" : "neutral"}
                meta={arTotal > 0 ? "Action needed" : "Clear"}
                onDetails={() => workbench.openList("sales-invoice")}
              />
              <KpiCard
                label="AP Outstanding"
                value={formatIDR(apTotal)}
                delta="Payables awaiting payment"
                deltaTone={apTotal > 0 ? "neg" : "neutral"}
                tone={apTotal > 0 ? "warn" : "neutral"}
                meta={apTotal > 0 ? "Due soon" : "Clear"}
                onDetails={() => workbench.openList("cash-other-payment")}
              />
            </div>
          </div>

          {/* Main row: recent transactions (8) + quick actions/period (4) */}
          <div className="dashboard-grid__cell dashboard-grid__cell--span-8">
            <RecentTxnsWidget
              transactions={recentTxns}
              onOpenLedger={() => workbench.openList("cash-other-payment")}
            />
          </div>
          <div className="dashboard-grid__cell dashboard-grid__cell--span-4">
            <div className="dashboard-widget">
              <div className="dashboard-widget__head">
                <h2 className="dashboard-widget__title">Book period</h2>
                <span className="dashboard-widget__meta">
                  {period?.status ?? "—"}
                </span>
              </div>
              {period?.period_start ? (
                <p className="dashboard-widget__sub">
                  {period.period_start} → {period.period_end}
                </p>
              ) : null}
              <PeriodCard />
            </div>
          </div>

          {/* Aging row: AR (6) + AP (6) */}
          <div className="dashboard-grid__cell dashboard-grid__cell--span-6">
            <AgingChart data={arAging} title="AR Aging" />
          </div>
          <div className="dashboard-grid__cell dashboard-grid__cell--span-6">
            <AgingChart data={apAging} title="AP Aging" />
          </div>

          {/* Bottom row: low stock (4) + PPN (4) + P&L breakdown placeholder (4) */}
          <div className="dashboard-grid__cell dashboard-grid__cell--span-4">
            <LowStockWidget
              items={lowStock}
              onOpenInventory={() => workbench.openList("inventory-items")}
            />
          </div>
          <div className="dashboard-grid__cell dashboard-grid__cell--span-4">
            <TaxSummaryWidget data={ppn} />
          </div>
          <div className="dashboard-grid__cell dashboard-grid__cell--span-4">
            <div className="dashboard-widget">
              <div className="dashboard-widget__head">
                <h2 className="dashboard-widget__title">P&L breakdown</h2>
                <span className="dashboard-widget__meta">MTD</span>
              </div>
              <ul className="tax-list">
                <li className="tax-list__row">
                  <span className="tax-list__label">Net profit</span>
                  <span
                    className={`tax-list__value${profitLoss !== null && profitLoss >= 0 ? " is-positive" : " is-negative"}`}
                  >
                    {profitLoss !== null ? formatIDR(profitLoss) : "—"}
                  </span>
                </li>
                <li className="tax-list__row">
                  <span className="tax-list__label">Cash position</span>
                  <span className="tax-list__value">
                    {cashBalance !== null ? formatIDR(cashBalance) : "—"}
                  </span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
