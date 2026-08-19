import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { Supplier } from "../../types";
import { Button } from "../../components/m3";

export function PurchaseSupplierList() {
  const workbench = useWorkbench();
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listSuppliers().then(setSuppliers).catch(() => setError("Failed to load suppliers")).finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading suppliers..." />;
  if (error) return <FormError message={error} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Suppliers</span>
          <small>Vendor master data for purchase orders.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("purchase-supplier-entry")}
          >
            + New Supplier
          </Button>
          <span className="listtab__count">{suppliers.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {suppliers.length === 0 ? (
          <EmptyState
            title="No suppliers yet"
            message="Add suppliers to create purchase orders and track payables."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("purchase-supplier-entry")}>
                New Supplier
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Contact</span>
              <span>Phone</span>
              <span>City</span>
              <span>Status</span>
            </div>
            {suppliers.map((s) => (
              <div
                key={s.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => workbench.openEntryExisting("purchase-supplier-entry", s.id, `${s.code} · ${s.name}`, s.is_active ? "ACTIVE" : "INACTIVE")}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    workbench.openEntryExisting("purchase-supplier-entry", s.id, `${s.code} · ${s.name}`, s.is_active ? "ACTIVE" : "INACTIVE");
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{s.code}</span>
                <span className="ledger-table__cat">{s.name}</span>
                <span className="ledger-table__memo">{s.contact_person || "—"}</span>
                <span className="ledger-table__memo">{s.phone || "—"}</span>
                <span className="ledger-table__memo">{s.city || "—"}</span>
                <span><span className={`kind-mark ${s.is_active ? "is-positive" : "is-negative"}`}>{s.is_active ? "Active" : "Inactive"}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{suppliers.length} supplier(s)</span>
      </div>
    </div>
  );
}
