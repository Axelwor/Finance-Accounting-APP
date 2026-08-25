import { useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { JournalEntryListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

/**
 * Journal Entries list (Accountant Mode v1).
 *
 * Shows every posted journal entry (manual and otherwise) with its
 * number, date, description, intent, and totals. Date-range filterable.
 * Click a row to drill into the entry detail (lines).
 */
export function JournalEntryList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<JournalEntryListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listJournalEntries({
        from_date: fromDate || undefined,
        to_date: toDate || undefined,
      });
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load journal entries.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useTabRefresh(load);

  const totalDebit = items.reduce((sum, it) => sum + it.total_debit_cents, 0);
  const totalCredit = items.reduce((sum, it) => sum + it.total_credit_cents, 0);
  const openNew = () => workbench.openEntryDraft("journal-entry");

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Journal Entries</span>
          <small>Manual journals and posted entries across all modules.</small>
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
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Apply
          </Button>
        </div>
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={openNew}
          >
            + New Journal
          </Button>
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
          <LoadingState label="Loading journals..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState
            title="No journal entries"
            message="Post a manual journal entry to record adjustments, accruals, or corrections."
            action={
              <Button variant="filled" onClick={openNew}>
                New Journal Entry
              </Button>
            }
          />
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
                onClick={() => workbench.openEntryExisting("journal-entry", it.id, it.number, it.status)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    workbench.openEntryExisting("journal-entry", it.id, it.number, it.status);
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
          {totalDebit === totalCredit ? " · balanced" : " · unbalanced"}
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
