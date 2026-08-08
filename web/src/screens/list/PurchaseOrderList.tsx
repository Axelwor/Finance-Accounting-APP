import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { PurchaseOrderListItem } from "../../types";

const PO_STATUS_TONE: Record<string, string> = {
  CONFIRMED: "",
  PARTIALLY_RECEIVED: "",
  RECEIVED: "is-positive",
  CANCELLED: "is-negative",
};

export function PurchaseOrderList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<PurchaseOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listPurchaseOrders();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const openEntry = (item: PurchaseOrderListItem) =>
    workbench.openEntryExisting("purchase-order-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Purchase Orders</span>
          <small>Supplier purchase orders (PO). Commitment only — no journal posted.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("purchase-order-entry")}>
            + New PO
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading purchase orders..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No purchase orders yet"
            message="Create a purchase order to order goods from a supplier. PO is a commitment only — no journal is posted until goods are received (GRN)."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("purchase-order-entry")}>
                New Purchase Order
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Supplier</span>
              <span>Status</span>
              <span className="right">Received</span>
              <span className="right">Total</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => openEntry(it)}
                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); openEntry(it); } }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.order_date}</span>
                <span className="ledger-table__cat">{it.supplier_name ?? `#${it.supplier_id}`}</span>
                <span><span className={`kind-mark ${PO_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{it.received_cents > 0 ? formatIDR(it.received_cents) : "—"}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} order(s)</span>
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
