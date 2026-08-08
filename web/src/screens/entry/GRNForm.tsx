import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { PurchaseOrderListItem, Item, GRNLineInput } from "../../types";

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
}

export function GRNForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [purchaseOrders, setPurchaseOrders] = useState<PurchaseOrderListItem[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [poId, setPoId] = useState("");
  const [grnDate, setGrnDate] = useState(new Date().toISOString().slice(0, 10));
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([{ id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0" }]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listPurchaseOrders("CONFIRMED").then(setPurchaseOrders).catch(() => {});
    api.listPurchaseOrders("PARTIALLY_RECEIVED").then((partial) => {
      setPurchaseOrders((prev) => [...prev, ...partial]);
    }).catch(() => {});
    api.listItems().then(setItems).catch(() => {});
  }, []);

  const totalCents = lines.reduce(
    (sum, l) => sum + Math.round((parseFloat(l.qty) || 0) * (parseInt(l.unitCostCents) || 0)),
    0,
  );

  function setLine(id: string, field: keyof Line, value: string) {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
    workbench.markUnsaved(tabId, true);
  }

  function addLine() {
    setLines((prev) => [...prev, { id: crypto.randomUUID(), itemId: "", qty: "1", unitCostCents: "0" }]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!poId) { setError("Purchase order is required."); return; }
    if (lines.every((l) => !l.itemId)) { setError("At least one item line is required."); return; }
    const inputLines: GRNLineInput[] = lines.filter((l) => l.itemId).map((l) => ({
      item_id: Number(l.itemId),
      qty: parseFloat(l.qty) || 0,
      unit_cost_cents: parseInt(l.unitCostCents) || 0,
    }));
    if (inputLines.some((l) => l.qty <= 0)) { setError("Quantity must be positive."); return; }
    setSaving(true);
    try {
      const grn = await api.createGRN({
        purchase_order_id: Number(poId),
        grn_date: grnDate,
        notes: notes || undefined,
        lines: inputLines,
      });
      workbench.replaceDraft(tabId, grn.number, grn.status);
      workbench.markUnsaved(tabId, false);
    } catch (err: any) {
      setError(err?.message || "Failed to create GRN.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Goods Received Note"}</div>
          <div className="entrytab__header-number">{isExisting ? initialTitle : draftNumber("grn-entry")}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Purchase Order *</span>
              <select className="input" value={poId} onChange={(e) => setPoId(e.target.value)} disabled={isExisting}>
                <option value="">Choose PO...</option>
                {purchaseOrders.map((po) => (
                  <option key={po.id} value={po.id}>
                    {po.number} · {po.supplier_name ?? `#${po.supplier_id}`}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">GRN Date *</span>
              <input className="input" type="date" value={grnDate} onChange={(e) => setGrnDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={isExisting} />
          </label>

          <div className="entrytab__detail-title">Received items *</div>
          <div className="detail-grid detail-grid--quote">
            <div className="detail-grid__head">
              <div>Item</div>
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
            <button type="button" className="btn btn--ghost" onClick={addLine} style={{ marginTop: 8 }}>
              + Add line
            </button>
          )}

          <div className="entrytab__total">
            <span className="entrytab__total-label">Total (Dr Inventory / Cr Payable)</span>
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
