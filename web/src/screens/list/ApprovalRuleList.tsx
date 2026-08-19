import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRule, CreateApprovalRuleInput } from "../../types";
import { Button } from "../../components/m3";

const ENTITY_TYPE_LABEL: Record<string, string> = {
  "sales-invoice": "Sales Invoice",
  "purchase-order": "Purchase Order",
  "supplier-invoice": "Supplier Invoice",
  "journal-entry": "Journal Entry",
};

export function ApprovalRuleList() {
  const [items, setItems] = useState<ApprovalRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [entityType, setEntityType] = useState("sales-invoice");
  const [minAmount, setMinAmount] = useState(0);
  const [approverAccountId, setApproverAccountId] = useState<number | "">("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getApprovalRules();
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
    setEntityType("sales-invoice");
    setMinAmount(0);
    setApproverAccountId("");
    setFormError(null);
    setShowForm(false);
    setEditingId(null);
  };

  const handleSave = async () => {
    if (!entityType.trim()) {
      setFormError("Entity type is required.");
      return;
    }
    if (minAmount < 0) {
      setFormError("Minimum amount must be >= 0.");
      return;
    }
    if (approverAccountId === "" || approverAccountId === undefined) {
      setFormError("Approver account is required.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      const input: CreateApprovalRuleInput = {
        entity_type: entityType,
        min_amount_cents: minAmount * 100,
        approver_account_id: Number(approverAccountId),
      };
      if (editingId) {
        await api.updateApprovalRule(editingId, input);
      } else {
        await api.createApprovalRule(input);
      }
      resetForm();
      await load();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to save approval rule.");
    } finally {
      setSaving(false);
    }
  };

  const editRule = (rule: ApprovalRule) => {
    setEditingId(rule.id);
    setEntityType(rule.entity_type);
    setMinAmount(Math.floor(rule.min_amount_cents / 100));
    setApproverAccountId(rule.approver_account_id);
    setShowForm(true);
  };

  const deleteRule = async (id: number) => {
    if (!confirm("Delete this approval rule?")) return;
    try {
      await api.deleteApprovalRule(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete rule.");
    }
  };

  const formatAmount = (cents: number) => {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(cents / 100);
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
                onChange={(e) => setEntityType(e.target.value)}
                style={{ padding: "6px 10px" }}
              >
                <option value="sales-invoice">Sales Invoice</option>
                <option value="purchase-order">Purchase Order</option>
                <option value="supplier-invoice">Supplier Invoice</option>
                <option value="journal-entry">Journal Entry</option>
              </select>

              <label className="field__label">Minimum Amount</label>
              <input
                type="number"
                className="input"
                value={minAmount}
                onChange={(e) => setMinAmount(Number(e.target.value))}
                placeholder="e.g. 5000000"
                style={{ padding: "6px 10px" }}
                min="0"
              />

              <label className="field__label">Approver Account ID</label>
              <input
                type="number"
                className="input"
                value={approverAccountId}
                onChange={(e) => setApproverAccountId(e.target.value === "" ? "" : Number(e.target.value))}
                placeholder="e.g. 1"
                style={{ padding: "6px 10px" }}
                min="1"
              />
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
                {saving ? "Saving..." : editingId ? "Update" : "Create"}
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
                  <th>Approver ID</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((rule) => (
                  <tr key={rule.id}>
                    <td>{ENTITY_TYPE_LABEL[rule.entity_type] ?? rule.entity_type}</td>
                    <td style={{ fontFamily: "var(--md-ref-typeface-plain)" }}>{formatAmount(rule.min_amount_cents)}</td>
                    <td>{rule.approver_account_id}</td>
                    <td>
                      <Button
                        variant="outlined"
                        size="sm"
                        onClick={() => editRule(rule)}
                      >
                        Edit
                      </Button>
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
