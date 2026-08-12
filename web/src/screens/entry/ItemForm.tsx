import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { AmountField, FieldShell, FormError, LoadingState, SelectField } from "../../components/ui";
import { api } from "../../api";
import { parseAmountInput } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type {
  BackendAccount,
  CreateItemInput,
  Item,
  ItemCostingMethod,
  SupplierListItem,
} from "../../types";

interface Props {
  tabId: string;
  /** Present when an existing item row was opened -> edit/view mode. */
  entryId?: string | number;
  initialTitle?: string;
}

/** ERP item form state (all inputs kept as strings). */
interface ItemFormState {
  code: string;
  name: string;
  itemType: "" | "goods" | "service";
  uom: string;
  saleUom: string;
  purchaseUom: string;
  description: string;
  descriptionLong: string;
  costingMethod: "" | ItemCostingMethod;
  inventoryAccountId: string;
  cogsAccountId: string;
  saleAccountId: string;
  weightGrams: string;
  volumeCc: string;
  salePrice: string;
  purchasePrice: string;
  openingQty: string;
  openingCost: string;
  supplierId: string;
  reorderPoint: string;
  reorderQty: string;
  leadTimeDays: string;
  abc: "" | "A" | "B" | "C";
  category: string;
  brand: string;
  barcode: string;
}

type FieldErrors = Partial<Record<keyof ItemFormState, string>>;

const EMPTY_FORM: ItemFormState = {
  code: "",
  name: "",
  itemType: "",
  uom: "pcs",
  saleUom: "",
  purchaseUom: "",
  description: "",
  descriptionLong: "",
  costingMethod: "",
  inventoryAccountId: "",
  cogsAccountId: "",
  saleAccountId: "",
  weightGrams: "",
  volumeCc: "",
  salePrice: "",
  purchasePrice: "",
  openingQty: "",
  openingCost: "",
  supplierId: "",
  reorderPoint: "",
  reorderQty: "",
  leadTimeDays: "",
  abc: "",
  category: "",
  brand: "",
  barcode: "",
};

/** Backend numerics arrive as text (NUMERIC::text); normalize to a plain string. */
function numStr(value: number | string | null | undefined): string {
  if (value === null || value === undefined || value === "") return "";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? String(parsed) : "";
}

function formFromItem(item: Item): ItemFormState {
  return {
    ...EMPTY_FORM,
    code: item.code,
    name: item.name,
    itemType: item.item_type,
    uom: item.uom ?? item.unit ?? "pcs",
    saleUom: item.sale_uom ?? "",
    purchaseUom: item.purchase_uom ?? "",
    descriptionLong: item.description_long ?? "",
    costingMethod: item.costing_method ?? "",
    inventoryAccountId: item.inventory_account_id ? String(item.inventory_account_id) : "",
    cogsAccountId: item.cogs_account_id ? String(item.cogs_account_id) : "",
    saleAccountId: item.sale_account_id ? String(item.sale_account_id) : "",
    weightGrams: numStr(item.weight_grams),
    volumeCc: numStr(item.volume_cc),
    // GET /items does not return prices; the edit-mode effect prefills the
    // sale price from the item's "Umum" price-list entry instead.
    supplierId: item.preferred_supplier_id ? String(item.preferred_supplier_id) : "",
    reorderPoint: numStr(item.reorder_point),
    reorderQty: numStr(item.reorder_qty),
    leadTimeDays: numStr(item.lead_time_days),
    abc: item.abc_classification ?? "",
    category: item.category ?? "",
    brand: item.brand ?? "",
    barcode: item.barcode ?? "",
  };
}

/** Whole IDR amount (typed digits) -> cents, matching the rest of the app. */
function toCents(raw: string): number {
  return parseAmountInput(raw) * 100;
}

