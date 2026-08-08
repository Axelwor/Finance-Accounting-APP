import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { Supplier, Item, PurchaseOrderLineInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface Line {
  id: string;
  itemId: string;
  qty: string;
  unitPriceCents: string;
  discountCents: string;
  taxRate: string;
}

const emptyLine = (): Line => ({
  id: crypto.randomUUID(),
  itemId: "",
  qty: "1",
  unitPriceCents: "0",
  discountCents: "0",
  taxRate: "0",
});

export function PurchaseOrderForm({ tabId, entryId, initialTitle }: Props) {
  const { replaceDraft, markUnsaved, getNested } = useWorkbench();
  const isExisting = !!entryId;

  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [supplierId, setSupplierId] = useState("");
  const [orderDate, setOrderDate] = useState(new Date().toISOString().slice(0, 10));
  const [notes, setNotes] = useState("");
  const [lines, setLines] = useState<Line[]>([emptyLine()]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listSuppliers().then(setSuppliers).catch(() => {});
    api.listItems().then(setItems).catch(() => {});
  }, []);

  useEffect(() => {
    if (isExisting && entryId) {
      api.getPurchaseOrder(Number(entryId)).then((po) => {
        setSupplierId(String(po.supplier_id));
        setOrderDate(po.order_date);
        setNotes(po.notes || "");
        setLines(
          po.lines.map((l) => ({
            id: String(l.id),
            itemId: String(l.item_id),
            qty: l.qty,
            unitPriceCents: String(l.unit_price_cents),
            discountCents: String(l.discount_cents),
            taxRate: String(l.tax_rate || 0),
          })),
        );
      }).catch(() => {});
    }
  }, [entryId, isExisting]);

  const totalCents = lines.reduce(
    (sum, l) => sum + lineTotal(parseFloat(l.qty) || 0, parseInt(l.unitPriceCents) || 0, parseInt(l.discountCents) || 0),
    0,
  );

  function setLine(id: string, field: keyof Line, value: string) {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, [field]: value } : l)));
    markUnsaved(tabId, true);
  }

  function addLine() {
    setLines((prev) => [...prev, emptyLine()]);
  }

  function removeLine(id: string) {
    setLines((prev) => (prev.length > 1 ? prev.filter((l) => l.id !== id) : prev));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!supplierId) {
      setError("Supplier is required.");
      return;
    }
    if (lines.length === 0 || lines.every((l) => !l.itemId)) {
      setError("At least one item line is required.");
      return;
    }

    const inputLines: PurchaseOrderLineInput[] = lines
      .filter((l) => l.itemId)
      .map((l) => ({
        item_id: Number(l.itemId),
        qty: parseFloat(l.qty) || 0,
        unit_price_cents: parseInt(l.unitPriceCents) || 0,
        discount_cents: parseInt(l.discountCents) || 0,
        tax_rate: parseFloat(l.taxRate) || 0,
      }));

    if (inputLines.some((l) => l.qty <= 0)) {
      setError("Quantity must be positive.");
      return;
    }

    setSaving(true);
    try {
      const po = await api.createPurchaseOrder({
        supplier_id: Number(supplierId),
        order_date: orderDate,
        notes: notes || undefined,
        lines: inputLines,
      });
      replaceDraft(tabId, po.number, po.status);
      markUnsaved(tabId, false);
    } catch (err: any) {
      setError(err?.message || "Failed to save purchase order.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Purchase Order"}</div>
          <div className="entrytab__header-number">{isExisting ? initialTitle : draftNumber("purchase-order-entry")}</div>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Supplier *</span>
              <select className="input" value={supplierId} onChange={(e) => setSupplierId(e.target.value)} disabled={isExisting}>
                <option value="">Choose supplier...</option>
                {suppliers.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.code} · {s.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">Order Date *</span>
              <input className="input" type="date" value={orderDate} onChange={(e) => setOrderDate(e.target.value)} disabled={isExisting} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea className="input" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={isExisting} />
          </label>

          <div className="entrytab__detail-title">Item lines *</div>
          <div className="detail-grid detail-grid--quote">
            <div className="detail-grid__head">
              <div>Item</div>
              <div>Qty</div>
              <div className="right">Unit Price</div>
              <div className="right">Discount</div>
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
                  <input className="input input--narrow right" type="number" value={line.unitPriceCents} onChange={(e) => setLine(line.id, "unitPriceCents", e.target.value)} disabled={isExisting} />
                </div>
                <div>
                  <input className="input input--narrow right" type="number" value={line.discountCents} onChange={(e) => setLine(line.id, "discountCents", e.target.value)} disabled={isExisting} />
                </div>
                <div className="right">
                  {formatIDR(lineTotal(parseFloat(line.qty) || 0, parseInt(line.unitPriceCents) || 0, parseInt(line.discountCents) || 0))}
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
            <span className="entrytab__total-label">Total</span>
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

function lineTotal(qty: number, unitPriceCents: number, discountCents: number): number {
  return Math.round(qty * unitPriceCents) - discountCents;
}
