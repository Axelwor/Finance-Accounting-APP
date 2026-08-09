import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Item, ProductionJob, ProductionCostType } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const COST_TYPES: ProductionCostType[] = ["material", "labor", "overhead"];

const JOB_STATUS_TONE: Record<string, string> = {
  OPEN: "is-muted",
  IN_PROGRESS: "is-info",
  COMPLETED: "is-positive",
  CANCELLED: "is-negative",
};

export function ProductionJobForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [items, setItems] = useState<Item[]>([]);
  const [finishedGoodItemId, setFinishedGoodItemId] = useState("");
  const [targetQty, setTargetQty] = useState("1");
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10));
  const [existing, setExisting] = useState<ProductionJob | null>(null);

  // Cost panel state.
  const [costType, setCostType] = useState<ProductionCostType>("material");
  const [costItemId, setCostItemId] = useState("");
  const [costQty, setCostQty] = useState("1");
  const [costUnitCents, setCostUnitCents] = useState("0");
  const [costDescription, setCostDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [addingCost, setAddingCost] = useState(false);
  const [completing, setCompleting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listItems().then(setItems).catch(() => {});
  }, []);

  const loadJob = async (id: number) => {
    const job = await api.getProductionJob(id);
    setExisting(job);
    setFinishedGoodItemId(String(job.finished_good_item_id));
    setTargetQty(String(job.target_qty));
    setStartDate(job.start_date);
  };

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    loadJob(id).catch(() => {});
  }, [entryId]);

  const canAddCost = !!existing && (existing.status === "OPEN" || existing.status === "IN_PROGRESS");
  const canComplete = !!existing && (existing.status === "OPEN" || existing.status === "IN_PROGRESS");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!finishedGoodItemId) { setError("Finished good item is required."); return; }
    setSaving(true);
    try {
      const job = await api.createProductionJob({
        finished_good_item_id: Number(finishedGoodItemId),
        target_qty: parseFloat(targetQty) || 0,
        start_date: startDate,
      });
      workbench.replaceDraft(tabId, job.number, job.status);
      setExisting(job);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create production job.");
    } finally {
      setSaving(false);
    }
  }

  async function handleAddCost(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!existing) return;
    if (costType === "material" && !costItemId) { setError("Material cost requires an item."); return; }
    setAddingCost(true);
    try {
      await api.addProductionJobCost(existing.id, {
        cost_type: costType,
        item_id: costType === "material" ? Number(costItemId) : undefined,
        description: costDescription,
        qty: parseFloat(costQty) || 0,
        unit_cost_cents: parseInt(costUnitCents) || 0,
      });
      await loadJob(existing.id);
      setCostDescription("");
      setCostQty("1");
      setCostUnitCents("0");
      setCostItemId("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add cost.");
    } finally {
      setAddingCost(false);
    }
  }

  async function handleComplete() {
    setError("");
    if (!existing) return;
    setCompleting(true);
    try {
      const job = await api.completeProductionJob(existing.id, {});
      setExisting(job);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to complete job.");
    } finally {
      setCompleting(false);
    }
  }

  const costLineTotal = Math.round((parseFloat(costQty) || 0) * (parseInt(costUnitCents) || 0));

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__title-row">
          <h2 className="entrytab__title">{existing ? existing.number : initialTitle ?? "New Production Job"}</h2>
          {existing && <span className={`kind-mark ${JOB_STATUS_TONE[existing.status] ?? "is-muted"}`}>{existing.status}</span>}
        </div>
        <div className="entrytab__detail">
          <div className="entrytab__detail-row">
            <label className="field">
              <span className="field__label">Finished Good Item *</span>
              <select className="input" value={finishedGoodItemId} onChange={(e) => setFinishedGoodItemId(e.target.value)} disabled={isExisting}>
                <option value="">Choose item...</option>
                {items.map((i) => (
                  <option key={i.id} value={i.id}>
                    {i.code} · {i.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">Target Qty *</span>
              <input className="input input--narrow" type="number" step="0.001" value={targetQty} onChange={(e) => setTargetQty(e.target.value)} disabled={isExisting} />
            </label>
            <label className="field">
              <span className="field__label">Start Date *</span>
              <input className="input" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          {existing && (
            <>
              <div className="entrytab__detail-title">Cost Summary</div>
              <div className="entrytab__detail-row">
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Material</span>
                  <span className="entrytab__total-value">{formatIDR(existing.total_material_cents)}</span>
                </div>
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Labor</span>
                  <span className="entrytab__total-value">{formatIDR(existing.total_labor_cents)}</span>
                </div>
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Overhead</span>
                  <span className="entrytab__total-value">{formatIDR(existing.total_overhead_cents)}</span>
                </div>
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Total WIP</span>
                  <span className="entrytab__total-value">{formatIDR(existing.total_cost_cents)}</span>
                </div>
              </div>

              {existing.costs && existing.costs.length > 0 && (
                <>
                  <div className="entrytab__detail-title">Posted Costs</div>
                  <div className="ledger-table">
                    <div className="ledger-table__row ledger-table__row--head">
                      <span>Type</span>
                      <span>Item / Description</span>
                      <span className="right">Qty</span>
                      <span className="right">Unit Cost</span>
                      <span className="right">Total</span>
                      <span className="right">JE</span>
                    </div>
                    {existing.costs.map((c) => (
                      <div className="ledger-table__row" key={c.id}>
                        <span><span className="kind-mark is-muted">{c.cost_type}</span></span>
                        <span className="ledger-table__memo">
                          {c.item_name ? `${c.item_name}` : c.description ?? "—"}
                          {c.description && c.item_name ? ` · ${c.description}` : ""}
                        </span>
                        <span className="ledger-table__amount right">{c.qty ?? "—"}</span>
                        <span className="ledger-table__amount right">{formatIDR(c.unit_cost_cents)}</span>
                        <span className="ledger-table__amount right">{formatIDR(c.total_cents)}</span>
                        <span className="ledger-table__amount right">{c.journal_entry_id ? `#${c.journal_entry_id}` : "—"}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {canAddCost && (
                <>
                  <div className="entrytab__detail-title">Add Cost</div>
                  <div className="entrytab__detail-row">
                    <label className="field">
                      <span className="field__label">Cost Type *</span>
                      <select className="input" value={costType} onChange={(e) => setCostType(e.target.value as ProductionCostType)}>
                        {COST_TYPES.map((ct) => (
                          <option key={ct} value={ct}>{ct}</option>
                        ))}
                      </select>
                    </label>
                    {costType === "material" && (
                      <label className="field">
                        <span className="field__label">Item (raw material) *</span>
                        <select className="input" value={costItemId} onChange={(e) => setCostItemId(e.target.value)}>
                          <option value="">Choose item...</option>
                          {items.map((i) => (
                            <option key={i.id} value={i.id}>
                              {i.code} · {i.name}
                            </option>
                          ))}
                        </select>
                      </label>
                    )}
                    <label className="field">
                      <span className="field__label">Qty</span>
                      <input className="input input--narrow" type="number" step="0.001" value={costQty} onChange={(e) => setCostQty(e.target.value)} />
                    </label>
                    <label className="field">
                      <span className="field__label">Unit Cost (cents)</span>
                      <input className="input input--narrow" type="number" value={costUnitCents} onChange={(e) => setCostUnitCents(e.target.value)} disabled={costType === "material"} />
                    </label>
                  </div>
                  <label className="field">
                    <span className="field__label">Description</span>
                    <input className="input" value={costDescription} onChange={(e) => setCostDescription(e.target.value)} placeholder="Optional note..." />
                  </label>
                  <div className="entrytab__total">
                    <span className="entrytab__total-label">Cost Line Total</span>
                    <span className="entrytab__total-value">{formatIDR(costLineTotal)}</span>
                  </div>
                  <button type="button" className="btn btn--ghost" onClick={handleAddCost} disabled={addingCost} style={{ marginTop: 8 }}>
                    {addingCost ? "Posting..." : "+ Post Cost"}
                  </button>
                  {costType === "material" && (
                    <small style={{ display: "block", marginTop: 4, color: "var(--muted, #888)" }}>
                      Material cost consumes stock from inventory (Dr 1303 WIP / Cr 1301 Inventory). Unit cost is resolved by the costing method (FIFO / moving average).
                    </small>
                  )}
                  {costType !== "material" && (
                    <small style={{ display: "block", marginTop: 4, color: "var(--muted, #888)" }}>
                      {`${costType === "labor" ? "Labor" : "Overhead"} cost posts Dr 1303 WIP / Cr 1101 Cash.`}
                    </small>
                  )}
                </>
              )}

              {existing.status === "COMPLETED" && (
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Completion Journal</span>
                  <span className="entrytab__total-value">#{existing.journal_entry_id}</span>
                </div>
              )}
            </>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
          {canComplete && (
            <button type="button" className="action-rail__btn action-rail__btn--primary" onClick={handleComplete} disabled={completing}>
              <span>{completing ? "Completing..." : "Complete Job"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
