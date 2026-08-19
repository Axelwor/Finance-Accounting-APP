import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { SupplierInvoiceListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

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
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}
          >
            + New Tagihan
          </Button>
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
            title="Reload"
          >
            <ReloadIcon />
          </IconButton>
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
              <Button variant="filled" onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}>
                New Tagihan
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Supplier invoices list">
            <thead>
              <tr>
                <th scope="col">Number</th>
                <th scope="col">Supplier</th>
                <th scope="col">Date</th>
                <th scope="col">Due</th>
                <th scope="col" className="right">DPP</th>
                <th scope="col" className="right">VAT</th>
                <th scope="col" className="right">Total</th>
                <th scope="col" className="right">Payable</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <SIRow key={it.id} item={it} onOpen={() => openEntry(it)} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function SIRow({ item, onOpen }: { item: SupplierInvoiceListItem; onOpen: () => void }) {
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.number}</th>
      <td>{item.supplier_name ?? `#${item.supplier_id}`}</td>
      <td>{item.invoice_date}</td>
      <td>{item.due_date ?? "—"}</td>
      <td className="right">{formatIDR(item.dpp_cents)}</td>
      <td className="right">{item.vat_cents > 0 ? formatIDR(item.vat_cents) : "—"}</td>
      <td className="right">{formatIDR(item.total_cents)}</td>
      <td className="right">{formatIDR(item.payable_cents)}</td>
      <td><span className={`kind-mark ${SI_STATUS_TONE[item.status] ?? "is-muted"}`}>{item.status}</span></td>
    </tr>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
