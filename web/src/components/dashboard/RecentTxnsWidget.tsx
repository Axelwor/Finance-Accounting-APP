import { formatIDR } from "../../lib/format";
import type { JournalEntryListItem } from "../../types";

/** Recent transactions widget — 8 rows from GET /journal-entries. */
export function RecentTxnsWidget({
  transactions,
  onOpenLedger,
}: {
  transactions: JournalEntryListItem[];
  onOpenLedger?: () => void;
}) {
  return (
    <div className="dashboard-widget">
      <div className="dashboard-widget__head">
        <h2 className="dashboard-widget__title">Latest entries</h2>
        <span className="dashboard-widget__meta">
          {transactions.length} {transactions.length === 1 ? "entry" : "entries"}
        </span>
      </div>
      {transactions.length === 0 ? (
        <div className="empty-state empty-state--compact">
          <p className="empty-state__message">No posted entries yet.</p>
        </div>
      ) : (
        <div className="ledger-table ledger-table--compact">
          <div className="ledger-table__head">
            <span>Date</span>
            <span>Description</span>
            <span>Type</span>
            <span className="right">Amount</span>
          </div>
          {transactions.map((t) => (
            <div key={t.id} className="ledger-table__row ledger-table__row--compact">
              <span className="ledger-table__date">{t.entry_date}</span>
              <span className="ledger-table__desc">{t.description || t.number}</span>
              <span className="ledger-table__cat">{t.intent_type || "—"}</span>
              <span className="ledger-table__amount right">
                {t.total_debit_cents > 0 ? formatIDR(t.total_debit_cents) : "—"}
              </span>
            </div>
          ))}
        </div>
      )}
      {onOpenLedger && transactions.length > 0 ? (
        <button type="button" className="dashboard-widget__action" onClick={onOpenLedger}>
          View ledger
        </button>
      ) : null}
    </div>
  );
}
