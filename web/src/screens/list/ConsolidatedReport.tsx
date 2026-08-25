import { useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type {
  ConsolidatedTrialBalanceResult,
  ConsolidatedProfitLossResult,
} from "../../types";
import { Button } from "../../components/m3";

type ReportKind = "trial-balance" | "profit-loss";

/**
 * Consolidated report screen for PSAK 65 multi-entity consolidation.
 * Shows either the consolidated trial balance or consolidated P&L,
 * with inter-company elimination applied.
 */
export function ConsolidatedReport({ initialKind = "trial-balance" }: { initialKind?: ReportKind }) {
  const [kind, setKind] = useState<ReportKind>(initialKind);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [tb, setTb] = useState<ConsolidatedTrialBalanceResult | null>(null);
  const [pl, setPl] = useState<ConsolidatedProfitLossResult | null>(null);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      if (kind === "trial-balance") {
        setTb(await api.getConsolidatedTrialBalance());
      } else {
        setPl(await api.getConsolidatedProfitLoss());
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load consolidated report.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, [kind]);
  useTabRefresh(load);

  const title = kind === "trial-balance" ? "Consolidated Trial Balance" : "Consolidated Profit & Loss";
  const desc = "PSAK 65 — aggregated across parent + child entities with inter-company elimination.";

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{title}</span>
          <small>{desc}</small>
        </div>
        <div className="listtab__toolbar">
          <div className="listtab__filters">
            <Button
              variant={kind === "trial-balance" ? "filled" : "outlined"}
              size="sm"
              onClick={() => setKind("trial-balance")}
            >
              Trial Balance
            </Button>
            <Button
              variant={kind === "profit-loss" ? "filled" : "outlined"}
              size="sm"
              onClick={() => setKind("profit-loss")}
            >
              Profit & Loss
            </Button>
          </div>
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Reload
          </Button>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Computing consolidated report..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : kind === "trial-balance" && tb ? (
          tb.rows.length === 0 ? (
            <EmptyState title="No data" message="No consolidated balances found. Set up entity hierarchy first." />
          ) : (
            <>
              <div className="ledger-table">
                <div className="ledger-table__row ledger-table__row--header">
                  <span>Code</span>
                  <span>Account</span>
                  <span>Group</span>
                  <span className="right">Debit</span>
                  <span className="right">Credit</span>
                </div>
                {tb.rows.map((r) => (
                  <div key={r.account_id} className="ledger-table__row">
                    <span className="ledger-table__num">{r.account_code}</span>
                    <span className="ledger-table__memo">{r.account_name}</span>
                    <span className="ledger-table__date">{r.report_group}</span>
                    <span className="ledger-table__amount right">{r.debit_cents > 0 ? formatIDR(r.debit_cents) : "—"}</span>
                    <span className="ledger-table__amount right">{r.credit_cents > 0 ? formatIDR(r.credit_cents) : "—"}</span>
                  </div>
                ))}
              </div>
              <div className="listtab__footer">
                <span>Total Debit: <strong>{formatIDR(tb.total_debit_cents)}</strong></span>
                <span>Total Credit: <strong>{formatIDR(tb.total_credit_cents)}</strong></span>
                <span>Eliminated: <strong>{formatIDR(tb.elimination_cents)}</strong></span>
                <span>Balanced: <strong className={tb.balanced ? "is-positive" : "is-negative"}>{tb.balanced ? "Yes" : "No"}</strong></span>
              </div>
            </>
          )
        ) : pl ? (
          <div className="kpi-list">
            <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
              <div className="kpi-list__label"><span className="kpi-list__label-title">Revenue</span></div>
              <span className="kpi-list__value is-pos">{formatIDR(pl.revenue_cents)}</span>
            </div>
            <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
              <div className="kpi-list__label"><span className="kpi-list__label-title">Expenses</span></div>
              <span className="kpi-list__value is-neg">{formatIDR(pl.expense_cents)}</span>
            </div>
            <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
              <div className="kpi-list__label"><span className="kpi-list__label-title">Net Profit</span></div>
              <span className={`kpi-list__value ${pl.profit_cents >= 0 ? "is-pos" : "is-neg"}`}>{formatIDR(pl.profit_cents)}</span>
            </div>
            {pl.elimination_cents > 0 && (
              <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
                <div className="kpi-list__label"><span className="kpi-list__label-title">Inter-company Eliminated</span></div>
                <span className="kpi-list__value">{formatIDR(pl.elimination_cents)}</span>
              </div>
            )}
          </div>
        ) : (
          <EmptyState title="No data" message="No consolidated data found." />
        )}
      </div>
    </div>
  );
}
