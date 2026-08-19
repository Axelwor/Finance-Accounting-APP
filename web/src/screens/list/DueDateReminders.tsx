import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { DueDateReminder } from "../../types";
import { IconButton } from "../../components/m3";

/**
 * Due Date Reminders (Pengingat Jatuh Tempo).
 *
 * Read-only two-panel view: customer receivables due (left) and supplier
 * payables due (right). Each row shows the party name, invoice number, due
 * date, amount, and how many days the invoice is overdue (or until due).
 * The horizon (days ahead) is configurable via the toolbar.
 */
export function DueDateReminders() {
  const [days, setDays] = useState(7);
  const [items, setItems] = useState<DueDateReminder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listDueDateReminders(days);
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load due-date reminders.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days]);

  const receivables = items.filter((it) => it.direction === "customer");
  const payables = items.filter((it) => it.direction === "supplier");
  const totalReceivable = receivables.reduce((s, it) => s + it.amount_cents, 0);
  const totalPayable = payables.reduce((s, it) => s + it.amount_cents, 0);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Due Date Reminders</span>
          <small>Invoices (customer &amp; supplier) due within the horizon and not yet paid or voided.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Days ahead</span>
            <select
              className="filter-pill__input"
              value={days}
              onChange={(e) => setDays(Number(e.target.value) || 7)}
            >
              <option value={7}>7 days</option>
              <option value={14}>14 days</option>
              <option value={30}>30 days</option>
              <option value={60}>60 days</option>
            </select>
          </label>
        </div>
        <div className="listtab__actions">
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
          <LoadingState label="Loading reminders..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState
            title="Nothing due"
            message={`No customer or supplier invoices fall due within the next ${days} day(s).`}
          />
        ) : (
          <div className="reminders-grid">
            <ReminderPanel
              title="Receivables (Customer)"
              tone="pos"
              items={receivables}
              total={totalReceivable}
            />
            <ReminderPanel
              title="Payables (Supplier)"
              tone="neg"
              items={payables}
              total={totalPayable}
            />
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} due invoice(s)</span>
      </div>
    </div>
  );
}

function ReminderPanel({
  title,
  tone,
  items,
  total,
}: {
  title: string;
  tone: "pos" | "neg";
  items: DueDateReminder[];
  total: number;
}) {
  return (
    <section className="reminders-panel">
      <header className={`reminders-panel__head is-${tone}`}>
        <span>{title}</span>
        <span className="reminders-panel__total">{formatIDR(total)}</span>
      </header>
      <div className="reminders-panel__body">
        {items.length === 0 ? (
          <p className="reminders-panel__empty">No invoices due in this window.</p>
        ) : (
          <div className="reminders-rows">
            <div className="reminders-rows__head">
              <span>Party</span>
              <span>Invoice #</span>
              <span>Due</span>
              <span className="right">Amount</span>
              <span className="right">Days</span>
            </div>
            {items.map((it) => (
              <div className="reminders-rows__row" key={`${it.direction}-${it.id}`}>
                <span className="reminders-rows__party">{it.party_name}</span>
                <span className="reminders-rows__number">{it.number}</span>
                <span className="reminders-rows__due">{formatDue(it.due_date)}</span>
                <span className="reminders-rows__amount right">{formatIDR(it.amount_cents)}</span>
                <span className={`reminders-rows__days right ${daysTone(it.days_overdue)}`}>
                  {formatDays(it.days_overdue)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function formatDays(daysOverdue: number): string {
  if (daysOverdue > 0) return `${daysOverdue}d overdue`;
  if (daysOverdue === 0) return "Due today";
  return `in ${Math.abs(daysOverdue)}d`;
}

function daysTone(daysOverdue: number): string {
  if (daysOverdue > 0) return "is-neg";
  if (daysOverdue <= -7) return "is-muted";
  return "is-warn";
}

function formatDue(date: string): string {
  const datePart = (date ?? "").slice(0, 10);
  const [y, m, d] = datePart.split("-").map(Number);
  if (!y || !m || !d) return date;
  return `${String(d).padStart(2, "0")}/${String(m).padStart(2, "0")}/${y}`;
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
