import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { BudgetListItem, BudgetVsActualResult } from "../../types";
import { Button } from "../../components/m3";

const MONTH_LABELS = [
  "Jan", "Feb", "Mar", "Apr", "May", "Jun",
  "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

/**
 * Budget vs Actual report (US-093). The user picks a budget from the
 * dropdown; the table compares planned (budget_lines) against actual posted
 * journal movements per account/month, with variance = actual − budget.
 *
 * Follows the PPNReconciliation screen pattern: standalone list tab with its
 * own picker, no entry form.
 */
export function BudgetVsActual() {
  const [budgets, setBudgets] = useState<BudgetListItem[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [result, setResult] = useState<BudgetVsActualResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadBudgets = async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await api.listBudgets();
      setBudgets(items);
      if (items.length > 0 && selectedId === null) {
        setSelectedId(items[0].id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load budgets.");
    } finally {
      setLoading(false);
    }
  };

  const loadReport = async (id: number) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.getBudgetVsActual(id);
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load budget vs actual.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadBudgets();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (selectedId !== null) void loadReport(selectedId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId]);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Budget vs Actual</span>
          <small>Anggaran vs realisasi per akun / bulan</small>
        </div>
        <div className="listtab__toolbar">
          <select
            className="field__input"
            value={selectedId ?? ""}
            onChange={(e) => setSelectedId(Number(e.target.value))}
            style={{ minWidth: 240 }}
            disabled={budgets.length === 0}
          >
            {budgets.length === 0 ? (
              <option value="">No budgets available</option>
            ) : (
              budgets.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name} · {b.fiscal_year}
                </option>
              ))
            )}
          </select>
          <Button
            variant="outlined"
            size="sm"
            onClick={() => (selectedId !== null ? void loadReport(selectedId) : void loadBudgets())}
          >
            Reload
          </Button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Computing..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => (selectedId !== null ? void loadReport(selectedId) : void loadBudgets())} />
        ) : budgets.length === 0 ? (
          <EmptyState
            title="No budgets"
            message="Create a budget first, then come back to compare it against actual postings."
          />
        ) : !result ? (
          <EmptyState title="No data" message="Select a budget to view the variance report." />
        ) : result.rows.length === 0 ? (
          <EmptyState
            title="No budget lines"
            message="This budget has no lines. Add monthly amounts per account in the budget form."
          />
        ) : (
          <>
            <div className="kpi-list" style={{ marginBottom: 16 }}>
              <BudgetStat
                label="Total Budget"
                value={formatIDR(result.total_budget_cents)}
                tone="acc"
                note={`FY ${result.fiscal_year}`}
              />
              <BudgetStat
                label="Total Actual"
                value={formatIDR(result.total_actual_cents)}
                tone={result.total_actual_cents >= 0 ? "pos" : "neg"}
                note="posted"
              />
              <BudgetStat
                label="Total Variance"
                value={formatIDR(Math.abs(result.total_variance_cents))}
                tone={result.total_variance_cents <= 0 ? "pos" : "neg"}
                suffix={result.total_variance_cents <= 0 ? "UNDER" : "OVER"}
              />
            </div>

            <div style={{ overflowX: "auto" }}>
              <table className="data-table" style={{ width: "100%", borderCollapse: "collapse" }}>
                <thead>
                  <tr>
                    <th style={{ textAlign: "left", padding: "8px 12px" }}>Account</th>
                    <th style={{ textAlign: "left", padding: "8px 12px" }}>Month</th>
                    <th style={{ textAlign: "right", padding: "8px 12px" }}>Budget</th>
                    <th style={{ textAlign: "right", padding: "8px 12px" }}>Actual</th>
                    <th style={{ textAlign: "right", padding: "8px 12px" }}>Variance</th>
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row, idx) => {
                    const variance = row.variance_cents;
                    const over = variance > 0;
                    return (
                      <tr key={`${row.account_id}-${row.month}-${idx}`} style={{ borderBottom: "1px solid var(--md-sys-color-outline-variant)" }}>
                        <td style={{ padding: "8px 12px" }}>
                          <span style={{ fontFamily: "var(--md-ref-typeface-plain)", marginRight: 8 }}>{row.account_code}</span>
                          {row.account_name}
                        </td>
                        <td style={{ padding: "8px 12px" }}>{MONTH_LABELS[row.month - 1] ?? row.month}</td>
                        <td style={{ padding: "8px 12px", textAlign: "right" }}>{formatIDR(row.budget_cents)}</td>
                        <td style={{ padding: "8px 12px", textAlign: "right" }}>{formatIDR(row.actual_cents)}</td>
                        <td
                          style={{
                            padding: "8px 12px",
                            textAlign: "right",
                            color: over ? "var(--md-sys-color-error)" : "var(--md-sys-color-success)",
                            fontWeight: 600,
                          }}
                        >
                          {formatIDR(Math.abs(variance))}
                          <span style={{ fontSize: 10, marginLeft: 4 }}>{over ? "OVER" : "UNDER"}</span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
                <tfoot>
                  <tr style={{ borderTop: "2px solid var(--md-sys-color-outline-variant)" }}>
                    <td colSpan={2} style={{ padding: "8px 12px", fontWeight: 600 }}>
                      Total
                    </td>
                    <td style={{ padding: "8px 12px", textAlign: "right", fontWeight: 600 }}>
                      {formatIDR(result.total_budget_cents)}
                    </td>
                    <td style={{ padding: "8px 12px", textAlign: "right", fontWeight: 600 }}>
                      {formatIDR(result.total_actual_cents)}
                    </td>
                    <td style={{ padding: "8px 12px", textAlign: "right", fontWeight: 600 }}>
                      {formatIDR(Math.abs(result.total_variance_cents))}
                    </td>
                  </tr>
                </tfoot>
              </table>
            </div>
          </>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{result?.rows.length ?? 0} Line(s)</span>
      </div>
    </div>
  );
}

function BudgetStat({
  label,
  value,
  tone,
  note,
  suffix,
}: {
  label: string;
  value: string;
  tone: "pos" | "neg" | "acc";
  note?: string;
  suffix?: string;
}) {
  return (
    <div
      className="kpi-list__row"
      style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}
    >
      <div className="kpi-list__label">
        <span className="kpi-list__label-title">{label}</span>
        {note ? <span className="kpi-list__label-note">{note}</span> : null}
        {suffix ? <span className="kpi-list__label-note">{suffix}</span> : null}
      </div>
      <span className={`kpi-list__value is-${tone}`}>{value}</span>
      <span className={`kpi-list__dot is-${tone === "pos" ? "pos" : tone === "neg" ? "neg" : "warn"}`} aria-hidden="true" />
    </div>
  );
}
