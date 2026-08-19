import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { JournalRegisterItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

/**
 * Journal Register — Accountant Mode v1.
 *
 * Lists all posted journal entries (any intent type) with their totals,
 * filterable by date range and intent_type. Click a row to drill into the
 * entry detail (lines).
 */
export function JournalRegister() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<JournalRegisterItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [intentType, setIntentType] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getJournalRegister({
        from_date: fromDate || undefined,
        to_date: toDate || undefined,
        intent_type: intentType || undefined,
      });
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the journal register.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalDebit = items.reduce((sum, it) => sum + it.total_debit_cents, 0);
  const totalCredit = items.reduce((sum, it) => sum + it.total_credit_cents, 0);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Journal Register</span>
          <small>All posted journals across modules, filterable by date and intent.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
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
          <label className="filter-pill">
            <span className="filter-pill__label">Intent</span>
            <select
              className="filter-pill__input"
              value={intentType}
              onChange={(e) => setIntentType(e.target.value)}
            >
              <option value="">All</option>
              <option value="MANUAL_JOURNAL">Manual</option>
              <option value="CASH_IN">Cash In</option>
              <option value="CASH_OUT">Cash Out</option>
              <option value="TRANSFER">Transfer</option>
              <option value="OPENING_BALANCE">Opening</option>
              <option value="REVERSAL">Reversal</option>
            </select>
          </label>
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Apply
          </Button>
        </div>
        <div className="listtab__actions">
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
          >
            <ReloadIcon />
          </IconButton>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading register..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState title="No journal entries" message="No posted journals match the selected filters." />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Description</span>
              <span>Intent</span>
              <span className="right">Debit</span>
              <span className="right">Credit</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => workbench.openEntryExisting("journal-entry", it.id, it.number, "POSTED")}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    workbench.openEntryExisting("journal-entry", it.id, it.number, "POSTED");
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.entry_date}</span>
                <span className="ledger-table__cat">{it.description || "—"}</span>
                <span>
                  <span className="kind-mark is-muted">{it.intent_type}</span>
                </span>
                <span className="ledger-table__amount right">{formatIDR(it.total_debit_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_credit_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Totals <strong>{formatIDR(totalDebit)}</strong> / <strong>{formatIDR(totalCredit)}</strong>
        </span>
        <span className="listtab__footer-count">{items.length} entr(ies)</span>
      </div>
    </div>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
