import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { SupplierInvoiceListItem } from "../../types";

const SI_STATUS_TONE: Record<string, string> = {
  ISSUED: "",
  PARTIALLY_PAID: "",
  PAID: "is-positive",
  VOID: "is-negative",
};

export function SupplierInvoiceList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<SupplierInvoiceListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | SupplierInvoiceListItem["status"]>("ALL");

  const load = async (filter: "ALL" | SupplierInvoiceListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listSupplierInvoices(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalPayable = useMemo(
    () => items.filter((i) => i.status !== "VOID").reduce((acc, it) => acc + it.payable_cents, 0),
    [items],
  );
  const openEntry = (item: SupplierInvoiceListItem) =>
    workbench.openEntryExisting("supplier-invoice-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Supplier Invoices</span>
          <small>Supplier invoices (Tagihan / BIL). Reclassifies uninvoiced payables to AP + records input VAT.</small>
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
            onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}
          >
            + New Tagihan
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload" title="Reload">
            <ReloadIcon />
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading supplier invoices..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No supplier invoices yet"
            message="Create your first supplier invoice (Tagihan) to reclassify uninvoiced payables."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}
              >
                New Tagihan
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Supplier</span>
              <span>Date</span>
              <span>Due</span>
              <span className="right">DPP</span>
              <span className="right">VAT</span>
              <span className="right">Total</span>
              <span className="right">Payable</span>
              <span>Status</span>
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
                <span className="ledger-table__cat">{it.supplier_name ?? `#${it.supplier_id}`}</span>
                <span className="ledger-table__date">{it.invoice_date}</span>
                <span className="ledger-table__memo">{it.due_date ?? "—"}</span>
                <span className="ledger-table__amount right">{formatIDR(it.dpp_cents)}</span>
                <span className="ledger-table__amount right">{it.vat_cents > 0 ? formatIDR(it.vat_cents) : "—"}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.payable_cents)}</span>
                <span>
                  <span className={`kind-mark ${SI_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Outstanding payable <strong>{formatIDR(totalPayable)}</strong>
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
