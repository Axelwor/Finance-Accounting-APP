import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { Button, EmptyState, ErrorState, FieldShell, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { CostCenter, CreateAllocationInput, CostCenterAllocation } from "../../types";

interface Props {
  tabId: string;
  initialTitle?: string;
}

function uid(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

const ALLOCATION_BASES = [
  { value: "HEADCOUNT", label: "Headcount" },
  { value: "SQUARE_FOOTAGE", label: "Square Footage" },
  { value: "REVENUE", label: "Revenue" },
  { value: "DIRECT_LABOR", label: "Direct Labor" },
  { value: "MACHINERY_HOURS", label: "Machinery Hours" },
  { value: "CUSTOM", label: "Custom" },
];

interface AllocationLineDraft {
  id: string;
  targetCostCenterId: number | "";
  allocationPercentage: string;
  allocationBasis: string;
}

/**
 * Cost Center Allocation form (US-094). Allocates costs from source to targets
 * with percentage breakdown. Percentage must sum to 100%.
 */
export function CostCenterAllocationForm({ tabId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [sourceCostCenters, setSourceCostCenters] = useState<CostCenter[]>([]);
  const [targetCostCenters, setTargetCostCenters] = useState<CostCenter[]>([]);
  const [sourceId, setSourceId] = useState<number | null>(null);
  const [existingAllocations, setExistingAllocations] = useState<CostCenterAllocation[]>([]);
  const [lines, setLines] = useState<AllocationLineDraft[]>([]);
  const [allocating, setAllocating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    void load();
  }, []);

  const load = async () => {
    try {
      const all = await api.listCostCenters();
      const active = all.filter((c) => c.is_active);
      setSourceCostCenters(active);
      setTargetCostCenters(active.filter((c) => !c.parent_id));
      if (active.length > 0 && !sourceId) {
        setSourceId(active[0].id);
      }
    } catch {
      /* ignore */
    }
  };

  const totalPercentage = useMemo(() => {
    return lines.reduce((sum, line) => sum + Number(line.allocationPercentage || 0), 0);
  }, [lines]);

  const addLine = () => {
    setLines([
      ...lines,
      { id: uid(), targetCostCenterId: "", allocationPercentage: "", allocationBasis: "CUSTOM" },
    ]);
  };

  const removeLine = (id: string) => {
    setLines(lines.filter((l) => l.id !== id));
  };

  const updateLine = (id: string, field: keyof AllocationLineDraft, value: string | number | "") => {
    setLines(lines.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
  };

  const handleSubmit = async () => {
    if (!sourceId) {
      setError("Please select a source cost center.");
      return;
    }
    if (totalPercentage !== 100) {
      setError(`Percentage must sum to 100% (currently ${totalPercentage.toFixed(1)}%).`);
      return;
    }
    if (lines.length === 0) {
      setError("Add at least one allocation line.");
      return;
    }
    if (new Set(lines.map((l) => l.targetCostCenterId)).size !== lines.length) {
      setError("Each target cost center can only be allocated once.");
      return;
    }

    setAllocating(true);
    setError(null);
    setSuccess(null);

    try {
      for (const line of lines) {
        if (line.targetCostCenterId === "" || !line.allocationPercentage) continue;
        const input: CreateAllocationInput = {
          source_cost_center_id: sourceId,
          target_cost_center_id: Number(line.targetCostCenterId),
          allocation_percentage: Number(line.allocationPercentage),
          allocation_basis: line.allocationBasis,
          is_active: true,
        };
        await api.createAllocation(sourceId, input);
      }
      setSuccess("Allocations created successfully.");
      await load();
      setTimeout(() => {
        void workbench.activate(tabId);
      }, 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create allocations.");
    } finally {
      setAllocating(false);
    }
  };

  if (error) {
    return (
      <div className="formtab">
        <div className="formtab__head">
          <h2>{initialTitle ?? "New Allocation"}</h2>
        </div>
        <div className="formtab__body">
          <ErrorState message={error} onRetry={() => setError(null)} />
        </div>
        <div className="formtab__footer">
          <Button variant="secondary" onClick={() => void workbench.activate(tabId)}>
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="formtab">
      <div className="formtab__head">
        <h2>{initialTitle ?? "Allocate Costs"}</h2>
        <small>Distribute costs from source to target centers by percentage</small>
      </div>

      <div className="formtab__body">
        {success ? (
          <EmptyState
            title="Success"
            message={success}
            action={<Button variant="primary" onClick={() => void workbench.activate(tabId)}>Done</Button>}
          />
        ) : (
          <>
            <div style={{ display: "grid", gap: 16 }}>
              <FieldShell label="Source Cost Center">
                <select
                  value={sourceId ?? ""}
                  onChange={(e) => setSourceId(e.target.value ? Number(e.target.value) : null)}
                  disabled={allocating}
                  style={{ width: "100%" }}
                >
                  <option value="">-- Select Source --</option>
                  {sourceCostCenters.map((cc) => (
                    <option key={cc.id} value={cc.id}>
                      {cc.code} — {cc.name}
                    </option>
                  ))}
                </select>
              </FieldShell>

              {sourceId && (
                <>
                  <FieldShell
                    label="Allocation Lines"
                    hint={`Total: ${totalPercentage.toFixed(1)}% (must equal 100%)`}
                    error={!lines.every((l) => l.targetCostCenterId && l.allocationPercentage) ? "All fields required." : undefined}
                  >
                    {lines.map((line, idx) => (
                      <div
                        key={line.id}
                        style={{
                          display: "grid",
                          gridTemplateColumns: "1fr 120px 1fr auto",
                          gap: 8,
                          alignItems: "center",
                          padding: 8,
                          backgroundColor: "#fafafa",
                          borderRadius: 4,
                          marginBottom: 8,
                        }}
                      >
                        <select
                          value={line.targetCostCenterId as string}
                          onChange={(e) => updateLine(line.id, "targetCostCenterId", e.target.value)}
                          disabled={allocating}
                          style={{ width: "100%" }}
                        >
                          <option value="">-- Target --</option>
                          {targetCostCenters
                            .filter((c) => c.id !== sourceId)
                            .map((cc) => (
                              <option key={cc.id} value={cc.id}>
                                {cc.code} — {cc.name}
                              </option>
                            ))}
                        </select>

                        <input
                          type="number"
                          placeholder="% %"
                          value={line.allocationPercentage}
                          onChange={(e) => updateLine(line.id, "allocationPercentage", e.target.value)}
                          disabled={allocating}
                          min="0"
                          max="100"
                          step="0.01"
                          style={{ width: "100%", textAlign: "right" }}
                        />

                        <select
                          value={line.allocationBasis}
                          onChange={(e) => updateLine(line.id, "allocationBasis", e.target.value)}
                          disabled={allocating}
                          style={{ width: "100%" }}
                        >
                          {ALLOCATION_BASES.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>

                        <button
                          type="button"
                          className="btn btn--danger btn--xs"
                          onClick={() => removeLine(line.id)}
                          disabled={allocating}
                        >
                          Remove
                        </button>
                      </div>
                    ))}
                    <Button variant="secondary" onClick={addLine} disabled={allocating}>
                      + Add Line
                    </Button>
                  </FieldShell>
                </>
              )}
            </div>
          </>
        )}
      </div>

      <div className="formtab__footer">
        <Button variant="secondary" onClick={() => void workbench.activate(tabId)}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={handleSubmit}
          disabled={allocating || !sourceId || totalPercentage !== 100 || lines.length === 0}
        >
          {allocating ? "Allocating..." : "Create Allocations"}
        </Button>
      </div>
    </div>
  );
}
