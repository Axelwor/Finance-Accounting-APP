import type { Transaction } from "../types";
import { formatIDR } from "../lib/format";

const KIND_LABEL: Record<Transaction["kind"], string> = {
  "money-in": "IN",
  "money-out": "OUT",
  transfer: "XFER",
};

const KIND_CLASS: Record<Transaction["kind"], string> = {
  "money-in": "kind-mark--money-in",
  "money-out": "kind-mark--money-out",
  transfer: "kind-mark--transfer",
};

const SIGN_CLASS: Record<Transaction["kind"], string> = {
  "money-in": "is-positive",
  "money-out": "is-negative",
  transfer: "is-muted",
};

interface TransactionRowProps {
  transaction: Transaction;
  /** Optional action element on the right side of the row (e.g. delete button). */
  action?: React.ReactNode;
}

/** One row in the ruled transaction table. */
export function TransactionRow({ transaction, action }: TransactionRowProps) {
  const positive = transaction.kind === "money-in";
  const neutral = transaction.kind === "transfer";
  const amount = positive
    ? `+${formatIDR(transaction.amount)}`
    : neutral
      ? formatIDR(transaction.amount)
      : `-${formatIDR(transaction.amount)}`;
  const signClass = SIGN_CLASS[transaction.kind];

  return (
    <div className="ledger-table__row">
      <span className="ledger-table__date">{transaction.date}</span>
      <div className="ledger-table__desc">
        <span className={`kind-mark ${KIND_CLASS[transaction.kind]}`}>{KIND_LABEL[transaction.kind]}</span>
        <div className="ledger-table__desc-text">
          <span className="ledger-table__desc-title">{transaction.description || KIND_LABEL[transaction.kind]}</span>
          <span className="ledger-table__desc-meta">
            {transaction.from && transaction.to
              ? `${transaction.from} to ${transaction.to}`
              : `Ruled ${transaction.date}`}
          </span>
        </div>
      </div>
      <span className="ledger-table__cat">{transaction.categoryName ?? "Uncategorized"}</span>
      <span className={`ledger-table__amount ${signClass}`}>{amount}</span>
      {action ?? <span aria-hidden="true" />}
    </div>
  );
}

/** Full transaction list (used on the Transactions page). */
export function TransactionList({
  transactions,
  onDelete,
}: {
  transactions: Transaction[];
  onDelete?: (id: string) => void;
}) {
  return (
    <div className="ledger-table">
      <div className="ledger-table__head">
        <span>Date</span>
        <span>Description</span>
        <span>Category</span>
        <span className="right">Amount</span>
        <span aria-hidden="true" />
      </div>
      {transactions.map((t) => (
        <TransactionRow
          key={t.id}
          transaction={t}
          action={
            onDelete ? (
              <button
                type="button"
                className="ledger-table__delete"
                aria-label={`Delete entry ${t.description || "without description"}`}
                onClick={() => onDelete(t.id)}
              >
                ×
              </button>
            ) : undefined
          }
        />
      ))}
    </div>
  );
}
