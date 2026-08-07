import { useEffect, useMemo, useState } from "react";
import { useAppState } from "../state";
import { EmptyState, LoadingState } from "../components/ui";
import type { TransactionKind } from "../types";

const FILTERS: { value: TransactionKind | "all"; label: string }[] = [
  { value: "all", label: "All" },
  { value: "money-in", label: "Money in" },
  { value: "money-out", label: "Money out" },
  { value: "transfer", label: "Transfers" },
];

const KIND_LABEL: Record<TransactionKind, string> = {
  "money-in": "RECEIPT",
  "money-out": "PAYMENT",
  transfer: "TRANSFER",
};

const KIND_TONE: Record<TransactionKind, string> = {
  "money-in": "kind-mark--money-in",
  "money-out": "kind-mark--money-out",
  transfer: "kind-mark--transfer",
};

/** Ledger view in the Accurate-style list layout. */
export function TransactionsScreen() {
  const { transactions, setTransactions } = useAppState();
  const [filter, setFilter] = useState<TransactionKind | "all">("all");
  const [search, setSearch] = useState("");

  useEffect(() => {
    document.title = "Ledger - Ledgerly";
  }, []);

  const visible = useMemo(() => {
    const sorted = [...transactions].sort(
      (a, b) => b.date.localeCompare(a.date) || b.createdAt.localeCompare(a.createdAt),
    );
    return filter === "all" ? sorted : sorted.filter((t) => t.kind === filter);
  }, [transactions, filter]);

  const searched = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return visible;
    return visible.filter((t) =>
      [t.description, t.categoryName, t.from, t.to].some((f) => f?.toLowerCase().includes(q)),
    );
  }, [visible, search]);

  const total = useMemo(
    () =>
      searched.reduce(
        (acc, t) => acc + (t.kind === "money-in" ? t.amount : t.kind === "money-out" ? -t.amount : 0),
        0,
      ),
    [searched],
  );

  const remove = (id: string) => {
    setTransactions(transactions.filter((t) => t.id !== id));
  };

  const filterLabel = (k: TransactionKind | "all") => FILTERS.find((f) => f.value === k)!.label;
  const activeFilterValue = filterLabel(filter);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Ledger</span>
          <small>All entries, ruled order.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <div className="filter-pill">
            <span className="filter-pill__label">Type</span>
            <span className="filter-pill__value">{activeFilterValue}</span>
            <span className="filter-pill__caret">▾</span>
          </div>
          <button type="button" className="filter-pill__toggle" aria-label="More filters">
            <span aria-hidden="true">▾</span>
          </button>
          <div className="filter-strip filter-strip--inline" role="group" aria-label="Quick filter">
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
        </div>
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => setFilter("all")}
            disabled={transactions.length === 0}
          >
            + Reset
          </button>
          <input
            type="search"
            className="input listtab__search"
            placeholder="Type and [Enter]"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <span className="listtab__count">{searched.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {transactions.length === 0 ? (
          <EmptyState
            title="The book is empty"
            message="Money in, money out, or transfers will be ruled in here. The ledger reads the same as the reports."
            action={
              <a className="btn btn--primary" href="/record/money-in">
                Record the first entry
              </a>
            }
          />
        ) : searched.length === 0 ? (
          <LoadingState label="No matches for the current filter." />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Account</span>
              <span>Reference</span>
              <span>Description</span>
              <span className="right">Amount</span>
            </div>
            {searched.map((t) => {
              const positive = t.kind === "money-in";
              const neutral = t.kind === "transfer";
              const amount = positive
                ? `+${formatNet(t.amount)}`
                : neutral
                  ? formatNet(t.amount)
                  : `-${formatNet(t.amount)}`;
              const tone = positive ? "is-positive" : neutral ? "is-muted" : "is-negative";
              const accountLine =
                t.from && t.to
                  ? `${t.from} → ${t.to}`
                  : t.categoryName ?? "Uncategorized";
              return (
                <div key={t.id} className="ledger-table__row">
                  <span className="ledger-table__no">—</span>
                  <span className="ledger-table__date">{t.date}</span>
                  <span className="ledger-table__cat">{accountLine}</span>
                  <span>
                    <span className={`kind-mark ${KIND_TONE[t.kind]}`}>{KIND_LABEL[t.kind]}</span>
                  </span>
                  <span className="ledger-table__desc">
                    <span>{t.description || KIND_LABEL[t.kind]}</span>
                  </span>
                  <span className={`ledger-table__amount right ${tone}`}>{amount}</span>
                  <button
                    type="button"
                    className="ledger-table__delete"
                    aria-label={`Delete entry ${t.description || "without description"}`}
                    onClick={() => remove(t.id)}
                  >
                    ×
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Net{" "}
          <strong
            className={total >= 0 ? "is-positive" : "is-negative"}
            style={{ color: total >= 0 ? "var(--pos)" : "var(--neg)" }}
          >
            {total >= 0 ? "+" : "−"}
            {formatNet(Math.abs(total))}
          </strong>
        </span>
        <span className="listtab__footer-count">
          {searched.length} of {transactions.length}
        </span>
      </div>
    </div>
  );
}

function formatNet(n: number): string {
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(n);
}
