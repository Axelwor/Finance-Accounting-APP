import type { LowStockItem } from "../../types";

/** Low stock alert widget — top 5 items below reorder point. */
export function LowStockWidget({
  items,
  onOpenInventory,
}: {
  items: LowStockItem[];
  onOpenInventory?: () => void;
}) {
  const top = items.slice(0, 5);
  return (
    <div className="dashboard-widget">
      <div className="dashboard-widget__head">
        <h2 className="dashboard-widget__title">Low stock</h2>
        <span className="dashboard-widget__meta">
          {items.length} {items.length === 1 ? "item" : "items"}
        </span>
      </div>
      {top.length === 0 ? (
        <div className="empty-state empty-state--compact">
          <p className="empty-state__message">All tracked items are above reorder point.</p>
        </div>
      ) : (
        <ul className="lowstock-list">
          {top.map((item, i) => {
            const deficit = (item.min_stock_qty ?? 0) - (item.qty_on_hand ?? 0);
            return (
              <li key={`${item.code ?? i}`} className="lowstock-list__row">
                <span className="lowstock-list__code">{item.code ?? "—"}</span>
                <span className="lowstock-list__name">{item.name ?? "Unnamed"}</span>
                <span className="lowstock-list__qty">
                  {item.qty_on_hand ?? 0} / {item.min_stock_qty ?? 0}
                </span>
                <span className="lowstock-list__deficit is-negative">-{deficit}</span>
              </li>
            );
          })}
        </ul>
      )}
      {onOpenInventory && items.length > 0 ? (
        <button type="button" className="dashboard-widget__action" onClick={onOpenInventory}>
          View inventory
        </button>
      ) : null}
    </div>
  );
}
