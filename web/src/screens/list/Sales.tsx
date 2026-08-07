import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { MockList, type MockListColumn } from "./MockList";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { makeSalesInvoices, makeSalesReceipts, type SalesInvoice, type SalesReceipt } from "../../lib/mockData";
import type { QuotationListItem } from "../../types";

/* ---------------------- Sales Invoice ---------------------- */

function StatusBadge({ status }: { status: SalesInvoice["status"] }) {
  const cls =
    status === "POSTED"
      ? "is-positive"
      : status === "VOID"
        ? "is-negative"
        : "is-muted";
  return (
    <span className={`kind-mark ${cls}`} style={{ minWidth: 64, display: "inline-block" }}>
      {status}
    </span>
  );
}

export function SalesInvoiceList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeSalesInvoices(), []);

  const columns: MockListColumn<SalesInvoice>[] = [
    {
      key: "date",
      label: "Date",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)" }}>{r.date}</span>,
    },
    {
      key: "number",
      label: "Number",
      primary: true,
      render: (r) => r.number,
      secondary: (r) => `Customer: ${r.customer}`,
    },
    {
      key: "dueDate",
      label: "Due",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)", color: "var(--ink-secondary)" }}>{r.dueDate}</span>,
    },
    {
      key: "amount",
      label: "Amount",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : ""),
      render: (r) => formatIDR(r.amount),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <StatusBadge status={r.status} />,
    },
  ];

  return (
    <MockList
      title="Sales Invoices"
      description="Invoices issued to customers (mock data — no backend yet)."
      kind="sales"
      columns={columns}
      rows={rows}
      searchFields={["number", "customer", "date", "dueDate"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, customer, date..."
      onAdd={() => workbench.openEntryDraft("sales-invoice")}
      onOpen={(r) => workbench.openEntryExisting("sales-invoice", r.id, r.number, r.status)}
    />
  );
}

/* ---------------------- Sales Receipt ---------------------- */

export function SalesReceiptList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeSalesReceipts(), []);

  const columns: MockListColumn<SalesReceipt>[] = [
    {
      key: "date",
      label: "Date",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)" }}>{r.date}</span>,
    },
    {
      key: "number",
      label: "Number",
      primary: true,
      render: (r) => r.number,
      secondary: (r) => `Customer: ${r.customer} · ${r.payer}`,
    },
    {
      key: "payer",
      label: "Payer",
      render: (r) => r.payer,
    },
    {
      key: "amount",
      label: "Amount",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : "is-positive"),
      render: (r) => formatIDR(r.amount),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <StatusBadge status={r.status} />,
    },
  ];

  return (
    <MockList
      title="Sales Receipts"
      description="Receipts collected from customers (mock data — no backend yet)."
      kind="sales"
      columns={columns}
      rows={rows}
      searchFields={["number", "customer", "payer", "date"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, customer, payer..."
      onAdd={() => workbench.openEntryDraft("sales-receipt")}
      onOpen={(r) => workbench.openEntryExisting("sales-receipt", r.id, r.number, r.status)}
    />
  );
}

/* ---------------------- Quotations (real backend) ---------------------- */

const QUOTATION_STATUS_TONE: Record<string, string> = {
  DRAFT: "is-muted",
  SENT: "",
  CONVERTED: "is-positive",
  EXPIRED: "is-negative",
  CANCELLED: "is-negative",
};

export function QuotationList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<QuotationListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | QuotationListItem["status"]>("ALL");

  const load = async (filter: "ALL" | QuotationListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listQuotations(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const total = useMemo(() => items.reduce((acc, it) => acc + it.total_cents, 0), [items]);
  const openEntry = (item: QuotationListItem) =>
    workbench.openEntryExisting("sales-quotation-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Quotations</span>
          <small>Sales offers sent to customers (SQ).</small>
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
            onClick={() => workbench.openEntryDraft("sales-quotation-entry")}
          >
            + New Quotation
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading quotations..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No quotations yet"
            message="Send a sales offer to a customer. A quotation is a commitment — it posts no journal."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("sales-quotation-entry")}
              >
                New Quotation
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Customer</span>
              <span>Valid Until</span>
              <span>Status</span>
              <span className="right">Total</span>
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
                <span className="ledger-table__date">{it.quotation_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span className="ledger-table__memo">{it.valid_until ?? "—"}</span>
                <span>
                  <span className={`kind-mark ${QUOTATION_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span>
                </span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Total <strong>{formatIDR(total)}</strong>
        </span>
        <span className="listtab__footer-count">{items.length} quotation(s)</span>
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
