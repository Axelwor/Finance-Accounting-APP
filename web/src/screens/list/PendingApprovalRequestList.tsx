import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRequest } from "../../types";
import { ApprovalActions } from "../../components/ApprovalActions";
import { formatIDR } from "../../lib/format";

export function PendingApprovalRequestList() {
  const [items, setItems] = useState<ApprovalRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedRequest, setSelectedRequest] = useState<ApprovalRequest | null>(null);
  const [showHistoryModal, setShowHistoryModal] = useState(false);
  const [sortByAmount, setSortByAmount] = useState(true);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listApprovalRequests();
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

  const sortedItems = [...items].sort((a, b) => {
    if (sortByAmount) return b.amount_cents - a.amount_cents;
    return new Date(a.requested_at).getTime() - new Date(b.requested_at).getTime();
  });

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

  const handleApproveSuccess = () => {
    load();
  };

  const getHistoryColor = (action: string) => {
    switch (action) {
      case "approved":
        return "success";
      case "rejected":
        return "danger";
      default:
        return "";
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
          <button
            type="button"
            className="btn btn--secondary btn--sm"
            onClick={() => void load()}
          >
            Reload
          </button>
          <label style={{ display: "flex", alignItems: "center", gap: "4px", fontSize: "12px" }}>
            <input
              type="checkbox"
              checked={sortByAmount}
              onChange={(e) => setSortByAmount(e.target.checked)}
            />
            Sort by Amount
          </label>
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
          <>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Entity</th>
                  <th>Amount</th>
                  <th>Requested By</th>
                  <th>Requested At</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sortedItems.map((request) => (
                  <tr key={request.id}>
                    <td>
                      <strong>{request.entity_type}</strong> #{request.entity_id}
                      <br />
                      <small style={{ color: "var(--muted)" }}>{formatStatus(request.status)}</small>
                    </td>
                    <td style={{ fontFamily: "var(--font-mono)", fontWeight: "500" }}>
                      {formatIDR(request.amount_cents / 100)}
                    </td>
                    <td>{request.requested_by || "Unknown"}</td>
                    <td>{new Date(request.requested_at).toLocaleString()}</td>
                    <td>
                      <button
                        type="button"
                        className="btn btn--sm btn--secondary"
                        onClick={() => {
                          setSelectedRequest(request);
                          setShowHistoryModal(true);
                        }}
                        title="View History"
                      >
                        📜
                      </button>
                      <ApprovalActions
                        request={request}
                        onSuccess={handleApproveSuccess}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {selectedRequest && showHistoryModal && (
              <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="history-modal-title">
                <div className="modal" style={{ maxWidth: "500px" }}>
                  <div className="modal__header">
                    <h3 id="history-modal-title">Approval History - {selectedRequest.entity_type} #{selectedRequest.entity_id}</h3>
                    <button
                      type="button"
                      className="modal__close"
                      onClick={() => {
                        setShowHistoryModal(false);
                        setSelectedRequest(null);
                      }}
                      aria-label="Close"
                    >
                      ×
                    </button>
                  </div>
                  <div className="modal__body" style={{ maxHeight: "60vh", overflowY: "auto" }}>
                    {selectedRequest.approval_history && selectedRequest.approval_history.length > 0 ? (
                      <div className="timeline-container">
                        <ul className="timeline timeline--vertical">
                          {selectedRequest.approval_history.map((entry, idx) => (
                            <li key={idx} className={`timeline-item timeline-item--${getHistoryColor(entry.action)}`}>
                              <div className="timeline-marker">
                                {entry.action === "approved" ? "✓" : "✕"}
                              </div>
                              <div className="timeline-content">
                                <div className="timeline-text">
                                  <strong>{entry.by}</strong> {entry.action} this transaction
                                </div>
                                <div className="timeline-time">{new Date(entry.at).toLocaleString()}</div>
                              </div>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ) : (
                      <p style={{ color: "var(--muted)" }}>No approval history yet.</p>
                    )}
                    {(selectedRequest.approved_at || selectedRequest.rejected_at) && (
                      <div style={{ marginTop: "16px", borderTop: "1px solid var(--border)", paddingTop: "12px" }}>
                        {selectedRequest.approved_at && (
                          <div>
                            <strong>Approved:</strong> {new Date(selectedRequest.approved_at).toLocaleString()}
                            {selectedRequest.approved_by && <em> by {selectedRequest.approved_by}</em>}
                          </div>
                        )}
                        {selectedRequest.rejected_at && (
                          <div style={{ marginTop: "8px" }}>
                            <strong>Rejected:</strong> {new Date(selectedRequest.rejected_at).toLocaleString()}
                            {selectedRequest.rejected_by && <em> by {selectedRequest.rejected_by}</em>}
                            {selectedRequest.rejection_reason && (
                              <div style={{ marginTop: "4px", color: "var(--neg)" }}>
                                Reason: {selectedRequest.rejection_reason}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                  <div className="modal__footer">
                    <button
                      type="button"
                      className="btn"
                      onClick={() => {
                        setShowHistoryModal(false);
                        setSelectedRequest(null);
                      }}
                    >
                      Close
                    </button>
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Pending Request(s)</span>
      </div>
    </div>
  );
}
