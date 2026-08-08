import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { SalesOrderListItem } from "../../types";

const SO_STATUS_TONE: Record<string, string> = {
  CONFIRMED: "",
  CLOSED: "is-positive",
  CANCELLED: "is-negative",
};

export function SalesOrderList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<SalesOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | SalesOrderListItem["status"]>("ALL");

  const load = async (filter: "ALL" | SalesOrderListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listSalesOrders(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const total = useMemo(() => items.reduce((acc, it) => acc + it.total_cents, 0), [items]);
  const dpTotal = useMemo(() => items.reduce((acc, it) => acc + it.dp_received_cents, 0), [items]);
  const openEntry = (item: SalesOrderListItem) =>
    workbench.openEntryExisting("sales-order-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Sales Orders</span>
          <small>Confirmed customer orders with down payments (SO).</small>
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
            onClick={() => workbench.openEntryDraft("sales-order-entry")}
          >
            + New Order
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading sales orders..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No sales orders yet"
            message="Convert a quotation into an order, or create one from scratch. An order locks price and quantity — it posts no journal until a down payment is received."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("sales-order-entry")}
              >
                New Sales Order
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Customer</span>
              <span>Status</span>
              <span className="right">DP Received</span>
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
                <span className="ledger-table__date">{it.order_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span>
                  <span className={`kind-mark ${SO_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span>
                </span>
                <span className="ledger-table__amount right">{it.dp_received_cents > 0 ? formatIDR(it.dp_received_cents) : "—"}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          DP <strong>{formatIDR(dpTotal)}</strong> · Total <strong>{formatIDR(total)}</strong>
        </span>
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
