import { useState } from "react";
import { useToast } from "./Toast";
import { api } from "../api";
import type { ApprovalRequest } from "../types";
import { Button } from "./m3";

interface ApprovalActionsProps {
  request: ApprovalRequest;
  onSuccess?: () => void;
}

export function ApprovalActions({ request, onSuccess }: ApprovalActionsProps) {
  const toast = useToast();
  const [showModal, setShowModal] = useState(false);
  const [actionType, setActionType] = useState<"approve" | "reject" | null>(null);
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);

  const openModal = (type: "approve" | "reject") => {
    setActionType(type);
    setReason("");
    setShowModal(true);
  };

  const handleSubmit = async () => {
    if (!actionType) return;
    setLoading(true);
    try {
      if (actionType === "approve") {
        await api.approveApprovalRequest(request.id, { reason } as any);
        toast.success(`✓ ${request.entity_type} approved`);
      } else {
        await api.rejectApprovalRequest(request.id, { reason } as any);
        toast.error(`✕ ${request.entity_type} rejected`);
      }
      setShowModal(false);
      setActionType(null);
      setReason("");
      onSuccess?.();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Failed to process approval.";
      if (actionType === "approve") {
        toast.error(`✕ Failed to approve: ${message}`);
      } else {
        toast.error(`✕ Failed to reject: ${message}`);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        variant="filled"
        size="sm"
        success
        onClick={() => openModal("approve")}
        disabled={request.status !== "PENDING"}
      >
        Approve
      </Button>
      <Button
        variant="outlined"
        size="sm"
        danger
        onClick={() => openModal("reject")}
        disabled={request.status !== "PENDING"}
      >
        Reject
      </Button>
      {showModal && (
        <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="approval-modal-title">
          <div className="modal">
            <div className="modal__header">
              <h3 id="approval-modal-title">{actionType === "approve" ? "Approve Transaction" : "Reject Transaction"}</h3>
              <button type="button" className="modal__close" onClick={() => setShowModal(false)} aria-label="Close">×</button>
            </div>
            <div className="modal__body">
              <p><strong>Entity:</strong> {request.entity_type} #{request.entity_id}</p>
              <p><strong>Amount:</strong> {(request.amount_cents / 100).toLocaleString("id-ID", { style: "currency", currency: "IDR" })}</p>
              <p><strong>Requested by:</strong> {request.requested_by || "Unknown"}</p>
              <hr />
              <div className="form-group">
                <label htmlFor="approval-reason">Reason {actionType === "reject" ? "*" : ""}</label>
                <textarea
                  id="approval-reason"
                  className="form-control"
                  rows={4}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={actionType === "reject" ? "Enter rejection reason..." : "Enter approval reason (optional)..."}
                />
              </div>
              {actionType === "reject" && !reason.trim() && (
                <p className="field__error">Reason is required for rejection.</p>
              )}
              {request.approval_history && request.approval_history.length > 0 && (
                <div className="approval-history">
                  <h4>Approval History</h4>
                  <ul className="timeline">
                    {request.approval_history.map((entry, idx) => (
                      <li key={idx} className={`timeline__item timeline__item--${entry.action}`}>
                        <span className="timeline__marker">{getIconForStatus(entry.action)}</span>
                        <div className="timeline__content">
                          <span className="timeline__status">{entry.action}</span>
                          <span className="timeline__actor">by {entry.by || "Unknown"}</span>

                          <span className="timeline__date">{new Date(entry.at).toLocaleString("id-ID")}</span>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
            <div className="modal__footer">
              <Button
                variant="tonal"
                onClick={() => setShowModal(false)}
                disabled={loading}
              >Cancel</Button>
              <Button
                variant="filled"
                success={actionType === "approve"}
                danger={actionType === "reject"}
                onClick={handleSubmit}
                disabled={loading || (actionType === "reject" && !reason.trim())}
              >
                {loading ? "Processing..." : actionType === "approve" ? "Confirm Approve" : "Confirm Reject"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function getIconForStatus(status: string): string {
  switch (status.toLowerCase()) {
    case "approved":
    case "approve":
      return "✓";
    case "rejected":
    case "reject":
      return "✕";
    default:
      return "•";
  }
}
