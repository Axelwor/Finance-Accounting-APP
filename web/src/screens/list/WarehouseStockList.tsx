import { useEffect, useState } from "react";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { WarehouseStockItem } from "../../types";
import { Button } from "../../components/m3";

interface Props {
  warehouseId: number;
  warehouseName: string;
  onBack?: () => void;
}

export function WarehouseStockList({ warehouseId, warehouseName, onBack }: Props) {
  const [items, setItems] = useState<WarehouseStockItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showDetails, setShowDetails] = useState<number | null>(null);

  useEffect(() => {
    setLoading(true);
    api.getWarehouseStock(warehouseId)
      .then(setItems)
      .catch(() => setError("Failed to load stock items"))
      .finally(() => setLoading(false));
  }, [warehouseId]);

  const fmtIDR = (cents: number): string =>
    new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR" }).format(cents / 100);

  if (loading) return <LoadingState label={`Loading stock for ${warehouseName}...`} />;
  if (error) return <FormError message={error} />;

  const activeItem = showDetails !== null ? items.find((i) => i.item_id === showDetails) : null;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>
            Stock · {warehouseName}
          </span>
          <small>Current inventory levels per item.</small>
        </div>
      </div>
      <div className="listtab__body">
        {items.length === 0 ? (
          <EmptyState
            title="No items in this warehouse"
            message="Add items to track inventory or transfer stock from other warehouses."
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Qty on Hand</span>
              <span>Avg Cost</span>
              <span>Total Value</span>
            </div>
            {items.map((item) => (
              <div
                key={item.item_id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => setShowDetails(item.item_id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setShowDetails(item.item_id);
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{item.item_code}</span>
                <span className="ledger-table__cat">{item.item_name}</span>
                <span className="ledger-table__memo">{item.qty_on_hand.toFixed(2)}</span>
                <span className="ledger-table__amount">{fmtIDR(item.avg_unit_cost_cents)}</span>
                <span className="ledger-table__amount">
                  {fmtIDR(Math.round(item.qty_on_hand * item.avg_unit_cost_cents))}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} item(s)</span>
      </div>

      {activeItem && (
        <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="stock-detail-title">
          <div className="modal modal--accurate">
            <div className="modal__header">
              <h3 id="stock-detail-title" className="modal__title">
                Stock Details — {activeItem.item_code}
              </h3>
              <button
                type="button"
                className="modal__close"
                onClick={() => setShowDetails(null)}
                aria-label="Close"
              >
                ×
              </button>
            </div>
            <div className="modal__body">
              <dl className="detail-list">
                <dt>Item Name</dt>
                <dd>{activeItem.item_name}</dd>
                <dt>Quantity on Hand</dt>
                <dd>{activeItem.qty_on_hand.toFixed(2)}</dd>
                <dt>Average Unit Cost</dt>
                <dd>{fmtIDR(activeItem.avg_unit_cost_cents)}</dd>
                <dt>Total Value</dt>
                <dd className="is-positive">
                  {fmtIDR(Math.round(activeItem.qty_on_hand * activeItem.avg_unit_cost_cents))}
                </dd>
              </dl>
            </div>
            <div className="modal__footer">
              <Button variant="filled" onClick={() => setShowDetails(null)}>
                Close
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
