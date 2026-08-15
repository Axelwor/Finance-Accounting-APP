import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { Button, EmptyState, ErrorState, FieldShell, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { draftNumber } from "../../workbench/modules";
import type { CostCenter, CostCenterListItem, CreateCostCenterInput, UpdateCostCenterInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

function uid(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

const CENTER_TYPE_OPTIONS = [
  { value: "COST", label: "Cost Center" },
  { value: "PROFIT", label: "Profit Center" },
  { value: "INVESTMENT", label: "Investment Center" },
];

/**
 * Cost Center form (US-094). Creates/edits cost centers with hierarchy support.
 */
export function CostCenterForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [centerType, setCenterType] = useState<CreateCostCenterInput["center_type"]>("COST");
  const [parentId, setParentId] = useState<number | null>(null);
  const [isActive, setIsActive] = useState(true);
  const [description, setDescription] = useState("");
  const [costCenters, setCostCenters] = useState<CostCenterListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const draftNo = useMemo(() => (entryId ? "" : draftNumber("cost-center-entry")), [entryId]);
  const title = initialTitle ?? (entryId ? "Edit Cost Center" : `CC-${draftNo || "NEW"}`);

  useEffect(() => {
    void load();
    if (entryId) {
      void loadEntry();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entryId]);

  const load = async () => {
    try {
      const data = await api.listCostCenters();
      setCostCenters(data.filter((c) => c.is_active));
    } catch {
      /* ignore */
    }
  };

  const loadEntry = async () => {
    try {
      const cc = await api.getCostCenter(Number(entryId));
      setCode(cc.code);
      setName(cc.name);
      setCenterType(cc.center_type);
      setParentId(cc.parent_id);
      setIsActive(cc.is_active);
      setDescription(cc.description ?? "");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cost center.");
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (!code.trim() || !name.trim()) {
      setError("Code and name are required.");
      return;
    }
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      const input: CreateCostCenterInput = {
        code: code.trim(),
        name: name.trim(),
        center_type: centerType,
        parent_id: parentId,
        is_active: isActive,
        description: description.trim() || undefined,
      };

      if (entryId) {
        const updateInput: UpdateCostCenterInput = {
          name: name.trim(),
          center_type: centerType,
          parent_id: parentId,
          is_active: isActive,
          description: description.trim() || undefined,
        };
        await api.updateCostCenter(Number(entryId), updateInput);
        setSuccess("Cost center updated successfully.");
      } else {
        await api.createCostCenter(input);
        setSuccess("Cost center created successfully.");
      }
      setTimeout(() => {
        void workbench.activate(tabId);
      }, 1000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save cost center.");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading && entryId) {
    return <LoadingState label="Loading cost center..." />;
  }

  return (
    <div className="formtab">
      <div className="formtab__head">
        <h2>{title}</h2>
        <small>Create or edit a cost center with hierarchical relationship</small>
      </div>

      <div className="formtab__body">
        {error ? (
          <ErrorState message={error} onRetry={() => setError(null)} />
        ) : success ? (
          <EmptyState
            title="Success"
            message={success}
            action={<Button variant="primary" onClick={() => void workbench.activate(tabId)}>Done</Button>}
          />
        ) : null}

        <div style={{ display: "grid", gap: 16 }}>
          <FieldShell
            label="Code"
            error={error && !code.trim() ? "Code is required." : undefined}
          >
            <input
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="e.g., CC-HQ"
              disabled={submitting || !!entryId}
              style={{ width: "100%" }}
            />
          </FieldShell>

          <FieldShell label="Name">
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Headquarters"
              disabled={submitting}
              style={{ width: "100%" }}
            />
          </FieldShell>

          <FieldShell label="Type">
            <select
              value={centerType}
              onChange={(e) => setCenterType(e.target.value as typeof centerType)}
              disabled={submitting}
              style={{ width: "100%" }}
            >
              {CENTER_TYPE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </FieldShell>

          <FieldShell label="Parent Cost Center" hint="Leave empty for root-level center">
            <select
              value={parentId ?? ""}
              onChange={(e) => setParentId(e.target.value ? Number(e.target.value) : null)}
              disabled={submitting}
              style={{ width: "100%" }}
            >
              <option value="">-- None (Root) --</option>
              {costCenters.map((cc) => (
                <option key={cc.id} value={cc.id}>
                  {cc.code} — {cc.name}
                </option>
              ))}
            </select>
          </FieldShell>

          <FieldShell label="Active" hint="Deactivate to hide from active lists">
            <select
              value={isActive ? "true" : "false"}
              onChange={(e) => setIsActive(e.target.value === "true")}
              disabled={submitting}
              style={{ width: "100%" }}
            >
              <option value="true">Yes</option>
              <option value="false">No</option>
            </select>
          </FieldShell>

          <FieldShell label="Description">
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description..."
              disabled={submitting}
              style={{ width: "100%", minHeight: 60, resize: "vertical" }}
            />
          </FieldShell>
        </div>
      </div>

      <div className="formtab__footer">
        <Button variant="secondary" onClick={() => void workbench.activate(tabId)}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSubmit} disabled={submitting}>
          {submitting ? "Saving..." : entryId ? "Update" : "Create"} Cost Center
        </Button>
      </div>
    </div>
  );
}
