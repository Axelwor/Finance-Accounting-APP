import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { InvoiceListItem } from "../../types";

const INV_STATUS_TONE: Record<string, string> = {
  DRAFT: "is-muted",
  ISSUED: "",
  PARTIALLY_PAID: "",
  PAID: "is-positive",
  VOID: "is-negative",
};

export function InvoiceList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<InvoiceListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | InvoiceListItem["status"]>("ALL");

  const load = async (filter: "ALL" | InvoiceListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listInvoices(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalReceivable = useMemo(
    () => items.filter((i) => i.status !== "VOID").reduce((acc, it) => acc + it.receivable_cents, 0),
    [items],
  );
  const openEntry = (item: InvoiceListItem) =>
    workbench.openEntryExisting("sales-invoice", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Sales Invoices</span>
          <small>Invoices issued to customers (INV). Posts revenue + DP realization.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <div className="filter-pill">
            <span className="filter-pill__label">Status</span>
            <span className="filter-pill__value">{status === "ALL" ? "All" : status}</span>
            <span className="filter-pill__caret">▾</span>
          </div>
        </div>
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("sales-invoice")}
          >
            + New Invoice
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading invoices..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No invoices yet"
            message="Issue an invoice to recognize revenue and accounts receivable. When the linked SO has down payments, the invoice automatically realizes them against the receivable."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("sales-invoice")}
              >
                New Invoice
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Customer</span>
              <span>Due</span>
              <span>Status</span>
              <span className="right">DP Applied</span>
              <span className="right">Receivable</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => openEntry(it)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openEntry(it);
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.invoice_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span className="ledger-table__memo">{it.due_date ?? "—"}</span>
                <span>
                  <span className={`kind-mark ${INV_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span>
                </span>
                <span className="ledger-table__amount right">{it.dp_applied_cents > 0 ? formatIDR(it.dp_applied_cents) : "—"}</span>
                <span className="ledger-table__amount right">{formatIDR(it.receivable_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Outstanding receivable <strong>{formatIDR(totalReceivable)}</strong>
        </span>
        <span className="listtab__footer-count">{items.length} invoice(s)</span>
      </div>
    </div>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
