import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { EmailTemplate } from "../../types";

type TriggerEvent =
  | "INVOICE_SENT"
  | "PAYMENT_RECEIVED"
  | "CREDIT_NOTE_ISSUED"
  | "QUOTATION_ACCEPTED"
  | "DELIVERY_ORDER_COMPLETED";

const TRIGGER_OPTIONS: { value: string; label: string }[] = [
  { value: "INVOICE_SENT", label: "Invoice Sent" },
  { value: "PAYMENT_RECEIVED", label: "Payment Received" },
  { value: "CREDIT_NOTE_ISSUED", label: "Credit Note Issued" },
  { value: "QUOTATION_ACCEPTED", label: "Quotation Accepted" },
  { value: "DELIVERY_ORDER_COMPLETED", label: "Delivery Order Completed" },
];

export function EmailTemplateList() {
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<EmailTemplate | null>(null);
  const [filterTrigger, setFilterTrigger] = useState<string>("");
  const [filterStatus, setFilterStatus] = useState<string>("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listEmailTemplates();
      setTemplates(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load email templates.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const handleToggleActive = async (template: EmailTemplate) => {
    try {
      await api.updateEmailTemplate(template.id, { is_active: !template.is_active });
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to update template status.");
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this email template?")) return;
    try {
      await api.deleteEmailTemplate(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete template.");
    }
  };

  const openEdit = (template: EmailTemplate) => {
    setEditingTemplate(template);
    setShowForm(true);
  };

  const openNew = () => {
    setEditingTemplate(null);
    setShowForm(true);
  };

  const handleFormSaved = () => {
    setShowForm(false);
    setEditingTemplate(null);
    load();
  };

  const filtered = templates.filter((t) => {
    if (filterTrigger && t.trigger_event !== filterTrigger) return false;
    if (filterStatus === "active" && !t.is_active) return false;
    if (filterStatus === "inactive" && t.is_active) return false;
    return true;
  });

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Email Templates</span>
          <small>Manage automated email templates for transactions and communications.</small>
        </div>
        <div className="listtab__toolbar">
          <select
            className="btn btn--secondary btn--sm"
            value={filterTrigger}
            onChange={(e) => setFilterTrigger(e.target.value)}
          >
            <option value="">All Triggers</option>
            {TRIGGER_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <select
            className="btn btn--secondary btn--sm"
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
          >
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </select>
          {!showForm ? (
            <button className="btn btn--primary btn--sm" onClick={openNew}>+ New Template</button>
          ) : null}
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : showForm ? (
          <EmailTemplateFormModal
            visible={showForm}
            onClose={() => { setShowForm(false); setEditingTemplate(null); }}
            onSave={handleFormSaved}
            initialData={editingTemplate}
          />
        ) : null}

        {!loading && !error ? (
          filtered.length === 0 && !showForm ? (
            <EmptyState
              title="No email templates"
              message="Create a template to automate email notifications."
            />
          ) : filtered.length > 0 ? (
            <table className="data-table">
              <thead>
                <tr>
                  <th style={{ width: "40%" }}>Subject</th>
                  <th style={{ width: "20%" }}>Trigger</th>
                  <th style={{ width: "15%" }}>Status</th>
                  <th style={{ width: "25%", textAlign: "right" }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((template) => (
                  <tr key={template.id}>
                    <td>{template.subject}</td>
                    <td>
                      <Badge variant="info">{formatTrigger(template.trigger_event)}</Badge>
                    </td>
                    <td>
                      <Badge variant={template.is_active ? "success" : "secondary"}>
                        {template.is_active ? "Active" : "Inactive"}
                      </Badge>
                    </td>
                    <td style={{ textAlign: "right" }}>
                      <button
                        className="btn btn--ghost btn--sm"
                        onClick={() => openEdit(template)}
                        style={{ marginRight: "4px" }}
                      >
                        Edit
                      </button>
                      <button
                        className="btn btn--danger btn--sm"
                        onClick={() => handleDelete(template.id)}
                        style={{ marginRight: "4px" }}
                      >
                        Delete
                      </button>
                      <button
                        className="btn btn--ghost btn--sm"
                        onClick={() => handleToggleActive(template)}
                      >
                        {template.is_active ? "Deactivate" : "Activate"}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null
        ) : null}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{filtered.length} Template(s)</span>
      </div>
    </div>
  );
}

function formatTrigger(event: string): string {
  return event.replace(/_/g, " ");
}

interface BadgeProps {
  variant?: "info" | "success" | "secondary" | "danger";
  children: React.ReactNode;
}

function Badge({ variant = "info", children }: BadgeProps) {
  return (
    <span
      className={`status-badge status-badge--${variant}`}
      role="badge"
      aria-label={typeof children === "string" ? children : undefined}
    >
      {children}
    </span>
  );
}

interface EmailTemplateFormModalProps {
  visible: boolean;
  onClose: () => void;
  onSave: () => void;
  initialData: EmailTemplate | null;
}

function EmailTemplateFormModal({ visible, onClose, onSave, initialData }: EmailTemplateFormModalProps) {
  const [subject, setSubject] = useState(initialData?.subject ?? "");
  const [bodyHtml, setBodyHtml] = useState(initialData?.body_html ?? "");
  const [bodyText, setBodyText] = useState(initialData?.body_text ?? "");
  const [triggerEvent, setTriggerEvent] = useState<TriggerEvent>(initialData?.trigger_event ?? "INVOICE_SENT");
  const [isPreviewMode, setIsPreviewMode] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (visible) {
      setSubject(initialData?.subject ?? "");
      setBodyHtml(initialData?.body_html ?? "");
      setBodyText(initialData?.body_text ?? "");
      setTriggerEvent(initialData?.trigger_event ?? "INVOICE_SENT");
      setError(null);
    }
  }, [visible, initialData]);

  const handleSubmit = async () => {
    if (!subject.trim()) {
      setError("Subject is required.");
      return;
    }
    if (!bodyHtml.trim()) {
      setError("HTML body is required.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const input = {
        subject: subject.trim(),
        body_html: bodyHtml.trim(),
        body_text: bodyText.trim(),
        trigger_event: triggerEvent,
        is_active: initialData?.is_active ?? true,
      };
      if (initialData) {
        await api.updateEmailTemplate(initialData.id, input);
      } else {
        await api.createEmailTemplate(input);
      }
      onSave();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save template.");
    } finally {
      setSaving(false);
    }
  };

  if (!visible) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" role="dialog" aria-modal="true" aria-label={initialData ? "Edit Template" : "New Template"} onClick={(e) => e.stopPropagation()}>
        <div className="modal__header">
          <h3>{initialData ? "Edit Template" : "New Template"}</h3>
          <button className="modal__close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="modal__body">
          {error && <p className="error-message" style={{ color: "var(--neg)", marginBottom: "12px" }}>{error}</p>}
          
          <div className="detail-grid" style={{ gridTemplateColumns: "180px 1fr", gap: "12px", marginBottom: "16px" }}>
            <label className="field__label">Subject</label>
            <input
              type="text"
              className="input"
              placeholder="e.g. Invoice #{{invoice_number}} - Thank you"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              style={{ padding: "8px 10px" }}
            />

            <label className="field__label">Trigger Event</label>
            <select
              className="input"
              value={triggerEvent}
              onChange={(e) => setTriggerEvent(e.target.value as TriggerEvent)}
              style={{ padding: "8px 10px" }}
            >
              {TRIGGER_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "8px" }}>
            <strong>Body</strong>
            <div style={{ display: "flex", gap: "8px" }}>
              <button
                className="btn btn--ghost btn--sm"
                onClick={() => setIsPreviewMode(true)}
              >
                Preview
              </button>
              <button
                className="btn btn--ghost btn--sm"
                onClick={() => setIsPreviewMode(false)}
              >
                Edit HTML
              </button>
            </div>
          </div>

          <div className="card" style={{ minHeight: "200px" }}>
            {isPreviewMode ? (
              <div
                className="email-preview"
                style={{ padding: "16px", border: "1px solid var(--border)", borderRadius: "4px", overflowX: "auto" }}
                dangerouslySetInnerHTML={{ __html: bodyHtml || "<em>No content</em>" }}
              />
            ) : (
              <textarea
                className="input"
                value={bodyHtml}
                onChange={(e) => setBodyHtml(e.target.value)}
                placeholder="<html><body>Your email content here...</body></html>"
                style={{ width: "100%", height: "200px", fontFamily: "monospace", padding: "8px", boxSizing: "border-box" }}
              />
            )}
          </div>

          <div className="detail-grid" style={{ gridTemplateColumns: "180px 1fr", gap: "12px", marginTop: "16px" }}>
            <label className="field__label">Plain Text Body</label>
            <textarea
              className="input"
              value={bodyText}
              onChange={(e) => setBodyText(e.target.value)}
              placeholder="Plain text version of the email..."
              style={{ width: "100%", height: "60px", padding: "8px", boxSizing: "border-box" }}
            />
          </div>

          <div style={{ display: "flex", gap: "8px", marginTop: "16px" }}>
            <button className="btn btn--primary btn--sm" onClick={handleSubmit} disabled={saving}>
              {saving ? "Saving..." : (initialData ? "Update" : "Create")}
            </button>
            <button className="btn btn--secondary btn--sm" onClick={onClose}>Cancel</button>
          </div>
        </div>
      </div>
    </div>
  );
}
