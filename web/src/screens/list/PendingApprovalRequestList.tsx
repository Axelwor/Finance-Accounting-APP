import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRequest } from "../../types";
import { ApprovalActions } from "../../components/ApprovalActions";

export function PendingApprovalRequestList() {
  const workbench = useWorkbench();
  const [requests, setRequests] = useState<ApprovalRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listPendingApprovalRequests().then(setRequests).catch(() => setError("Failed to load pending requests")).finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading pending approval requests..." />;
  if (error) return <FormError message={error} />;

  const entityLabels: Record<string, string> = {
    invoice: "Sales Invoice",
    "purchase-order": "Purchase Order",
    "supplier-invoice": "Supplier Invoice",
    "journal-entry": "Journal Entry",
    payment: "Payment",
  };

  const statusLabels: Record<string, string> = {
    pending: "Pending",
    approved: "Approved",
    rejected: "Rejected",
    cancelled: "Cancelled",
  };

  const formatDate = (dateStr: string): string => {
    try {
      return new Date(dateStr).toLocaleDateString("id-ID", {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  const formatAmount = (cents: number): string => {
    if (cents === 0) return "Any amount";
    const rupiah = new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
    });
    return rupiah.format(cents / 100);
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Pending Approvals</span>
          <small>Review and approve or reject transactions awaiting approval.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <span className="listtab__count">{requests.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {requests.length === 0 ? (
          <EmptyState
            title="No pending approvals"
            message="All transactions have been processed. There are no pending approval requests at this time."
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Entity</span>
              <span>Reference</span>
              <span>Amount</span>
              <span>Status</span>
              <span>Created</span>
              <span>Actions</span>
            </div>
            {requests.map((req) => (
              <div key={req.id} className="ledger-table__row">
                <span className="ledger-table__no">{entityLabels[req.entity_type] || req.entity_type}</span>
                <span className="ledger-table__cat">
                  {req.entity_id ? `#${req.entity_id}` : "-"}
                </span>
                <span className="ledger-table__memo">{formatAmount(req.amount_cents)}</span>
                <span>
                  <span className={`kind-mark ${req.status === "pending" ? "is-warning" : req.status === "approved" ? "is-positive" : "is-negative"}`}>
                    {statusLabels[req.status] || req.status}
                  </span>
                </span>
                <span className="ledger-table__memo">{formatDate(req.created_at)}</span>
                <span className="ledger-table__actions">
                  <ApprovalActions request={req} onSuccess={() => {
                    api.listPendingApprovalRequests().then(setRequests).catch(() => {});
                  }} />
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{requests.length} request(s)</span>
      </div>
    </div>
  );
}
