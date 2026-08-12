import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { Warehouse } from "../../types";
import { WarehouseStockList } from "./WarehouseStockList";

export function WarehouseList() {
  const workbench = useWorkbench();
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [stockWarehouse, setStockWarehouse] = useState<Warehouse | null>(null);

  useEffect(() => {
    api.listWarehouses().then(setWarehouses).catch(() => setError("Failed to load warehouses")).finally(() => setLoading(false));
  }, []);

  const filteredWarehouses = warehouses.filter((wh) =>
    wh.code.toLowerCase().includes(searchTerm.toLowerCase()) ||
    wh.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  if (loading) return <LoadingState label="Loading warehouses..." />;
  if (error) return <FormError message={error} />;

  if (stockWarehouse) {
    return (
      <WarehouseStockList
        warehouseId={stockWarehouse.id}
        warehouseName={`${stockWarehouse.code} · ${stockWarehouse.name}`}
        onBack={() => setStockWarehouse(null)}
      />
    );
  }

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Warehouses</span>
          <small>Master data for inventory locations and stock management.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Search</span>
            <input
              className="filter-pill__input"
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Filter by code or name..."
            />
          </label>
        </div>
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("warehouse-entry")}
          >
            + New Warehouse
          </button>
          <span className="listtab__count">{filteredWarehouses.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {filteredWarehouses.length === 0 ? (
          <EmptyState
            title="No warehouses yet"
            message="Add warehouses to track inventory across multiple locations."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("warehouse-entry")}
              >
                New Warehouse
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Address</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {filteredWarehouses.map((wh) => (
              <div
                key={wh.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() =>
                  workbench.openEntryExisting("warehouse-entry", wh.id, `${wh.code} · ${wh.name}`, wh.is_active ? "ACTIVE" : "INACTIVE")
                }
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    workbench.openEntryExisting("warehouse-entry", wh.id, `${wh.code} · ${wh.name}`, wh.is_active ? "ACTIVE" : "INACTIVE");
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{wh.code}</span>
                <span className="ledger-table__cat">{wh.name}</span>
                <span className="ledger-table__memo">{wh.address || "—"}</span>
                <span>
                  <span className={`kind-mark ${wh.is_active ? "is-positive" : "is-negative"}`}>{wh.is_active ? "Active" : "Inactive"}</span>
                </span>
                <span className="ledger-table__action">
                  <button
                    type="button"
                    className="btn btn--ghost btn--xs"
                    onClick={(e) => {
                      e.stopPropagation();
                      setStockWarehouse(wh);
                    }}
                  >
                    View Stock
                  </button>
                  <button
                    type="button"
                    className="btn btn--ghost btn--xs"
                    onClick={(e) => {
                      e.stopPropagation();
                      workbench.openEntryExisting("warehouse-entry", wh.id, `${wh.code} · ${wh.name}`, wh.is_active ? "ACTIVE" : "INACTIVE");
                    }}
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    className="btn btn--negative btn--xs"
                    onClick={(e) => {
                      e.stopPropagation();
                      if (!window.confirm(`Delete warehouse "${wh.code}"? This cannot be undone.`)) return;
                      api.deactivateWarehouse(wh.id).then(() => {
                        setWarehouses((prev) => prev.filter((w) => w.id !== wh.id));
                      }).catch((err) => {
                        alert(err instanceof Error ? err.message : "Failed to delete warehouse.");
                      });
                    }}
                  >
                    Delete
                  </button>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{filteredWarehouses.length} warehouse(s)</span>
      </div>
    </div>
  );
}
