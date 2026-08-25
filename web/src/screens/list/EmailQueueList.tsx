import { useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { EmailQueueItem } from "../../types";
import { Button, Select } from "../../components/m3";

export function EmailQueueList() {
  const [items, setItems] = useState<EmailQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterTrigger, setFilterTrigger] = useState<string>("");
  const [filterStatus, setFilterStatus] = useState<string>("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listEmailQueue();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load email queue.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);
  useTabRefresh(load);

  const handleSend = async (id: number) => {
    if (!confirm("Send this email now?")) return;
    try {
      await api.sendEmail(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to send email.");
    }
  };

  const handleCancel = async (id: number) => {
    if (!confirm("Cancel this queued email?")) return;
    try {
      await api.cancelEmail(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to cancel email.");
    }
  };

  const filtered = items.filter((item) => {
    if (filterTrigger && item.trigger_event !== filterTrigger) return false;
    if (filterStatus === "pending" && item.status !== "PENDING") return false;
    if (filterStatus === "sent" && item.status !== "SENT") return false;
    if (filterStatus === "failed" && item.status !== "FAILED") return false;
    return true;
  });

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case "SENT":
        return "status-badge status-badge--success";
      case "FAILED":
        return "status-badge status-badge--danger";
      default:
        return "status-badge status-badge--info";
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Email Queue</span>
          <small>View and manage queued email notifications.</small>
        </div>
        <div className="listtab__toolbar">
          <Select
            value={filterTrigger}
            onChange={(e) => setFilterTrigger((e.target as HTMLElement & { value: string }).value)}
            options={[
              { value: "", label: "All Triggers" },
              { value: "INVOICE_SENT", label: "Invoice Sent" },
              { value: "PAYMENT_RECEIVED", label: "Payment Received" },
              { value: "CREDIT_NOTE_ISSUED", label: "Credit Note Issued" },
              { value: "QUOTATION_ACCEPTED", label: "Quotation Accepted" },
              { value: "DELIVERY_ORDER_COMPLETED", label: "Delivery Order Completed" },
            ]}
          />
          <Select
            value={filterStatus}
            onChange={(e) => setFilterStatus((e.target as HTMLElement & { value: string }).value)}
            options={[
              { value: "", label: "All Status" },
              { value: "pending", label: "Pending" },
              { value: "sent", label: "Sent" },
              { value: "failed", label: "Failed" },
            ]}
          />
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >Reload</Button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : filtered.length === 0 ? (
          <EmptyState title="No emails in queue" message="Queued emails will appear here." />
        ) : filtered.length > 0 ? (
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: "25%" }}>Subject</th>
                <th style={{ width: "20%" }}>Recipient</th>
                <th style={{ width: "15%" }}>Trigger</th>
                <th style={{ width: "12%" }}>Status</th>
                <th style={{ width: "15%", textAlign: "right" }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((item) => (
                <tr key={item.id}>
                  <td>{item.subject}</td>
                  <td>{item.to_email}</td>
                  <td>{formatTrigger(item.trigger_event ?? "")}</td>
                  <td>
                    <span className={getStatusBadgeClass(item.status)}>
                      {item.status === "SENT" && item.sent_at
                        ? `Sent ${new Date(item.sent_at).toLocaleString()}`
                        : formatStatus(item.status)}
                    </span>
                    {item.last_error && (
                      <div style={{ fontSize: "12px", color: "var(--md-sys-color-error)", marginTop: "4px" }}>
                        Error: {item.last_error}
                      </div>
                    )}
                  </td>
                  <td style={{ textAlign: "right" }}>
                    {(item.status === "PENDING" || item.status === "FAILED") && (
                      <>
                        <Button
                          variant="text"
                          size="sm"
                          onClick={() => handleSend(item.id)}
                          style={{ marginRight: "4px" }}
                        >
                          Send
                        </Button>
                        <Button
                          variant="outlined"
                          size="sm"
                          danger
                          onClick={() => handleCancel(item.id)}
                        >
                          Cancel
                        </Button>
                      </>
                    )}
                    {item.status === "SENT" && (
                      <Button
                        variant="tonal"
                        size="sm"
                        disabled
                      >Viewed</Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{filtered.length} Email(s)</span>
      </div>
    </div>
  );
}

function formatTrigger(event: string): string {
  return event.replace(/_/g, " ");
}

function formatStatus(status: string): string {
  switch (status) {
    case "PENDING":
      return "Pending";
    case "SENT":
      return "Sent";
    case "FAILED":
      return "Failed";
    default:
      return status;
  }
}
