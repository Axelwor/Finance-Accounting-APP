import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState, Combobox } from "../../components/ui";
import { api } from "../../api";
import type { ApprovalRule, BackendAccount } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const ENTITY_TYPES = [
  { value: "invoice", label: "Sales Invoice" },
  { value: "purchase-order", label: "Purchase Order" },
  { value: "supplier-invoice", label: "Supplier Invoice" },
  { value: "journal-entry", label: "Journal Entry" },
  { value: "payment", label: "Payment" },
] as const;

export function ApprovalRuleForm({ tabId, entryId, initialTitle }: Props) {
  const { markUnsaved, replaceDraft, closeTab } = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null && entryId !== "";

  // Form fields
  const [entityType, setEntityType] = useState<(typeof ENTITY_TYPES)[number]["value"]>("invoice");
  const [minAmountCentsDisplay, setMinAmountCentsDisplay] = useState("");
  const [approverAccountId, setApproverAccountId] = useState<string | null>(null);
  const [approverName, setApproverName] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [description, setDescription] = useState("");

  // Masters
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [loading, setLoading] = useState(isEdit);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [savedRule, setSavedRule] = useState<ApprovalRule | null>(null);

  useEffect(() => {
    void api.listBackendAccounts().then(setAccounts).catch(() => {});
  }, []);

  useEffect(() => {
    if (!isEdit) return;
    let cancelled = false;
    setLoading(true);
    api
      .getApprovalRule(Number(entryId))
      .then((rule) => {
        if (cancelled) return;
        setEntityType(rule.entity_type as typeof entityType);
        setMinAmountCentsDisplay(centsInput(rule.min_amount_cents));
        setApproverAccountId(rule.approver_account_id ? String(rule.approver_account_id) : null);
        setApproverName(rule.approver_name || "");
        setIsActive(rule.is_active);
        setDescription(rule.description || "");
        setSavedRule(rule);
        setLoading(false);
        markUnsaved(tabId, false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load approval rule.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [entryId, isEdit, markUnsaved, tabId]);

  useEffect(() => {
    markUnsaved(tabId, true);
  }, [tabId, entityType, minAmountCentsDisplay, approverAccountId, isActive, description, markUnsaved]);

  const buildInput = (): CreateApprovalRuleInput => ({
    entity_type: entityType,
    min_amount_cents: parseCents(minAmountCentsDisplay),
    approver_account_id: approverAccountId ? Number(approverAccountId) : null,
    is_active: isActive,
    description: description.trim(),
  });

  const handleSubmit = async () => {
    setSaving(true);
    setError("");
    try {
      if (isEdit) {
        await api.updateApprovalRule(Number(entryId), buildInput());
        const updated = await api.getApprovalRule(Number(entryId));
        setSavedRule(updated);
        closeTab(tabId);
      } else {
        const created = await api.createApprovalRule(buildInput());
        setSavedRule(created);
        closeTab(tabId);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save approval rule.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <LoadingState label="Loading approval rule..." />;
  if (error) return <FormError message={error} />;

  const selectedApprover = accounts.find((a) => String(a.id) === approverAccountId) ?? null;

  return (
    <div className="entrytab">
      <div className="entrytab__header">
        <h2 className="entrytab__title">{isEdit ? "Edit Approval Rule" : "New Approval Rule"}</h2>
      </div>
      <div className="entrytab__body">
        <div className="form-group">
          <label htmlFor="entity_type">Entity Type *</label>
          <select
            id="entity_type"
            className="form-control"
            value={entityType}
            onChange={(e) => setEntityType(e.target.value as typeof entityType)}
            disabled={saving}
          >
            {ENTITY_TYPES.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label htmlFor="min_amount_cents">Minimum Amount (IDR) *</label>
          <input
            id="min_amount_cents"
            type="text"
            className="form-control"
            value={minAmountCentsDisplay}
            onChange={(e) => setMinAmountCentsDisplay(e.target.value)}
            placeholder="0 for any amount"
            disabled={saving}
          />
          <small className="form-hint">Enter 0 to require approval for any amount.</small>
        </div>
        <div className="form-group">
          <label>Approver Account *</label>
          <Combobox
            value={approverAccountId}
            onChange={(val, option) => {
              setApproverAccountId(val as string | null);
              setApproverName(option?.label || "");
            }}
            loadOptions={async (search: string) => {
              const all = accounts.filter((a) => !a.is_group);
              if (!search.trim()) return all.slice(0, 50);
              const q = search.toLowerCase();
              return all.filter((a) => a.name.toLowerCase().includes(q) || (a.code || "").toLowerCase().includes(q)).slice(0, 50);
            }}
            placeholder="Select approver account"
            disabled={saving}
          />
          {selectedApprover && (
            <small className="form-hint">Selected: {selectedApprover.code} - {selectedApprover.name}</small>
          )}
        </div>
        <div className="form-group">
          <label htmlFor="description">Description</label>
          <textarea
            id="description"
            className="form-control"
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={saving}
          />
        </div>
        <div className="form-group">
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={isActive}
              onChange={(e) => setIsActive(e.target.checked)}
              disabled={saving}
            />
            Active
          </label>
        </div>
        <div className="entrytab__footer">
          <button type="button" className="btn" onClick={() => closeTab(tabId)} disabled={saving}>Cancel</button>
          <button type="button" className="btn btn--primary" onClick={handleSubmit} disabled={saving || !entityType || !minAmountCentsDisplay || !approverAccountId}>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}

function centsInput(cents: number): string {
  if (cents === 0) return "0";
  return new Intl.NumberFormat("id-ID").format(cents / 100);
}

function parseCents(input: string): number {
  if (!input.trim()) return 0;
  const normalized = input.replace(/[^\d.]/g, "");
  const value = parseFloat(normalized);
  return Math.round(value * 100);
}

type CreateApprovalRuleInput = {
  entity_type: string;
  min_amount_cents: number;
  approver_account_id: number | null;
  is_active: boolean;
  description: string;
};
