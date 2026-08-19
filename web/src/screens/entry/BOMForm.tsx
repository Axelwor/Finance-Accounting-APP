import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Item, BOM, BOMLineInput, ProductionCostType } from "../../types";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface Line {
  id: string;
  itemId: string;
  qty: string;
  unitCostCents: string;
  costType: ProductionCostType;
  description: string;
}

const COST_TYPES: ProductionCostType[] = ["material", "labor", "overhead"];

export function BOMForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [items, setItems] = useState<Item[]>([]);
  const [code, setCode] = useState(draftNumber("bom-entry"));
  const [name, setName] = useState("");
  const [finishedGoodItemId, setFinishedGoodItemId] = useState("");
  const [outputQty, setOutputQty] = useState("1");
  const [lines, setLines] = useState<Line[]>([
    { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0", costType: "material", description: "" },
  ]);
  const [existing, setExisting] = useState<BOM | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listItems().then(setItems).catch(() => {});
  }, []);

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    api.getBOM(id).then((bom) => {
      setExisting(bom);
      setCode(bom.code);
      setName(bom.name);
      setFinishedGoodItemId(String(bom.finished_good_item_id));
      setOutputQty(String(bom.output_qty));
      if (bom.lines && bom.lines.length > 0) {
        setLines(bom.lines.map((l) => ({
          id: String(l.id),
          itemId: String(l.item_id),
          qty: String(l.qty),
          unitCostCents: String(l.unit_cost_cents),
          costType: l.cost_type,
          description: l.description ?? "",
        })));
      }
    }).catch(() => {});
  }, [entryId]);

  const totalCents = lines.reduce(
    (sum, l) => sum + Math.round((parseFloat(l.qty) || 0) * (parseInt(l.unitCostCents) || 0)),
    0,
  );

  function setLine(id: string, field: keyof Line, value: string) {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
    workbench.markUnsaved(tabId, true);
  }

  function addLine() {
    setLines((prev) => [...prev, { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0", costType: "material", description: "" }]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!name.trim()) { setError("Name is required."); return; }
    if (!finishedGoodItemId) { setError("Finished good item is required."); return; }
    if (lines.every((l) => !l.itemId)) { setError("At least one BOM line is required."); return; }
    const lineInputs: BOMLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: parseFloat(l.qty) || 0,
        unit_cost_cents: parseInt(l.unitCostCents) || 0,
        cost_type: l.costType,
        description: l.description,
      }));
    setSaving(true);
    try {
      const bom = await api.createBOM({
        code: code.trim(),
        name: name.trim(),
        finished_good_item_id: Number(finishedGoodItemId),
        output_qty: parseFloat(outputQty) || 0,
        lines: lineInputs,
      });
      workbench.replaceDraft(tabId, bom.code, bom.status);
      setExisting(bom);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create BOM.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__title-row">
          <h2 className="entrytab__title">{existing ? existing.code : initialTitle ?? "New BOM"}</h2>
          {existing && <span className="kind-mark is-positive">{existing.status}</span>}
        </div>
        <div className="entrytab__detail">
          <div className="entrytab__detail-row">
            <label className="field">
              <span className="field__label">Code *</span>
              <input className="input" value={code} onChange={(e) => setCode(e.target.value)} disabled={isExisting} />
            </label>
            <label className="field">
              <span className="field__label">Name *</span>
              <input className="input" value={name} onChange={(e) => setName(e.target.value)} disabled={isExisting} />
            </label>
          </div>
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
              <span className="field__label">Output Qty *</span>
              <input className="input input--narrow" type="number" step="0.001" value={outputQty} onChange={(e) => setOutputQty(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <div className="entrytab__detail-title">BOM Lines *</div>
          <div className="detail-grid detail-grid--quote">
            <div className="detail-grid__head">
              <div>Item</div>
              <div>Type</div>
              <div>Qty</div>
              <div className="right">Unit Cost</div>
              <div className="right">Line Total</div>
              <div aria-hidden="true" />
            </div>
            {lines.map((line) => (
              <div className="detail-grid__row" key={line.id}>
                <div>
                  <select className="input" value={line.itemId} onChange={(e) => setLine(line.id, "itemId", e.target.value)} disabled={isExisting}>
                    <option value="">Choose item...</option>
                    {items.map((i) => (
                      <option key={i.id} value={i.id}>
                        {i.code} · {i.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <select className="input" value={line.costType} onChange={(e) => setLine(line.id, "costType", e.target.value)} disabled={isExisting}>
                    {COST_TYPES.map((ct) => (
                      <option key={ct} value={ct}>{ct}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <input className="input input--narrow" type="number" step="0.001" value={line.qty} onChange={(e) => setLine(line.id, "qty", e.target.value)} disabled={isExisting} />
                </div>
                <div>
                  <input className="input input--narrow right" type="number" value={line.unitCostCents} onChange={(e) => setLine(line.id, "unitCostCents", e.target.value)} disabled={isExisting} />
                </div>
                <div className="right">
                  {formatIDR(Math.round((parseFloat(line.qty) || 0) * (parseInt(line.unitCostCents) || 0)))}
                </div>
                <div>
                  {!isExisting && (
                    <button type="button" className="detail-grid__remove" onClick={() => removeLine(line.id)} aria-label="Remove line">
                      ×
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
          {!isExisting && (
            <Button
              variant="text"
              onClick={addLine}
              style={{ marginTop: 8 }}
            >
              + Add line
            </Button>
          )}

          <div className="entrytab__total">
            <span className="entrytab__total-label">Estimated Standard Cost</span>
            <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
