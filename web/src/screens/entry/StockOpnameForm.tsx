import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Item, StockOpname, StockOpnameLineInput } from "../../types";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface Line {
  id: string;
  itemId: string;
  countedQty: string;
  unitCostCents: string;
  reason: string;
}

export function StockOpnameForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [items, setItems] = useState<Item[]>([]);
  const [opnameDate, setOpnameDate] = useState(new Date().toISOString().slice(0, 10));
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([
    { id: crypto.randomUUID(), itemId: "", countedQty: "0", unitCostCents: "0", reason: "" },
  ]);
  const [existing, setExisting] = useState<StockOpname | null>(null);
  const [saving, setSaving] = useState(false);
  const [approving, setApproving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listItems().then(setItems).catch(() => {});
  }, []);

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    api.getStockOpname(id).then((opn) => {
      setExisting(opn);
      setOpnameDate(opn.opname_date);
      setNotes(opn.notes ?? "");
    }).catch(() => {});
  }, [entryId]);

  const totalAdjustmentCents = lines.reduce(
    (sum, l) => sum + Math.round((parseFloat(l.countedQty) || 0) * (parseInt(l.unitCostCents) || 0)),
    0,
  );

  function setLine(id: string, field: keyof Line, value: string) {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
    workbench.markUnsaved(tabId, true);
  }

  function addLine() {
    setLines((prev) => [...prev, { id: crypto.randomUUID(), itemId: "", countedQty: "0", unitCostCents: "0", reason: "" }]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (lines.every((l) => !l.itemId)) { setError("At least one item line is required."); return; }
    const inputLines: StockOpnameLineInput[] = lines.filter((l) => l.itemId).map((l) => ({
      item_id: Number(l.itemId),
      counted_qty: parseFloat(l.countedQty) || 0,
      unit_cost_cents: parseInt(l.unitCostCents) || 0,
      reason: l.reason || undefined,
    }));
    if (inputLines.some((l) => l.counted_qty < 0)) { setError("Counted qty must be >= 0."); return; }
    setSaving(true);
    try {
      const opn = await api.createStockOpname({
        opname_date: opnameDate,
        notes: notes || undefined,
        lines: inputLines,
      });
      workbench.replaceDraft(tabId, opn.number, opn.status);
      workbench.markUnsaved(tabId, false);
      setExisting(opn);
    } catch (err: any) {
      setError(err?.message || "Failed to create stock opname.");
    } finally {
      setSaving(false);
    }
  }

  async function handleApprove() {
    if (!existing) return;
    setError("");
    setApproving(true);
    try {
      const opn = await api.approveStockOpname(existing.id);
      setExisting(opn);
      workbench.replaceDraft(tabId, opn.number, opn.status);
    } catch (err: any) {
      setError(err?.message || "Failed to approve stock opname.");
    } finally {
      setApproving(false);
    }
  }

  const canApprove = existing && existing.status === "COUNTED";

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Stock Opname"}</div>
          <div className="entrytab__header-number">{isExisting ? existing?.number : draftNumber("stock-opname-entry")}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Opname Date *</span>
              <input className="input" type="date" value={opnameDate} onChange={(e) => setOpnameDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={isExisting} />
          </label>

          {!isExisting && (
            <>
              <div className="entrytab__detail-title">Counted items *</div>
              <div className="detail-grid detail-grid--quote">
                <div className="detail-grid__head">
                  <div>Item</div>
                  <div>Counted Qty</div>
                  <div className="right">Unit Cost</div>
                  <div>Reason</div>
                  <div aria-hidden="true" />
                </div>
                {lines.map((line) => (
                  <div className="detail-grid__row" key={line.id}>
                    <div>
                      <select className="input" value={line.itemId} onChange={(e) => setLine(line.id, "itemId", e.target.value)}>
                        <option value="">Choose item...</option>
                        {items.filter((i) => i.item_type === "goods").map((i) => (
                          <option key={i.id} value={i.id}>
                            {i.code} · {i.name}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <input className="input input--narrow" type="number" step="0.001" value={line.countedQty} onChange={(e) => setLine(line.id, "countedQty", e.target.value)} />
                    </div>
                    <div>
                      <input className="input input--narrow right" type="number" value={line.unitCostCents} onChange={(e) => setLine(line.id, "unitCostCents", e.target.value)} />
                    </div>
                    <div>
                      <input className="input" type="text" value={line.reason} placeholder="optional" onChange={(e) => setLine(line.id, "reason", e.target.value)} />
                    </div>
                    <div>
                      <button type="button" className="detail-grid__remove" onClick={() => removeLine(line.id)} aria-label="Remove line">
                        ×
                      </button>
                    </div>
                  </div>
                ))}
              </div>
              <Button
                variant="text"
                onClick={addLine}
                style={{ marginTop: 8 }}
              >
                + Add line
              </Button>

              <div className="entrytab__total">
                <span className="entrytab__total-label">Total Adjustment (counted × cost)</span>
                <span className="entrytab__total-value">{formatIDR(totalAdjustmentCents)}</span>
              </div>
            </>
          )}

          {isExisting && existing && (
            <>
              <div className="entrytab__detail-title">Counted items</div>
              <div className="detail-grid detail-grid--quote">
                <div className="detail-grid__head">
                  <div>Item</div>
                  <div className="right">System Qty</div>
                  <div className="right">Counted Qty</div>
                  <div className="right">Diff</div>
                  <div className="right">Unit Cost</div>
                  <div className="right">Adjustment</div>
                </div>
                {existing.lines.map((line) => (
                  <div className="detail-grid__row" key={line.id}>
                    <div>{line.item_code ?? `#${line.item_id}`} · {line.item_name ?? ""}</div>
                    <div className="right">{line.system_qty}</div>
                    <div className="right">{line.counted_qty}</div>
                    <div className="right">{line.diff_qty}</div>
                    <div className="right">{formatIDR(line.unit_cost_cents)}</div>
                    <div className="right">{formatIDR(line.adjustment_cents)}</div>
                  </div>
                ))}
              </div>
              <div className="entrytab__total">
                <span className="entrytab__total-label">Total Adjustment</span>
                <span className="entrytab__total-value">{formatIDR(existing.total_adjustment_cents)}</span>
              </div>
              {existing.journal_entry_id ? (
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Journal Entry</span>
                  <span className="entrytab__total-value">#{existing.journal_entry_id}</span>
                </div>
              ) : null}
            </>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Save"}</span>
            </button>
          )}
          {canApprove && (
            <button type="button" className="action-rail__btn action-rail__btn--primary" onClick={handleApprove} disabled={approving}>
              <span>{approving ? "Approving..." : "Approve & Post"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
