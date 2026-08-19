import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { DeliveryOrderListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const DO_STATUS_TONE: Record<string, string> = {
  SHIPPED: "is-positive",
  RETURNED: "is-negative",
  CANCELLED: "is-negative",
};

export function DeliveryOrderList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<DeliveryOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | DeliveryOrderListItem["status"]>("ALL");

  const load = async (filter: "ALL" | DeliveryOrderListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listDeliveryOrders(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalCOGS = useMemo(() => items.reduce((acc, it) => acc + it.total_cogs_cents, 0), [items]);
  const openEntry = (item: DeliveryOrderListItem) =>
    workbench.openEntryExisting("delivery-order-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Delivery Orders</span>
          <small>Goods shipped to customers (DO). Posts COGS journal on creation.</small>
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
            onClick={() => workbench.openEntryDraft("delivery-order-entry")}
          >
            + New Delivery
          </Button>
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
          <LoadingState label="Loading delivery orders..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No delivery orders yet"
            message="Ship goods against a sales order. Each delivery posts a COGS journal (Dr COGS / Cr Inventory) and records an inventory movement."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("delivery-order-entry")}>
                New Delivery Order
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Customer</span>
              <span>SO Number</span>
              <span>Status</span>
              <span className="right">Total COGS</span>
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
                <span className="ledger-table__date">{it.delivery_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span className="ledger-table__memo">SO #{it.sales_order_id}</span>
                <span>
                  <span className={`kind-mark ${DO_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span>
                </span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cogs_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Total COGS <strong>{formatIDR(totalCOGS)}</strong>
        </span>
        <span className="listtab__footer-count">{items.length} delivery(ies)</span>
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
