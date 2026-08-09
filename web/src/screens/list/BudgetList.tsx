import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { BudgetListItem } from "../../types";

const STATUS_TONE: Record<string, string> = {
  DRAFT: "var(--warn)",
  APPROVED: "var(--pos)",
  CLOSED: "var(--neg)",
};

/**
 * Budget list (US-093). Each row is a budget for a fiscal year, optionally
 * scoped to a dimension. The "vs Actual" button opens the Budget vs Actual
 * report screen (which has its own budget picker). "New Budget" opens the
 * budget entry form.
 */
export function BudgetList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<BudgetListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listBudgets();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load budgets.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Budgets</span>
          <small>Anggaran per dimensi &amp; tahun fiskal</small>
        </div>
        <div className="listtab__toolbar">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("budget-entry")}
          >
            + New Budget
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading budgets..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState
            title="No budgets yet"
            message="Create a budget with monthly lines per account to compare against actual postings."
          />
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Fiscal Year</th>
                <th>Status</th>
                <th>Lines</th>
                <th style={{ textAlign: "right" }}>Total Budget</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {items.map((b) => (
                <tr key={b.id}>
                  <td style={{ fontWeight: 600 }}>{b.name}</td>
                  <td>{b.fiscal_year}</td>
                  <td>
                    <span style={{ color: STATUS_TONE[b.status] ?? "var(--text)", fontWeight: 600, fontSize: "12px" }}>
                      {b.status}
                    </span>
                  </td>
                  <td>{b.line_count}</td>
                  <td style={{ textAlign: "right", fontFamily: "var(--font-mono)" }}>{formatIDR(b.total_cents)}</td>
                  <td>
                    <button
                      type="button"
                      className="btn btn--secondary btn--sm"
                      onClick={() => workbench.openList("budget-vs-actual")}
                    >
                      vs Actual
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Budget(s)</span>
      </div>
    </div>
  );
}
