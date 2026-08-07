import type { Transaction } from "../types";
import { formatIDR, formatRelativeDate } from "../lib/format";

const KIND_LABEL: Record<Transaction["kind"], string> = {
  "money-in": "Money in",
  "money-out": "Money out",
  transfer: "Transfer",
};

interface TransactionRowProps {
  transaction: Transaction;
  /** Optional action element on the right side of the row (e.g. delete button). */
  action?: React.ReactNode;
}

/** One row in the transaction list. */
export function TransactionRow({ transaction, action }: TransactionRowProps) {
  const positive = transaction.kind === "money-in";
  const neutral = transaction.kind === "transfer";

  return (
    <li className="transaction-row">
      <span className={`transaction-row__badge transaction-row__badge--${transaction.kind}`}>
        {transaction.kind === "money-in"
          ? "In"
          : transaction.kind === "money-out"
            ? "Out"
            : "Transfer"}
      </span>
      <div className="transaction-row__body">
        <p className="transaction-row__description">
          {transaction.description || KIND_LABEL[transaction.kind]}
        </p>
        <p className="transaction-row__meta">
          {transaction.categoryName ?? KIND_LABEL[transaction.kind]}
          <span aria-hidden="true"> · </span>
          {formatRelativeDate(transaction.date)}
        </p>
      </div>
      <div className="transaction-row__amount">
        <p className={`transaction-row__nominal${positive ? " is-positive" : neutral ? " is-neutral" : ""}`}>
          {positive ? "+" : neutral ? "" : "-"}
          {formatIDR(transaction.amount)}
        </p>
        {transaction.from && transaction.to ? (
          <p className="transaction-row__meta">
            {transaction.from} to {transaction.to}
          </p>
        ) : null}
      </div>
      {action}
    </li>
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
    <ul className="transaction-list">
      {transactions.map((t) => (
        <TransactionRow
          key={t.id}
          transaction={t}
          action={
            onDelete ? (
              <button
                type="button"
                className="transaction-row__delete"
                aria-label={`Delete record ${t.description || "without description"}`}
                onClick={() => onDelete(t.id)}
              >
                Delete
              </button>
            ) : undefined
          }
        />
      ))}
    </ul>
  );
}
