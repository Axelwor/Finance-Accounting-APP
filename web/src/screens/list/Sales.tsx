import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { QuotationListItem, InvoiceListItem } from "../../types";

/* ---------------------- Sales Invoice (real backend) ---------------------- */

export function SalesInvoiceList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<InvoiceListItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listInvoices().then(setItems).catch(() => setItems([])).finally(() => setLoading(false));
  }, []);

  const openEntry = (it: InvoiceListItem) =>
    workbench.openEntryExisting("sales-invoice", it.id, it.number, it.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Sales Invoices</span>
          <small>Invoices issued to customers.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("sales-invoice")}>+ New Invoice</button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? <LoadingState label="Loading invoices..." /> : items.length === 0 ? (
          <EmptyState title="No invoices yet" message="Issue an invoice to recognize revenue and receivables." action={<button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("sales-invoice")}>New Invoice</button>} />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span><span>Date</span><span>Customer</span><span>Status</span><span className="right">Receivable</span>
            </div>
            {items.map((it) => (
              <div key={it.id} className="ledger-table__row" role="button" tabIndex={0} onClick={() => openEntry(it)} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); openEntry(it); } }} style={{ cursor: "pointer" }}>
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.invoice_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span><span className={`kind-mark ${it.status === "PAID" ? "is-positive" : it.status === "VOID" ? "is-negative" : ""}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{formatIDR(it.receivable_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/* ---------------------- Sales Receipt (real backend — paid invoices) ---------------------- */

export function SalesReceiptList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<InvoiceListItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listInvoices("PAID").then(setItems).catch(() => setItems([])).finally(() => setLoading(false));
  }, []);

  const totalReceived = useMemo(() => items.reduce((acc, it) => acc + (it.total_cents - it.receivable_cents), 0), [items]);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Sales Receipts</span>
          <small>Paid invoices — payments received from customers.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__actions">
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? <LoadingState label="Loading receipts..." /> : items.length === 0 ? (
          <EmptyState title="No receipts yet" message="Paid invoices will appear here. Receive payments from the Invoice screen." />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span><span>Date</span><span>Customer</span><span className="right">Total</span><span className="right">Received</span>
            </div>
            {items.map((it) => (
              <div key={it.id} className="ledger-table__row" role="button" tabIndex={0} onClick={() => workbench.openEntryExisting("sales-invoice", it.id, it.number, it.status)} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); workbench.openEntryExisting("sales-invoice", it.id, it.number, it.status); } }} style={{ cursor: "pointer" }}>
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.invoice_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents - it.receivable_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span>Total received <strong>{formatIDR(totalReceived)}</strong></span>
        <span className="listtab__footer-count">{items.length} receipt(s)</span>
      </div>
    </div>
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
