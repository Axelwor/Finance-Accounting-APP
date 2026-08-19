import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { SalesOrderListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

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
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("sales-order-entry")}
          >
            + New Order
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
          <LoadingState label="Loading sales orders..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No sales orders yet"
            message="Convert a quotation into an order, or create one from scratch. An order locks price and quantity — it posts no journal until a down payment is received."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("sales-order-entry")}>
                New Sales Order
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Sales orders list">
            <thead>
              <tr>
                <th scope="col">Number</th>
                <th scope="col">Date</th>
                <th scope="col">Customer</th>
                <th scope="col">Customer PO</th>
                <th scope="col">Status</th>
                <th scope="col" className="right">DP Received</th>
                <th scope="col" className="right">Total</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <SODow key={it.id} item={it} onOpen={() => openEntry(it)} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function SODow({ item, onOpen }: { item: SalesOrderListItem; onOpen: () => void }) {
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.number}</th>
      <td>{item.order_date}</td>
      <td>{item.customer_name ?? `#${item.customer_id}`}</td>
      <td>{item.customer_po_number ?? "—"}</td>
      <td><span className={`kind-mark ${SO_STATUS_TONE[item.status] ?? "is-muted"}`}>{item.status}</span></td>
      <td className="right">{item.dp_received_cents > 0 ? formatIDR(item.dp_received_cents) : "—"}</td>
      <td className="right">{formatIDR(item.total_cents)}</td>
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
