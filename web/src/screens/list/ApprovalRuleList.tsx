import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type {
  ApprovalWorkflow,
  ApprovalEntityType,
  ApprovalApproverRole,
} from "../../types";
import { parseRupiahToCents, formatIDRFromCents } from "../../lib/format";
import { Button } from "../../components/m3";

/** Values accepted by the approval gate (CheckAmount callers in the backend). */
const ENTITY_TYPE_OPTIONS: Array<{ value: ApprovalEntityType; label: string }> = [
  { value: "invoice", label: "Sales Invoice" },
  { value: "purchase_order", label: "Purchase Order" },
  { value: "supplier_invoice", label: "Supplier Invoice" },
  { value: "credit_note", label: "Credit Note" },
  { value: "journal_entry", label: "Journal Entry" },
];

/** Roles accepted by validateWorkflow on the backend. */
const APPROVER_ROLE_OPTIONS: Array<{ value: ApprovalApproverRole; label: string }> = [
  { value: "admin", label: "Admin" },
  { value: "accountant", label: "Accountant" },
  { value: "manager", label: "Manager" },
];

const ENTITY_TYPE_LABEL: Record<string, string> = Object.fromEntries(
  ENTITY_TYPE_OPTIONS.map((o) => [o.value, o.label]),
);

export function ApprovalRuleList() {
  const [items, setItems] = useState<ApprovalWorkflow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  /** When set, the form prefills an existing workflow — save re-posts it (upsert). */
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [entityType, setEntityType] = useState<ApprovalEntityType>("invoice");
  const [minAmount, setMinAmount] = useState("");
  const [approverRole, setApproverRole] = useState<ApprovalApproverRole>("accountant");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listApprovalWorkflows();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load approval rules.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const resetForm = () => {
    setEntityType("invoice");
    setMinAmount("");
    setApproverRole("accountant");
    setFormError(null);
    setShowForm(false);
    setEditingId(null);
  };

  const handleSave = async () => {
    if (minAmount.trim() === "") {
      setFormError("Minimum amount is required (use 0 to apply to any amount).");
      return;
    }
    const minAmountCents = parseRupiahToCents(minAmount);
    if (minAmountCents < 0) {
      setFormError("Minimum amount must be >= 0.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      // No PUT endpoint: POST upserts per entity_type.
      await api.createApprovalWorkflow({
        entity_type: entityType,
        min_amount_cents: minAmountCents,
        approver_role: approverRole,
      });
      resetForm();
      await load();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to save approval rule.");
    } finally {
      setSaving(false);
    }
  };

  const editRule = (rule: ApprovalWorkflow) => {
    setEditingId(rule.id);
    setEntityType((ENTITY_TYPE_LABEL[rule.entity_type] ? rule.entity_type : "invoice") as ApprovalEntityType);
    setMinAmount(String(Math.floor(rule.min_amount_cents / 100)));
    setApproverRole(
      (APPROVER_ROLE_OPTIONS.some((o) => o.value === rule.approver_role)
        ? rule.approver_role
        : "accountant") as ApprovalApproverRole,
    );
    setShowForm(true);
  };

  const deleteRule = async (id: number) => {
    if (!confirm("Delete this approval rule?")) return;
    try {
      await api.deleteApprovalWorkflow(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete rule.");
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Approval Rules</span>
          <small>Configure approval workflows by entity type and amount</small>
        </div>
        <div className="listtab__toolbar">
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Reload
          </Button>
          {!showForm ? (
            <Button
              variant="filled"
              size="sm"
              onClick={() => setShowForm(true)}
            >
              + New Rule
            </Button>
          ) : null}
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : showForm ? (
          <div className="card" style={{ padding: "16px", marginBottom: "12px" }}>
            <h3 style={{ marginTop: 0 }}>{editingId ? "Edit Rule" : "New Rule"}</h3>
            <div className="detail-grid" style={{ gridTemplateColumns: "160px 1fr", gap: "8px 12px", alignItems: "center" }}>
              <label className="field__label">Entity Type</label>
              <select
                className="input"
                value={entityType}
                onChange={(e) => setEntityType(e.target.value as ApprovalEntityType)}
                style={{ padding: "6px 10px" }}
              >
                {ENTITY_TYPE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>

              <label className="field__label">Minimum Amount (Rp)</label>
              <input
                type="text"
                inputMode="numeric"
                className="input"
                value={minAmount}
                onChange={(e) => setMinAmount(e.target.value)}
                placeholder="e.g. 5.000.000 (0 = any amount)"
                style={{ padding: "6px 10px" }}
              />

              <label className="field__label">Approver Role</label>
              <select
                className="input"
                value={approverRole}
                onChange={(e) => setApproverRole(e.target.value as ApprovalApproverRole)}
                style={{ padding: "6px 10px" }}
              >
                {APPROVER_ROLE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
            {formError ? (
              <p style={{ color: "var(--md-sys-color-error)", margin: "8px 0 0" }}>{formError}</p>
            ) : null}
            <div style={{ display: "flex", gap: "8px", marginTop: "12px" }}>
              <Button
                variant="filled"
                size="sm"
                disabled={saving}
                onClick={() => void handleSave()}
              >
                {saving ? "Saving..." : "Save"}
              </Button>
              <Button
                variant="outlined"
                size="sm"
                onClick={resetForm}
              >
                Cancel
              </Button>
            </div>
          </div>
        ) : null}

        {!loading && !error ? (
          items.length === 0 && !showForm ? (
            <EmptyState
              title="No approval rules"
              message="Create rules to define when transactions require approval."
            />
          ) : items.length > 0 ? (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Entity Type</th>
                  <th>Min Amount</th>
                  <th>Approver Role</th>
                  <th>Status</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((rule) => (
                  <tr key={rule.id}>
                    <td>{ENTITY_TYPE_LABEL[rule.entity_type] ?? rule.entity_type}</td>
                    <td style={{ fontFamily: "var(--md-ref-typeface-plain)" }}>
                      {formatIDRFromCents(rule.min_amount_cents)}
                    </td>
                    <td style={{ textTransform: "capitalize" }}>{rule.approver_role}</td>
                    <td>
                      {rule.is_active === false ? (
                        <span className="badge">Inactive</span>
                      ) : (
                        <span className="badge badge--success">Active</span>
                      )}
                    </td>
                    <td>
                      <Button
                        variant="outlined"
                        size="sm"
                        onClick={() => editRule(rule)}
                      >
                        Edit
                      </Button>{" "}
                      <Button
                        variant="outlined"
                        size="sm"
                        danger
                        onClick={() => deleteRule(rule.id)}
                      >
                        Delete
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null
        ) : null}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Rule(s)</span>
      </div>
    </div>
  );
}
