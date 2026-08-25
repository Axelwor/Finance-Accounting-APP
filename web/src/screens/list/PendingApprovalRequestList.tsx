import { useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRequest } from "../../types";
import { Button } from "../../components/m3";

const ENTITY_TYPE_LABEL: Record<string, string> = {
  invoice: "Sales Invoice",
  purchase_order: "Purchase Order",
  supplier_invoice: "Supplier Invoice",
  credit_note: "Credit Note",
  journal_entry: "Journal Entry",
};

export function PendingApprovalRequestList() {
  const [items, setItems] = useState<ApprovalRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionTarget, setActionTarget] = useState<ApprovalRequest | null>(null);
  const [actionType, setActionType] = useState<"approve" | "reject" | null>(null);
  const [reason, setReason] = useState("");
  const [acting, setActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listApprovalRequests({ status: "PENDING" });
      setItems(data.filter((i) => i.status === "PENDING"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load pending requests.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);
  useTabRefresh(load);

  // Backend already orders by requested_at DESC.
  const sortedItems = [...items].sort(
    (a, b) => new Date(b.requested_at).getTime() - new Date(a.requested_at).getTime(),
  );

  const formatStatus = (status: string) => {
    switch (status) {
      case "PENDING":
        return <span className="badge badge--warning">Pending</span>;
      case "APPROVED":
        return <span className="badge badge--success">Approved</span>;
      case "REJECTED":
        return <span className="badge badge--danger">Rejected</span>;
      default:
        return status;
    }
  };

  const openAction = (request: ApprovalRequest, type: "approve" | "reject") => {
    setActionTarget(request);
    setActionType(type);
    setReason("");
    setActionError(null);
  };

  const closeAction = () => {
    setActionTarget(null);
    setActionType(null);
    setReason("");
    setActionError(null);
  };

  const handleConfirm = async () => {
    if (!actionTarget || !actionType) return;
    if (actionType === "reject" && reason.trim() === "") {
      setActionError("Rejection reason is required.");
      return;
    }
    setActing(true);
    setActionError(null);
    try {
      if (actionType === "approve") {
        await api.approveApprovalRequest(actionTarget.id, { reason });
      } else {
        await api.rejectApprovalRequest(actionTarget.id, { reason });
      }
      closeAction();
      await load();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to process approval.");
    } finally {
      setActing(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Pending Approvals</span>
          <small>Review and approve/reject pending transactions</small>
        </div>
        <div className="listtab__toolbar">
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Reload
          </Button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState title="No pending approvals" message="All requests have been processed." />
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Entity</th>
                <th>Status</th>
                <th>Requested By</th>
                <th>Requested At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sortedItems.map((request) => (
                <tr key={request.id}>
                  <td>
                    <strong>{ENTITY_TYPE_LABEL[request.entity_type] ?? request.entity_type}</strong>{" "}
                    {request.entity_number || `#${request.entity_id}`}
                  </td>
                  <td>{formatStatus(request.status)}</td>
                  <td>{request.requested_by || "Unknown"}</td>
                  <td>{new Date(request.requested_at).toLocaleString()}</td>
                  <td>
                    <Button
                      variant="filled"
                      size="sm"
                      success
                      onClick={() => openAction(request, "approve")}
                    >
                      Approve
                    </Button>{" "}
                    <Button
                      variant="outlined"
                      size="sm"
                      danger
                      onClick={() => openAction(request, "reject")}
                    >
                      Reject
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Pending Request(s)</span>
      </div>

      {actionTarget && actionType ? (
        <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="approval-action-title">
          <div className="modal" style={{ maxWidth: "480px" }}>
            <div className="modal__header">
              <h3 id="approval-action-title">
                {actionType === "approve" ? "Approve Transaction" : "Reject Transaction"}
              </h3>
              <button type="button" className="modal__close" onClick={closeAction} aria-label="Close">×</button>
            </div>
            <div className="modal__body">
              <p>
                <strong>Entity:</strong>{" "}
                {ENTITY_TYPE_LABEL[actionTarget.entity_type] ?? actionTarget.entity_type}{" "}
                {actionTarget.entity_number || `#${actionTarget.entity_id}`}
              </p>
              <p><strong>Requested by:</strong> {actionTarget.requested_by || "Unknown"}</p>
              <div className="form-group">
                <label htmlFor="approval-action-reason">
                  Reason {actionType === "reject" ? "*" : "(optional)"}
                </label>
                <textarea
                  id="approval-action-reason"
                  className="form-control"
                  rows={3}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={
                    actionType === "reject"
                      ? "Enter rejection reason..."
                      : "Enter approval note (optional)..."
                  }
                />
              </div>
              {actionType === "reject" && reason.trim() === "" ? (
                <p style={{ color: "var(--md-sys-color-error)" }}>Reason is required for rejection.</p>
              ) : null}
              {actionError ? (
                <p style={{ color: "var(--md-sys-color-error)" }}>{actionError}</p>
              ) : null}
            </div>
            <div className="modal__footer">
              <Button variant="tonal" onClick={closeAction} disabled={acting}>
                Cancel
              </Button>
              <Button
                variant="filled"
                success={actionType === "approve"}
                danger={actionType === "reject"}
                disabled={acting}
                onClick={() => void handleConfirm()}
              >
                {acting ? "Processing..." : actionType === "approve" ? "Confirm Approve" : "Confirm Reject"}
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
