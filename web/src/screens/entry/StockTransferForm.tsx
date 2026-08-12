import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Item, StockTransfer, StockTransferLineInput } from "../../types";

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
  description: string;
}

export function StockTransferForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [items, setItems] = useState<Item[]>([]);
  const [transferDate, setTransferDate] = useState(new Date().toISOString().slice(0, 10));
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([
    { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0", description: "" },
  ]);
  const [existing, setExisting] = useState<StockTransfer | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listItems().then(setItems).catch(() => {});
  }, []);

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    api.getStockTransfer(id).then((trf) => {
      setExisting(trf);
      setTransferDate(trf.transfer_date);
      setNotes(trf.notes ?? "");
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
    setLines((prev) => [...prev, { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0", description: "" }]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (lines.every((l) => !l.itemId)) { setError("At least one item line is required."); return; }
    const inputLines: StockTransferLineInput[] = lines.filter((l) => l.itemId).map((l) => ({
      item_id: Number(l.itemId),
      qty: parseFloat(l.qty) || 0,
      unit_cost_cents: parseInt(l.unitCostCents) || 0,
      description: l.description || undefined,
    }));
    if (inputLines.some((l) => l.qty <= 0)) { setError("Quantity must be positive."); return; }
    setSaving(true);
    try {
      const trf = await api.createStockTransfer({
        transfer_date: transferDate,
        notes: notes || undefined,
        lines: inputLines,
      });
      workbench.replaceDraft(tabId, trf.number, trf.status);
      workbench.markUnsaved(tabId, false);
      setExisting(trf);
    } catch (err: any) {
      setError(err?.message || "Failed to create stock transfer.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Stock Transfer"}</div>
          <div className="entrytab__header-number">{isExisting ? existing?.number : draftNumber("stock-transfer-entry")}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Transfer Date *</span>
              <input className="input" type="date" value={transferDate} onChange={(e) => setTransferDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={isExisting} />
          </label>

          {!isExisting && (
            <>
              <div className="entrytab__detail-title">Items to transfer *</div>
              <div className="detail-grid detail-grid--quote">
                <div className="detail-grid__head" role="row">
                  <div role="columnheader">Item</div>
                  <div role="columnheader">Qty</div>
                  <div role="columnheader" className="right">Unit Cost</div>
                  <div role="columnheader">Description</div>
                  <div aria-hidden="true" />
                </div>
                {lines.map((line) => (
                  <div className="detail-grid__row" key={line.id} role="row">
                    <div role="cell">
                      <select
                        className="input"
                        value={line.itemId}
                        onChange={(e) => setLine(line.id, "itemId", e.target.value)}
                        aria-label={`Item for line ${line.id}`}
                      >
                        <option value="">Choose item...</option>
                        {items.filter((i) => i.item_type === "goods").map((i) => (
                          <option key={i.id} value={i.id}>
                            {i.code} · {i.name}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div role="cell">
                      <input
                        className="input input--narrow"
                        type="number"
                        step="0.001"
                        value={line.qty}
                        onChange={(e) => setLine(line.id, "qty", e.target.value)}
                        aria-label={`Quantity for line ${line.id}`}
                      />
                    </div>
                    <div role="cell">
                      <input
                        className="input input--narrow right"
                        type="number"
                        value={line.unitCostCents}
                        onChange={(e) => setLine(line.id, "unitCostCents", e.target.value)}
                        aria-label={`Unit cost for line ${line.id}`}
                      />
                    </div>
                    <div role="cell">
                      <input
                        className="input"
                        type="text"
                        value={line.description}
                        placeholder="optional"
                        onChange={(e) => setLine(line.id, "description", e.target.value)}
                        aria-label={`Description for line ${line.id}`}
                      />
                    </div>
                    <div role="cell">
                      <button
                        type="button"
                        className="detail-grid__remove"
                        onClick={() => removeLine(line.id)}
                        aria-label={`Remove line ${line.id}`}
                      >
                        ×
                      </button>
                    </div>
                  </div>
                ))}
              </div>
              <button type="button" className="btn btn--ghost" onClick={addLine} style={{ marginTop: 8 }}>
                + Add line
              </button>

              <div className="entrytab__total">
                <span className="entrytab__total-label">Total Value (no journal posted)</span>
                <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
              </div>
            </>
          )}

          {isExisting && existing && (
            <>
              <div className="entrytab__detail-title">Transferred items</div>
              <div className="detail-grid detail-grid--quote">
                <div className="detail-grid__head">
                  <div>Item</div>
                  <div className="right">Qty</div>
                  <div className="right">Unit Cost</div>
                  <div>Description</div>
                </div>
                {existing.lines.map((line) => (
                  <div className="detail-grid__row" key={line.id}>
                    <div>{line.item_code ?? `#${line.item_id}`} · {line.item_name ?? ""}</div>
                    <div className="right">{line.qty}</div>
                    <div className="right">{formatIDR(line.unit_cost_cents)}</div>
                    <div>{line.description ?? ""}</div>
                  </div>
                ))}
              </div>
            </>
          )}
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
