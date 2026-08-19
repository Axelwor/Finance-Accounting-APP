import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { CostCenter, CostCenterPnLResult } from "../../types";
import { Button } from "../../components/m3";

interface PnLRow {
  center: CostCenter;
  pnl: CostCenterPnLResult;
}

const CENTER_TYPE_LABEL: Record<string, string> = {
  COST: "Cost",
  PROFIT: "Profit",
  INVESTMENT: "Investment",
};

function isoDay(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/**
 * Cost Center P&L report (F-09). Fetches P&L per cost center for the selected
 * date range and shows revenue / expense / net with a totals row.
 */
export function CostCenterPnLList() {
  const workbench = useWorkbench();
  const [costCenters, setCostCenters] = useState<CostCenter[]>([]);
  const [rows, setRows] = useState<PnLRow[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const now = new Date();
  const [startDate, setStartDate] = useState(isoDay(new Date(now.getFullYear(), now.getMonth(), 1)));
  const [endDate, setEndDate] = useState(isoDay(now));

  useEffect(() => {
    void loadCostCenters();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadCostCenters = async () => {
    setInitialLoading(true);
    try {
      const all = await api.listCostCenters();
      setCostCenters(all);
    } finally {
      setInitialLoading(false);
    }
  };

  const handleLoad = async () => {
    if (!costCenters.length) return;
    setLoading(true);
    setError(null);
    try {
      const next: PnLRow[] = [];
      await Promise.all(
        costCenters.map(async (cc) => {
          try {
            const pnl = await api.getCostCenterPnL(cc.id, startDate, endDate);
            next.push({ center: cc, pnl });
          } catch {
            /* skip centers whose P&L call fails */
          }
        }),
      );
      next.sort((a, b) => a.center.code.localeCompare(b.center.code));
      setRows(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load P&L data.");
    } finally {
      setLoading(false);
    }
  };

  const totals = rows
    ? rows.reduce(
        (acc, r) => ({
          revenue: acc.revenue + r.pnl.revenue_cents,
          expense: acc.expense + r.pnl.expense_cents,
          net: acc.net + r.pnl.net_cents,
        }),
        { revenue: 0, expense: 0, net: 0 },
      )
    : null;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Cost Center P&L</span>
          <small>Laba/Rugi per pusat biaya dalam rentang tanggal</small>
        </div>
        <div className="listtab__toolbar">
          <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12 }}>
            From
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              disabled={loading}
              style={{ width: 140 }}
            />
          </label>
          <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12 }}>
            To
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              disabled={loading}
              style={{ width: 140 }}
            />
          </label>
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void handleLoad()}
            disabled={loading || !costCenters.length}
          >
            {loading ? "Loading..." : "Load"}
          </Button>
        </div>
      </div>

      <div className="listtab__body">
        {initialLoading ? (
          <LoadingState label="Loading cost centers..." />
        ) : !costCenters.length ? (
          <EmptyState
            title="No cost centers yet"
            message="Create cost centers first to see their P&L performance."
            action={<Button variant="filled" onClick={() => workbench.openEntryDraft("cost-center-entry")}>+ New Cost Center</Button>}
          />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void handleLoad()} />
        ) : loading ? (
          <LoadingState label="Computing P&L..." />
        ) : rows === null ? (
          <EmptyState
            title="Pick a date range"
            message="Select the period and press Load to compute P&L per cost center."
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title="No P&L data available"
            message="No posted journal lines were found for the selected period."
          />
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Code</th>
                <th>Cost Center</th>
                <th>Type</th>
                <th style={{ textAlign: "right" }}>Revenue</th>
                <th style={{ textAlign: "right" }}>Expense</th>
                <th style={{ textAlign: "right" }}>Net</th>
                <th style={{ textAlign: "right" }}>Lines</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(({ center, pnl }) => (
                <tr key={center.id}>
                  <td style={{ fontFamily: "var(--md-ref-typeface-plain)" }}>{center.code}</td>
                  <td>{center.name}</td>
                  <td>{CENTER_TYPE_LABEL[center.center_type] ?? center.center_type}</td>
                  <td style={{ textAlign: "right", fontFamily: "var(--md-ref-typeface-plain)" }}>
                    {formatIDR(pnl.revenue_cents)}
                  </td>
                  <td style={{ textAlign: "right", fontFamily: "var(--md-ref-typeface-plain)" }}>
                    {formatIDR(pnl.expense_cents)}
                  </td>
                  <td
                    style={{
                      textAlign: "right",
                      fontFamily: "var(--md-ref-typeface-plain)",
                      color: pnl.net_cents >= 0 ? "var(--md-sys-color-success)" : "var(--md-sys-color-error)",
                      fontWeight: 600,
                    }}
                  >
                    {formatIDR(pnl.net_cents)}
                  </td>
                  <td style={{ textAlign: "right" }}>{pnl.line_count}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <td colSpan={3} style={{ fontWeight: 700 }}>
                  Totals
                </td>
                <td style={{ textAlign: "right", fontFamily: "var(--md-ref-typeface-plain)", fontWeight: 700 }}>
                  {totals ? formatIDR(totals.revenue) : "-"}
                </td>
                <td style={{ textAlign: "right", fontFamily: "var(--md-ref-typeface-plain)", fontWeight: 700 }}>
                  {totals ? formatIDR(totals.expense) : "-"}
                </td>
                <td
                  style={{
                    textAlign: "right",
                    fontFamily: "var(--md-ref-typeface-plain)",
                    fontWeight: 700,
                    color: totals && totals.net >= 0 ? "var(--md-sys-color-success)" : "var(--md-sys-color-error)",
                  }}
                >
                  {totals ? formatIDR(totals.net) : "-"}
                </td>
                <td />
              </tr>
            </tfoot>
          </table>
        )}
      </div>

      <div className="listtab__footer">
        <span className="listtab__footer-count">
          {rows ? `${rows.length} Cost Center(s)` : `${costCenters.length} Cost Center(s)`} · {startDate} → {endDate}
        </span>
      </div>
    </div>
  );
}