function toNumber(raw: string): number | undefined {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

/** True when a filled numeric field is invalid (not a finite number >= 0). */
function invalidQty(raw: string): boolean {
  const value = toNumber(raw);
  return raw.trim() !== "" && (value === undefined || value < 0);
}

/**
 * Item master-data form (migrations 000005 + 000033 ERP columns).
 *
 * Create mode posts to POST /items via api.createItem. Edit mode is detected
 * from `entryId`: the item is loaded from GET /items and shown read-only
 * (the backend has no item update endpoint yet) with a Deactivate action.
 */
export function ItemForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null;

  const [form, setForm] = useState<ItemFormState>(EMPTY_FORM);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [existing, setExisting] = useState<Item | null>(null);
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [suppliers, setSuppliers] = useState<SupplierListItem[]>([]);
  const [loading, setLoading] = useState(isEdit);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isGoods = form.itemType === "goods";
  const locked = isEdit || saved;

  // Lookup data for the FK dropdowns (accounts + suppliers).
  useEffect(() => {
    api.listBackendAccounts().then(setAccounts);
    api.listSuppliers().then(setSuppliers);
  }, []);

  // Edit mode: load the existing item into the form.
  useEffect(() => {
    if (!isEdit) return;
    let cancelled = false;
    (async () => {
      const items = await api.listItems({ include_inactive: true });
      if (cancelled) return;
      const found = items.find((item) => item.id === Number(entryId));
      if (!found) {
        setError("This item could not be found (it may belong to another book).");
        setLoading(false);
        return;
      }
      setExisting(found);
      const loaded = formFromItem(found);
      // GET /items does not include prices; prefill the sale price from the
      // active "Umum" price-list entry.
      const prices = await api.listItemPrices(found.id);
      if (cancelled) return;
      const umum = prices.find((price) => price.is_active && price.price_list_name === "Umum");
      if (umum) loaded.salePrice = String(Math.round(umum.unit_price_cents / 100));
      setForm(loaded);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [isEdit, entryId]);

  const update = <K extends keyof ItemFormState>(key: K, value: ItemFormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    if (!locked) workbench.markUnsaved(tabId, true);
  };

  const accountOptions = useMemo(() => {
    const active = accounts
      .filter((account) => !account.is_group && account.is_active)
      .sort((a, b) => a.code.localeCompare(b.code));
    const byGroup = (reportGroup: string) => {
      const filtered = active.filter((account) => account.report_group === reportGroup);
      // Fall back to all detail accounts when the report group is empty or
      // named differently, so the picker is never unusable.
      const list = filtered.length > 0 ? filtered : active;
      return list.map((account) => ({ value: String(account.id), label: `${account.code} · ${account.name}` }));
    };
    return {
      inventory: byGroup("asset"),
      cogs: byGroup("expense"),
      revenue: byGroup("revenue"),
    };
  }, [accounts]);

  const supplierOptions = useMemo(
    () => suppliers.map((s) => ({ value: String(s.id), label: `${s.code} · ${s.name}` })),
    [suppliers],
  );

  function validate(): FieldErrors {
    const next: FieldErrors = {};
    if (!form.code.trim()) next.code = "Item code is required.";
    if (!form.name.trim()) next.name = "Item name is required.";
    if (!form.itemType) next.itemType = "Choose goods or service — this drives costing.";
    if (!form.uom.trim()) next.uom = "Base UoM is required.";

    if (isGoods) {
      if (!form.costingMethod) next.costingMethod = "Costing method is required for goods.";
      if (!form.inventoryAccountId) next.inventoryAccountId = "Inventory account is required for goods.";
      if (!form.cogsAccountId) next.cogsAccountId = "COGS account is required for goods.";
      if (toCents(form.salePrice) <= 0) next.salePrice = "Sale price must be greater than zero.";
    } else if (form.itemType === "service" && form.salePrice && toCents(form.salePrice) <= 0) {
      next.salePrice = "Sale price must be greater than zero.";
    }
    // Purchase price is not persisted server-side yet — optional, but must be
    // positive when entered.
    if (form.purchasePrice && toCents(form.purchasePrice) <= 0) {
      next.purchasePrice = "Purchase price must be greater than zero.";
    }

    if (invalidQty(form.weightGrams)) next.weightGrams = "Weight must be zero or more.";
    if (invalidQty(form.volumeCc)) next.volumeCc = "Volume must be zero or more.";
    if (invalidQty(form.reorderPoint)) next.reorderPoint = "Reorder point must be zero or more.";
    if (invalidQty(form.reorderQty)) next.reorderQty = "Reorder qty must be zero or more.";
    if (invalidQty(form.leadTimeDays)) next.leadTimeDays = "Lead time must be zero or more.";
    if (invalidQty(form.openingQty)) next.openingQty = "Opening qty must be zero or more.";
    if (invalidQty(form.openingCost)) next.openingCost = "Opening cost must be zero or more.";
    return next;
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    if (locked) return;
    const fieldErrors = validate();
    setErrors(fieldErrors);
    if (Object.keys(fieldErrors).length > 0) {
      setError("Please fix the highlighted fields.");
      return;
    }
    if (form.itemType === "") return; // flagged by validate(); narrows the type below

    const leadTime = toNumber(form.leadTimeDays);
    const input: CreateItemInput = {
      code: form.code,
      name: form.name,
      item_type: form.itemType,
      uom: form.uom,
      costing_method: isGoods ? (form.costingMethod as ItemCostingMethod) : undefined,
      inventory_account_id: isGoods && form.inventoryAccountId ? Number(form.inventoryAccountId) : undefined,
      cogs_account_id: isGoods && form.cogsAccountId ? Number(form.cogsAccountId) : undefined,
      sale_account_id: form.saleAccountId ? Number(form.saleAccountId) : undefined,
      sale_uom: form.saleUom,
      purchase_uom: form.purchaseUom,
      barcode: form.barcode,
      brand: form.brand,
      category: form.category,
      description: form.description,
      description_long: form.descriptionLong,
      weight_grams: toNumber(form.weightGrams),
      volume_cc: toNumber(form.volumeCc),
      sale_price_cents: form.salePrice ? toCents(form.salePrice) : undefined,
      purchase_price_cents: form.purchasePrice ? toCents(form.purchasePrice) : undefined,
      reorder_point: toNumber(form.reorderPoint),
      reorder_qty: toNumber(form.reorderQty),
      lead_time_days: leadTime === undefined ? undefined : Math.round(leadTime),
      preferred_supplier_id: form.supplierId ? Number(form.supplierId) : undefined,
      abc_classification: form.abc || undefined,
      opening_balance_qty: toNumber(form.openingQty),
      opening_balance_cost_cents: form.openingCost ? toCents(form.openingCost) : undefined,
    };

    setSaving(true);
    try {
      const created = await api.createItem(input);
      setSaved(true);
      workbench.replaceDraft(tabId, created.code, "ACTIVE");
      workbench.markUnsaved(tabId, false);
    } catch (err: any) {
      setError(err?.message || "Failed to create the item.");
    } finally {
      setSaving(false);
    }
  };

  const handleDeactivate = async () => {
    if (!existing) return;
    if (!window.confirm(`Deactivate item ${existing.code}? It will no longer be selectable in transactions.`)) return;
    setSaving(true);
    setError(null);
    try {
      await api.deactivateItem(existing.id);
      const updated = { ...existing, is_active: false };
      setExisting(updated);
      workbench.replaceDraft(tabId, updated.code, "INACTIVE");
    } catch (err: any) {
      setError(err?.message || "Failed to deactivate the item.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <LoadingState label="Loading item..." />;
  }

  const status = isEdit ? (existing?.is_active ? "ACTIVE" : "INACTIVE") : saved ? "ACTIVE" : "DRAFT";
  const headerLabel = isEdit ? `Item ${existing?.code ?? initialTitle ?? ""}` : "Inventory Item";

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{headerLabel}</span>
          <span className={`entrytab__status ${status === "DRAFT" ? "entrytab__status--draft" : "entrytab__status--posted"}`}>
            {status}
          </span>
          <span className="entrytab__number">{isEdit ? existing?.code ?? "" : draftNumber("inventory-item")}</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          {isEdit && (
            <p style={{ margin: 0, fontSize: "var(--text-xs)", color: "var(--ink-muted)" }}>
              Read-only — the item update endpoint is not available yet. Use Deactivate on the action rail
              if this item is no longer in use.
            </p>
          )}

          {/* 1 — General */}
          <fieldset className="entrytab__section" disabled={locked}>
            <legend className="entrytab__section-title">General</legend>
            <FieldShell label="Item Code *" error={errors.code}>
              <input
                className="input"
                value={form.code}
                onChange={(e) => update("code", e.target.value)}
                placeholder="e.g. ITM-0001"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Item Name *" error={errors.name}>
              <input
                className="input"
                value={form.name}
                onChange={(e) => update("name", e.target.value)}
                placeholder="Item name"
                disabled={locked}
              />
            </FieldShell>
            <SelectField
              label="Item Type *"
              value={form.itemType}
              onChange={(value) => update("itemType", value as ItemFormState["itemType"])}
              options={[
                { value: "goods", label: "Goods — stocked, costed inventory" },
                { value: "service", label: "Service — not stocked" },
              ]}
              placeholder="Choose type..."
              error={errors.itemType}
            />
            <FieldShell label="Base UoM *" hint="Unit all stock is counted in." error={errors.uom}>
              <input
                className="input"
                value={form.uom}
                onChange={(e) => update("uom", e.target.value)}
                placeholder="e.g. pcs"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Sale UoM">
              <input
                className="input"
                value={form.saleUom}
                onChange={(e) => update("saleUom", e.target.value)}
                placeholder="Defaults to base UoM"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Purchase UoM">
              <input
                className="input"
                value={form.purchaseUom}
                onChange={(e) => update("purchaseUom", e.target.value)}
                placeholder="Defaults to base UoM"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Description">
              <input
                className="input"
                value={form.description}
                onChange={(e) => update("description", e.target.value)}
                placeholder="Short description"
                disabled={locked}
              />
            </FieldShell>
            <div style={{ gridColumn: "1 / -1" }}>
              <FieldShell label="Description (Long)">
                <textarea
                  className="input"
                  rows={2}
                  value={form.descriptionLong}
                  onChange={(e) => update("descriptionLong", e.target.value)}
                  placeholder="Detailed description shown on documents"
                  disabled={locked}
                />
              </FieldShell>
            </div>
          </fieldset>

          {/* 2 — Inventory & Costing */}
          <fieldset className="entrytab__section" disabled={locked}>
            <legend className="entrytab__section-title">Inventory &amp; Costing</legend>
            {isGoods ? (
              <>
                <SelectField
                  label="Costing Method *"
                  value={form.costingMethod}
                  onChange={(value) => update("costingMethod", value as ItemFormState["costingMethod"])}
                  options={[
                    { value: "fifo", label: "FIFO" },
                    { value: "moving_average", label: "Moving Average" },
                    { value: "specific", label: "Specific Identification" },
                  ]}
                  placeholder="Choose method..."
                  error={errors.costingMethod}
                />
                <SelectField
                  label="Inventory Account *"
                  value={form.inventoryAccountId}
                  onChange={(value) => update("inventoryAccountId", value)}
                  options={accountOptions.inventory}
                  placeholder="Choose account..."
                  error={errors.inventoryAccountId}
                />
                <SelectField
                  label="COGS Account *"
                  value={form.cogsAccountId}
                  onChange={(value) => update("cogsAccountId", value)}
                  options={accountOptions.cogs}
                  placeholder="Choose account..."
                  error={errors.cogsAccountId}
                />
                <SelectField
                  label="Revenue Account"
                  value={form.saleAccountId}
                  onChange={(value) => update("saleAccountId", value)}
                  options={accountOptions.revenue}
                  placeholder="Choose account..."
                />
                <FieldShell label="Opening Balance Qty" hint="Opening stock is posted via Stock Opname after creation.">
                  <input
                    className="input"
                    type="number"
                    min="0"
                    step="any"
                    value={form.openingQty}
                    onChange={(e) => update("openingQty", e.target.value)}
                    placeholder="0"
                    disabled={locked}
                  />
                </FieldShell>
                <AmountField
                  label="Opening Balance Cost"
                  value={form.openingCost}
                  onChange={(value) => update("openingCost", value)}
                  placeholder="0"
                />
              </>
            ) : (
              <p style={{ gridColumn: "1 / -1", margin: 0, fontSize: "var(--text-xs)", color: "var(--ink-muted)" }}>
                Services are not stocked and carry no inventory, COGS, or costing method.
              </p>
            )}
            <FieldShell label="Weight (grams)" error={errors.weightGrams}>
              <input
                className="input"
                type="number"
                min="0"
                step="any"
                value={form.weightGrams}
                onChange={(e) => update("weightGrams", e.target.value)}
                placeholder="0"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Volume (cc)" error={errors.volumeCc}>
              <input
                className="input"
                type="number"
                min="0"
                step="any"
                value={form.volumeCc}
                onChange={(e) => update("volumeCc", e.target.value)}
                placeholder="0"
                disabled={locked}
              />
            </FieldShell>
          </fieldset>

          {/* 3 — Pricing */}
          <fieldset className="entrytab__section" disabled={locked}>
            <legend className="entrytab__section-title">Pricing</legend>
            <AmountField
              label={isGoods ? "Sale Price *" : "Sale Price"}
              value={form.salePrice}
              onChange={(value) => update("salePrice", value)}
              hint="Saved as the default “Umum” price list entry."
              placeholder="0"
              error={errors.salePrice}
            />
            <AmountField
              label="Purchase Price"
              value={form.purchasePrice}
              onChange={(value) => update("purchasePrice", value)}
              hint="Reference cost for purchasing — not persisted server-side yet."
              placeholder="0"
              error={errors.purchasePrice}
            />
          </fieldset>

          {/* 4 — Supply */}
          <fieldset className="entrytab__section" disabled={locked}>
            <legend className="entrytab__section-title">Supply</legend>
            <SelectField
              label="Preferred Supplier"
              value={form.supplierId}
              onChange={(value) => update("supplierId", value)}
              options={supplierOptions}
              placeholder={suppliers.length > 0 ? "Choose supplier..." : "No suppliers yet"}
            />
            <FieldShell label="Lead Time (days)" error={errors.leadTimeDays}>
              <input
                className="input"
                type="number"
                min="0"
                step="1"
                value={form.leadTimeDays}
                onChange={(e) => update("leadTimeDays", e.target.value)}
                placeholder="0"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Reorder Point" hint="Stock level that triggers a purchase suggestion." error={errors.reorderPoint}>
              <input
                className="input"
                type="number"
                min="0"
                step="any"
                value={form.reorderPoint}
                onChange={(e) => update("reorderPoint", e.target.value)}
                placeholder="0"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Reorder Qty" error={errors.reorderQty}>
              <input
                className="input"
                type="number"
                min="0"
                step="any"
                value={form.reorderQty}
                onChange={(e) => update("reorderQty", e.target.value)}
                placeholder="0"
                disabled={locked}
              />
            </FieldShell>
          </fieldset>

          {/* 5 — Classification */}
          <fieldset className="entrytab__section" disabled={locked}>
            <legend className="entrytab__section-title">Classification</legend>
            <SelectField
              label="ABC Classification"
              value={form.abc}
              onChange={(value) => update("abc", value as ItemFormState["abc"])}
              options={[
                { value: "A", label: "A — high value" },
                { value: "B", label: "B — medium value" },
                { value: "C", label: "C — low value" },
              ]}
              placeholder="Not classified"
            />
            <FieldShell label="Category">
              <input
                className="input"
                value={form.category}
                onChange={(e) => update("category", e.target.value)}
                placeholder="e.g. Bahan Baku"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Brand">
              <input
                className="input"
                value={form.brand}
                onChange={(e) => update("brand", e.target.value)}
                placeholder="Brand name"
                disabled={locked}
              />
            </FieldShell>
            <FieldShell label="Barcode">
              <input
                className="input"
                value={form.barcode}
                onChange={(e) => update("barcode", e.target.value)}
                placeholder="EAN / UPC / internal barcode"
                disabled={locked}
              />
            </FieldShell>
          </fieldset>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button
            type="submit"
            className="action-rail__btn action-rail__btn--primary"
            disabled={locked || saving}
            title={isEdit ? "Item update API is not available yet" : undefined}
          >
            <DiskIcon />
            <span>{saving ? "Saving..." : saved ? "Saved" : "Save"}</span>
          </button>
          {isEdit && existing?.is_active ? (
            <button type="button" className="action-rail__btn" onClick={handleDeactivate} disabled={saving}>
              <span>Deactivate</span>
            </button>
          ) : null}
          <div className="action-rail__hint">
            {isEdit ? (
              <>
                <strong>Viewing:</strong> {existing?.name ?? "item"}
              </>
            ) : (
              <>
                <strong>Tip:</strong> item type drives costing — goods need a costing method, inventory
                and COGS accounts, and a sale price above zero.
              </>
            )}
          </div>
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path
        d="M4 4h13l3 3v13H4V4z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinejoin="round"
      />
      <path d="M8 4v5h7V4M8 20v-6h8v6" fill="none" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}
