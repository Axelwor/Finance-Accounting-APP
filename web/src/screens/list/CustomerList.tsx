import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { Customer } from "../../types";

export function CustomerList() {
  const workbench = useWorkbench();
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listCustomers().then(setCustomers).catch(() => setError("Failed to load customers")).finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading customers..." />;
  if (error) return <FormError message={error} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Customers</span>
          <small>Customer master data for sales orders and invoices.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("customer-entry")}>
            + New Customer
          </button>
          <span className="listtab__count">{customers.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {customers.length === 0 ? (
          <EmptyState
            title="No customers yet"
            message="Add customers to create sales orders, delivery orders, and invoices."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("customer-entry")}>
                New Customer
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Price Level</span>
              <span>Group</span>
              <span>Phone</span>
              <span>Status</span>
            </div>
            {customers.map((c) => (
              <div
                key={c.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => workbench.openEntryExisting("customer-entry", c.id, `${c.code} · ${c.name}`, c.is_active ? "ACTIVE" : "INACTIVE")}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    workbench.openEntryExisting("customer-entry", c.id, `${c.code} · ${c.name}`, c.is_active ? "ACTIVE" : "INACTIVE");
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{c.code}</span>
                <span className="ledger-table__cat">{c.name}</span>
                <span className="ledger-table__memo">{c.price_level || "—"}</span>
                <span className="ledger-table__memo">{c.customer_group || "—"}</span>
                <span className="ledger-table__memo">{c.phone || "—"}</span>
                <span><span className={`kind-mark ${c.is_active ? "is-positive" : "is-negative"}`}>{c.is_active ? "Active" : "Inactive"}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{customers.length} customer(s)</span>
      </div>
    </div>
  );
}
