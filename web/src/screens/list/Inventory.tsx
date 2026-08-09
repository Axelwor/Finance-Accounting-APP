import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { Item } from "../../types";

export function InventoryItemsList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listItems()
      .then(setItems)
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Item List</span>
          <small>Master data for goods and services.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("inventory-item")}>
            + New Item
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading items..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No items yet"
            message="Add goods or service items to start creating quotations, orders, and invoices."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("inventory-item")}>
                New Item
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Type</span>
              <span>UoM</span>
              <span className="right">Price</span>
              <span>Status</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => workbench.openEntryExisting("inventory-item", it.id, it.code, it.is_active ? "ACTIVE" : "INACTIVE")}
                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); workbench.openEntryExisting("inventory-item", it.id, it.code, it.is_active ? "ACTIVE" : "INACTIVE"); } }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{it.code}</span>
                <span className="ledger-table__cat">{it.name}</span>
                <span><span className={`kind-mark ${it.item_type === "goods" ? "" : "is-muted"}`}>{it.item_type}</span></span>
                <span className="ledger-table__memo">{it.unit || "—"}</span>
                <span className="ledger-table__amount right">{it.sale_price_cents ? formatIDR(it.sale_price_cents) : "—"}</span>
                <span><span className={`kind-mark ${it.is_active ? "is-positive" : "is-negative"}`}>{it.is_active ? "Active" : "Inactive"}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} item(s)</span>
      </div>
    </div>
  );
}

export function StockMovementsList() {
  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Stock Movements</span>
          <small>Receipts, issues, transfers, and adjustments.</small>
        </div>
      </div>
      <div className="listtab__body">
        <EmptyState
          title="Stock movements"
          message="Stock movements are recorded automatically by GRN, DO, Stock Opname, and Stock Transfer. View individual item movements from the Item List."
        />
      </div>
    </div>
  );
}
