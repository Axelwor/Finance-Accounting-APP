import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatDate, formatIDR } from "../../lib/format";
import type { Customer, CustomerStatement } from "../../types";

/**
 * Customer Statement (Rekening Koran Pelanggan).
 *
 * Read-only AR statement for one customer over a date range: opening
 * balance, invoice/payment lines with a running balance, and period totals.
 * Defaults to the last 30 days.
 */
export function CustomerStatementScreen() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [fromDate, setFromDate] = useState(daysAgoISO(30));
  const [toDate, setToDate] = useState(todayLocalISO());
  const [statement, setStatement] = useState<CustomerStatement | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void api.listCustomers().then((list) => {
      if (cancelled) return;
      setCustomers(list);
      if (list.length > 0) setCustomerId((prev) => prev ?? list[0].id);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (customerId == null || !fromDate || !toDate) return;
    setLoading(true);
    setError(null);
    api
      .getCustomerStatement(customerId, fromDate, toDate)
      .then(setStatement)
      .catch((err) => {
        setStatement(null);
        setError(err instanceof Error ? err.message : "Failed to load customer statement.");
      })
      .finally(() => setLoading(false));
  }, [customerId, fromDate, toDate]);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Customer Statement</span>
          <small>AR statement: opening balance, invoices and payments with running balance.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Customer</span>
            <select
              className="filter-pill__input"
              value={customerId ?? ""}
              onChange={(e) => setCustomerId(Number(e.target.value) || null)}
            >
              {customers.length === 0 && <option value="">No customers</option>}
              {customers.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.code} — {c.name}
                </option>
              ))}
            </select>
          </label>
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
        </div>
      </div>

      {loading && <LoadingState label="Loading statement..." />}
      {!loading && error && <ErrorState message={error} />}
      {!loading && !error && customerId == null && (
        <EmptyState title="No customers" message="Create a customer first to view a statement." />
      )}
      {!loading && !error && statement && <StatementBody statement={statement} />}
    </div>
  );
}

function StatementBody({ statement }: { statement: CustomerStatement }) {
  return (
    <div className="listtab__body">
      <table className="table">
        <thead>
          <tr>
            <th>Date</th>
            <th>Type</th>
            <th>Reference</th>
            <th>Description</th>
            <th className="is-right">Debit</th>
            <th className="is-right">Credit</th>
            <th className="is-right">Balance</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td colSpan={6}>Opening balance</td>
            <td className="is-right">{formatIDR(statement.opening_balance_cents)}</td>
          </tr>
          {statement.lines.map((line, index) => (
            <tr key={`${line.date}-${index}`}>
              <td>{formatDate(line.date)}</td>
              <td>{line.type === "invoice" ? "Invoice" : "Payment"}</td>
              <td>{line.reference}</td>
              <td>{line.description || "—"}</td>
              <td className="is-right">{line.debit_cents ? formatIDR(line.debit_cents) : ""}</td>
              <td className="is-right">{line.credit_cents ? formatIDR(line.credit_cents) : ""}</td>
              <td className="is-right">{formatIDR(line.running_balance_cents)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr>
            <td colSpan={4}>
              Closing balance · Invoiced {formatIDR(statement.invoiced_cents)} · Paid{" "}
              {formatIDR(statement.paid_cents)}
            </td>
            <td colSpan={2} className="is-right">
              Closing
            </td>
            <td className="is-right">{formatIDR(statement.closing_balance_cents)}</td>
          </tr>
        </tfoot>
      </table>
      {statement.lines.length === 0 && (
        <EmptyState
          title="No transactions in range"
          message="No invoices or payments fall inside the selected date range."
        />
      )}
    </div>
  );
}

function todayLocalISO(): string {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

function daysAgoISO(days: number): string {
  const now = new Date();
  now.setDate(now.getDate() - days);
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}
