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

/** Ledger view with filters and local deletion. */
export function TransactionsScreen() {
  const { transactions, setTransactions } = useAppState();
  const [filter, setFilter] = useState<TransactionKind | "all">("all");

  useEffect(() => {
    document.title = "Ledger - Ledgerly";
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

  const total = useMemo(() => visible.reduce((acc, t) => acc + (t.kind === "money-in" ? t.amount : t.kind === "money-out" ? -t.amount : 0), 0), [visible]);

  return (
    <div className="list-page">
      <header className="page-head">
        <div className="page-head__left">
          <p className="page-head__meta"><strong>LEDGER</strong> &middot; {visible.length} of {transactions.length} entries</p>
          <h1 className="page-title">
            <span>Ledger</span>
            <span className="page-title__sep">/</span>
            <span className="page-title__sub">all entries, ruled order</span>
          </h1>
        </div>
        <div className="page-head__actions">
          <Link className="btn btn--primary" to="/record/money-in">
            New entry
          </Link>
        </div>
      </header>

      <div className="filter-strip" role="group" aria-label="Filter ledger">
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
        <span className="filter-strip__count">Net {total >= 0 ? "+" : "-"}{formatNet(total)}</span>
      </div>

      {visible.length === 0 ? (
        <EmptyState
          title="The book is empty"
          message="Money in, money out, or transfers will be ruled in here. The ledger reads the same as the reports."
          action={
            <Link className="btn btn--primary" to="/record/money-in">
              Record the first entry
            </Link>
          }
        />
      ) : (
        <TransactionList transactions={visible} onDelete={remove} />
      )}

      <div className="quick-actions">
        <Button to="/record/money-in" variant="secondary">Money in</Button>
        <Button to="/record/money-out" variant="secondary">Money out</Button>
        <Button to="/record/transfer" variant="secondary">Transfer</Button>
      </div>
    </div>
  );
}

function formatNet(n: number): string {
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(Math.abs(n));
}
