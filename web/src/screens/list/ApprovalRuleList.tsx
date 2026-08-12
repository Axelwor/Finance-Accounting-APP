import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRule } from "../../types";

export function ApprovalRuleList() {
  const workbench = useWorkbench();
  const [rules, setRules] = useState<ApprovalRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listApprovalRules().then(setRules).catch(() => setError("Failed to load approval rules")).finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading approval rules..." />;
  if (error) return <FormError message={error} />;

  const entityLabels: Record<string, string> = {
    invoice: "Sales Invoice",
    "purchase-order": "Purchase Order",
    "supplier-invoice": "Supplier Invoice",
    "journal-entry": "Journal Entry",
    "payment": "Payment",
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Approval Rules</span>
          <small>Define approval workflows based on entity type and amount thresholds.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("approval-rule-entry")}
          >
            + New Rule
          </button>
          <span className="listtab__count">{rules.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {rules.length === 0 ? (
          <EmptyState
            title="No approval rules yet"
            message="Add approval rules to require approvals for transactions above specified amounts."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("approval-rule-entry")}
              >
                New Rule
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Entity Type</span>
              <span>Min Amount</span>
              <span>Approver</span>
              <span>Status</span>
            </div>
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() =>
                  workbench.openEntryExisting(
                    "approval-rule-entry",
                    rule.id,
                    `${entityLabels[rule.entity_type] || rule.entity_type} · ${rule.approver_name}`,
                    rule.is_active ? "ACTIVE" : "INACTIVE"
                  )
                }
                onKeyPress={(e) => {
                  if (e.key === "Enter") {
                    workbench.openEntryExisting(
                      "approval-rule-entry",
                      rule.id,
                      `${entityLabels[rule.entity_type] || rule.entity_type} · ${rule.approver_name}`,
                      rule.is_active ? "ACTIVE" : "INACTIVE"
                    );
                  }
                }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{rule.entity_type}</span>
                <span className="ledger-table__cat">{formatAmount(rule.min_amount_cents)}</span>
                <span className="ledger-table__memo">{rule.approver_name}</span>
                <span>
                  <span
                    className={`kind-mark ${rule.is_active ? "is-positive" : "is-negative"}`}
                  >
                    {rule.is_active ? "Active" : "Inactive"}
                  </span>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{rules.length} rule(s)</span>
      </div>
    </div>
  );
}

function formatAmount(cents: number): string {
  if (cents === 0) return "Any amount";
  const rupiah = new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
  });
  return rupiah.format(cents / 100);
}
