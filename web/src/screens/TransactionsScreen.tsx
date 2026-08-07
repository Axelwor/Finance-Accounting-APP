import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useAppState } from "../state";
import { Button, EmptyState } from "../components/ui";
import { TransactionList } from "../components/transactions";
import type { TransactionKind } from "../types";

const FILTERS: { value: TransactionKind | "all"; label: string }[] = [
  { value: "all", label: "All" },
  { value: "money-in", label: "Money in" },
  { value: "money-out", label: "Money out" },
  { value: "transfer", label: "Transfers" },
];

/** Full transaction record list with filters and local deletion. */
export function TransactionsScreen() {
  const { transactions, setTransactions } = useAppState();
  const [filter, setFilter] = useState<TransactionKind | "all">("all");

  useEffect(() => {
    document.title = "Transactions - Ledgerly";
  }, []);

  const visible = useMemo(() => {
    const sorted = [...transactions].sort(
      (a, b) => b.date.localeCompare(a.date) || b.createdAt.localeCompare(a.createdAt),
    );
    return filter === "all" ? sorted : sorted.filter((t) => t.kind === filter);
  }, [transactions, filter]);

  const remove = (id: string) => {
    setTransactions(transactions.filter((t) => t.id !== id));
  };

  return (
    <div className="list-page">
      <header className="page-head">
        <div>
          <h1 className="page-title">Transactions</h1>
          <p className="page-sub">All money in, money out, and transfers.</p>
        </div>
      </header>

      <div className="filter-row" role="group" aria-label="Filter records">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            className={`filter-chip${filter === f.value ? " is-active" : ""}`}
            aria-pressed={filter === f.value}
            onClick={() => setFilter(f.value)}
          >
            {f.label}
          </button>
        ))}
      </div>

      <div className="list-card">
        {visible.length === 0 ? (
          <EmptyState
            title={filter === "all" ? "No records yet" : "No records for this filter"}
            message="Money in, money out, or transfers will show up here."
            action={
              <Link className="btn btn--primary" to="/record/money-in">
                Record transaction
              </Link>
            }
          />
        ) : (
          <>
            <TransactionList transactions={visible} onDelete={remove} />
            <p className="list-card__footer">Total {visible.length} records</p>
          </>
        )}
      </div>

      <div className="quick-actions">
        <Button to="/record/money-in">Money in</Button>
        <Button to="/record/money-out" variant="secondary">
          Money out
        </Button>
        <Button to="/record/transfer" variant="secondary">
          Transfer
        </Button>
      </div>
    </div>
  );
}
