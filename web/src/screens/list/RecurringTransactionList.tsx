import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { RecurringTransactionListItem, CreateRecurringTransactionInput, RecurringFrequency } from "../../types";
import { fmtDateIDR, parseDateInput, parseAmountInput, fmtCurrencyIDR } from "../../lib/format";

type FrequencyFilter = "all" | RecurringFrequency;

interface FilterState {
  isActive: boolean | null;
  frequency: FrequencyFilter;
  nextDateFrom: string;
  nextDateTo: string;
}

export function RecurringTransactionList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<RecurringTransactionListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState<FilterState>({
    isActive: null,
    frequency: "all",
    nextDateFrom: "",
    nextDateTo: "",
  });

  const loadRecurring = async () => {
    try {
      const allItems = await api.listRecurring(filter.isActive === true);
      const filtered = applyFilters(allItems, filter);
      setItems(filtered);
    } catch (err) {
      setError("Failed to load recurring transactions.");
    } finally {
      setLoading(false);
    }
  };

  const applyFilters = (data: RecurringTransactionListItem[], f: FilterState): RecurringTransactionListItem[] => {
    return data.filter((item) => {
      if (f.isActive !== null && item.is_active !== f.isActive) return false;
      if (f.frequency !== "all" && item.frequency !== f.frequency) return false;
      if (f.nextDateFrom && item.next_date < f.nextDateFrom) return false;
      if (f.nextDateTo && item.next_date > f.nextDateTo) return false;
      return true;
    });
  };

  useEffect(() => {
    void loadRecurring();
  }, [filter]);

  if (loading) return <LoadingState label="Loading recurring transactions..." />;
  if (error) return <FormError message={error} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Recurring Transactions</span>
          <small>Templates for scheduled journal postings (rent, salary, subscriptions).</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <div className="filter-group">
            <label>Status</label>
            <select
              className="input input--sm"
              value={filter.isActive === null ? "all" : filter.isActive ? "active" : "inactive"}
              onChange={(e) =>
                setFilter({ ...filter, isActive: e.target.value === "all" ? null : e.target.value === "active" })
              }
            >
              <option value="all">All</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
          </div>
          <div className="filter-group">
            <label>Frequency</label>
            <select
              className="input input--sm"
              value={filter.frequency}
              onChange={(e) => setFilter({ ...filter, frequency: e.target.value as FrequencyFilter })}
            >
              <option value="all">All Frequencies</option>
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="quarterly">Quarterly</option>
              <option value="yearly">Yearly</option>
            </select>
          </div>
          <div className="filter-group">
            <label>Next Date From</label>
            <input
              type="date"
              className="input input--sm"
              value={filter.nextDateFrom}
              onChange={(e) => setFilter({ ...filter, nextDateFrom: e.target.value })}
            />
          </div>
          <div className="filter-group">
            <label>Next Date To</label>
            <input
              type="date"
              className="input input--sm"
              value={filter.nextDateTo}
              onChange={(e) => setFilter({ ...filter, nextDateTo: e.target.value })}
            />
          </div>
        </div>
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("recurring-transaction-entry")}
          >
            + New
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {items.length === 0 ? (
          <EmptyState
            title="No recurring transactions"
            message="Create templates for rent, salary, insurance, and other recurring journals."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("recurring-transaction-entry")}
              >
                New Recurring Transaction
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Frequency</span>
              <span>Next Date</span>
              <span>Last Posted</span>
              <span>Amount</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {items.map((item) => (
              <div key={item.id} className="ledger-table__row">
                <span className="ledger-table__no">{item.code}</span>
                <span className="ledger-table__cat">{item.name}</span>
                <span className="ledger-table__memo">{toFreqLabel(item.frequency)}</span>
                <span>{fmtDateIDR(item.next_date)}</span>
                <span>{item.last_posted_date ? fmtDateIDR(item.last_posted_date) : "—"}</span>
                <span>{fmtCurrencyIDR(item.amount_cents)}</span>
                <span>
                  <span className={`kind-mark ${item.is_active ? "is-positive" : "is-negative"}`}>
                    {item.is_active ? "Active" : "Inactive"}
                  </span>
                </span>
                <span>
                  <button
                    className="btn btn--sm"
                    onClick={() => workbench.openEntryExisting("recurring-transaction-entry", item.id, item.name)}
                  >
                    Edit
                  </button>
                  {item.is_active && (
                    <>
                      <button
                        className="btn btn--sm"
                        onClick={() => handlePostNow(item.id)}
                        disabled={postNowLoadingSet.has(item.id)}
                      >
                        Post Now
                      </button>
                      <button
                        className="btn btn--sm"
                        onClick={() => handleDeactivate(item.id)}
                        disabled={deactivateLoadingSet.has(item.id)}
                      >
                        Deactivate
                      </button>
                    </>
                  )}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} recurring transaction(s)</span>
      </div>
    </div>
  );
}

const postNowLoadingSet = new Set<number>();
const deactivateLoadingSet = new Set<number>();

async function handlePostNow(id: number) {
  postNowLoadingSet.add(id);
  try {
    const result = await api.postRecurring(id);
    alert(`Posted at ${result.posted_at}. Next date: ${result.next_date}. Journal intent: ${result.intent_type}`);
  } catch (err) {
    alert("Failed to post recurring transaction.");
  } finally {
    postNowLoadingSet.delete(id);
  }
}

async function handleDeactivate(id: number) {
  if (!confirm("Deactivate this recurring transaction?")) return;
  deactivateLoadingSet.add(id);
  try {
    await api.deactivateRecurring(id);
    const items = document.querySelectorAll(".ledger-table__row");
    const row = items[id - 1];
    if (row) row.parentElement?.removeChild(row as Node);
  } catch (err) {
    alert("Failed to deactivate.");
  } finally {
    deactivateLoadingSet.delete(id);
  }
}

function toFreqLabel(freq: RecurringFrequency): string {
  const map: Record<RecurringFrequency, string> = {
    daily: "Daily",
    weekly: "Weekly",
    monthly: "Monthly",
    quarterly: "Quarterly",
    yearly: "Yearly",
  };
  return map[freq];
}
