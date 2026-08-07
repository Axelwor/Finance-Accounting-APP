import type { Transaction } from "../types";
import { formatIDR, formatDate } from "../lib/format";

const KIND_LABEL: Record<Transaction["kind"], string> = {
  "money-in": "In",
  "money-out": "Out",
  transfer: "Xfer",
};

const KIND_CLASS: Record<Transaction["kind"], string> = {
  "money-in": "tx-kind-mark--money-in",
  "money-out": "tx-kind-mark--money-out",
  transfer: "tx-kind-mark--transfer",
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

/** One row in the ruled transaction list. */
export function TransactionRow({ transaction, action }: TransactionRowProps) {
  const positive = transaction.kind === "money-in";
  const neutral = transaction.kind === "transfer";
  const amount = positive
    ? `+ ${formatIDR(transaction.amount)}`
    : neutral
      ? formatIDR(transaction.amount)
      : `- ${formatIDR(transaction.amount)}`;
  const signClass = SIGN_CLASS[transaction.kind];

  return (
    <li className="tx-table__row">
      <span className="tx-table__date">{formatDate(transaction.date)}</span>
      <div className="tx-table__desc">
        <span className="tx-table__desc-title">
          <span className={`tx-kind-mark ${KIND_CLASS[transaction.kind]}`}>{KIND_LABEL[transaction.kind]}</span>
          {transaction.description || KIND_LABEL[transaction.kind]}
        </span>
        <span className="tx-table__desc-meta">
          {transaction.from && transaction.to ? `${transaction.from} to ${transaction.to}` : formatDate(transaction.date)}
        </span>
      </div>
      <span className="tx-table__cat">{transaction.categoryName ?? "Uncategorized"}</span>
      <span className={`tx-table__amount ${signClass}`}>{amount}</span>
      {action ?? <span aria-hidden="true" />}
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
    <ul className="tx-table">
      <li className="tx-table__head">
        <span>Date</span>
        <span>Description</span>
        <span>Category</span>
        <span style={{ textAlign: "right" }}>Amount</span>
        <span aria-hidden="true" />
      </li>
      {transactions.map((t) => (
        <TransactionRow
          key={t.id}
          transaction={t}
          action={
            onDelete ? (
              <button
                type="button"
                className="tx-table__delete"
                aria-label={`Delete entry ${t.description || "without description"}`}
                onClick={() => onDelete(t.id)}
              >
                ×
              </button>
            ) : undefined
          }
        />
      ))}
    </ul>
  );
}
