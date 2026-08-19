import { useCallback, useEffect, useRef, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { FixedAsset, DepreciationMethod, ImpairAssetInput } from "../../types";
import { Button } from "../../components/m3";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const METHODS: { value: DepreciationMethod; label: string }[] = [
  { value: "straight_line", label: "Straight-line" },
  { value: "declining_balance", label: "Declining balance" },
  { value: "units_of_production", label: "Units of production" },
];

export function FixedAssetForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const toast = useToast();
  const impairPanelRef = useRef<HTMLDivElement>(null);

  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [acquisitionDate, setAcquisitionDate] = useState(new Date().toISOString().slice(0, 10));
  const [acquisitionCost, setAcquisitionCost] = useState("");
  const [salvageValue, setSalvageValue] = useState("0");
  const [usefulLifeMonths, setUsefulLifeMonths] = useState("");
  const [method, setMethod] = useState<DepreciationMethod>("straight_line");
  const [rate, setRate] = useState("");
  const [unitsTotal, setUnitsTotal] = useState("");
  const [paymentAccountCode, setPaymentAccountCode] = useState("1101");
  const [description, setDescription] = useState("");

  const [existing, setExisting] = useState<FixedAsset | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  /** Read-only once the asset exists (reopened tab or just registered). */
  const isExisting = !!entryId || existing !== null;

  // Revaluation panel state (PSAK 16 revaluation model).
  const [showRevalue, setShowRevalue] = useState(false);
  const [revalDate, setRevalDate] = useState(new Date().toISOString().slice(0, 10));
  const [revalAmount, setRevalAmount] = useState(0);
  const [revalDesc, setRevalDesc] = useState("");
  const [revalError, setRevalError] = useState("");
  const [postingReval, setPostingReval] = useState(false);

  // Impairment panel state (PSAK 16 / PSAK 48). Shown only for an existing
  // ACTIVE asset, mirroring the inline payment panel in InvoiceForm.
  const [impairDate, setImpairDate] = useState(new Date().toISOString().slice(0, 10));
  const [impairAmount, setImpairAmount] = useState(0);
  const [impairDesc, setImpairDesc] = useState("");
  const [impairError, setImpairError] = useState("");
  const [impairSuccess, setImpairSuccess] = useState("");
  const [postingImpair, setPostingImpair] = useState(false);

  const reloadAsset = useCallback(async (id: number) => {
    const asset = await api.getFixedAsset(id);
    setExisting(asset);
    setCode(asset.code);
    setName(asset.name);
    setAcquisitionDate(asset.acquisition_date);
    setAcquisitionCost(String(asset.acquisition_cost_cents));
    setSalvageValue(String(asset.salvage_value_cents));
    setUsefulLifeMonths(String(asset.useful_life_months));
    setMethod(asset.depreciation_method);
    setRate(asset.rate ?? "");
    setUnitsTotal(asset.units_total ? String(asset.units_total) : "");
    return asset;
  }, []);

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    void reloadAsset(id).catch(() => {});
  }, [entryId, reloadAsset]);

  function markDirty() {
    workbench.markUnsaved(tabId, true);
  }

  async function handlePostImpairment() {
    if (!existing) return;
    setImpairError("");
    setImpairSuccess("");
    const impairedCents = impairAmount;
    if (!Number.isFinite(impairedCents) || impairedCents < 0) {
      setImpairError("Impaired value must be a non-negative amount.");
      return;
    }
    if (!impairDate) {
      setImpairError("Entry date is required.");
      return;
    }
    if (impairedCents > existing.book_value_cents) {
      setImpairError("Impaired value cannot exceed the current book value.");
      return;
    }
    setPostingImpair(true);
    try {
      const result = await api.impairAsset(existing.id, {
        entry_date: impairDate,
        impaired_value_cents: impairedCents,
        description: impairDesc.trim() || undefined,
      } as ImpairAssetInput);
      setImpairSuccess(
        `Impairment posted. Loss: ${formatIDR(result.impairment_loss_cents)} · New book value: ${formatIDR(result.new_book_value_cents)}`
      );
      toast.success(`✓ Impairment posted — loss ${formatIDR(result.impairment_loss_cents)}`);
      setImpairAmount(0);
      setImpairDesc("");
      await reloadAsset(existing.id);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      const apiErr = err as { message?: string };
      setImpairError(apiErr?.message || (err instanceof Error ? err.message : "Could not post the impairment."));
    } finally {
      setPostingImpair(false);
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    const costCents = parseInt(acquisitionCost, 10);
    const salvageCents = parseInt(salvageValue, 10) || 0;
    const months = parseInt(usefulLifeMonths, 10);
    if (!code.trim()) { setError("Code is required."); return; }
    if (!name.trim()) { setError("Name is required."); return; }
    if (!acquisitionDate) { setError("Acquisition date is required."); return; }
    if (!Number.isFinite(costCents) || costCents <= 0) { setError("Acquisition cost must be > 0."); return; }
    if (salvageCents < 0) { setError("Salvage value must be >= 0."); return; }
    if (!Number.isFinite(months) || months <= 0) { setError("Useful life (months) must be > 0."); return; }
    if (method === "declining_balance" && !rate.trim()) { setError("Rate is required for declining balance."); return; }
    if (method === "units_of_production") {
      const ut = parseInt(unitsTotal, 10);
      if (!Number.isFinite(ut) || ut <= 0) { setError("Units total must be > 0 for units of production."); return; }
    }

    setSaving(true);
    try {
      const asset = await api.registerFixedAsset({
        code: code.trim(),
        name: name.trim(),
        acquisition_date: acquisitionDate,
        acquisition_cost_cents: costCents,
        salvage_value_cents: salvageCents,
        useful_life_months: months,
        depreciation_method: method,
        rate: method === "declining_balance" ? rate.trim() : undefined,
        units_total: method === "units_of_production" ? parseInt(unitsTotal, 10) : undefined,
        payment_account_code: paymentAccountCode || undefined,
        description: description.trim() || undefined,
      });
      workbench.replaceDraft(tabId, `${asset.code} · ${asset.name}`, asset.status);
      setExisting(asset);
      workbench.markUnsaved(tabId, false);
      toast.success(`✓ Registered asset ${asset.code}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register asset.");
    } finally {
      setSaving(false);
    }
  }

  async function handlePostRevaluation() {
    if (!existing) return;
    setRevalError("");
    if (!revalDate) {
      setRevalError("Entry date is required.");
      return;
    }
    if (!Number.isFinite(revalAmount) || revalAmount <= 0) {
      setRevalError("New fair value must be greater than zero.");
      return;
    }
    setPostingReval(true);
    try {
      const result = await api.revalueAsset(existing.id, {
        new_value_cents: revalAmount,
        entry_date: revalDate,
        description: revalDesc.trim() || undefined,
      });
      const direction = result.direction === "up" ? "increase" : "decrease";
      toast.success(`Revaluation posted — ${direction} of ${formatIDR(Math.abs(result.adjustment_cents))}`);
      setRevalAmount(0);
      setRevalDesc("");
      setShowRevalue(false);
      await reloadAsset(existing.id);
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setRevalError(err instanceof Error ? err.message : "Could not post the revaluation.");
    } finally {
      setPostingReval(false);
    }
  }

  /** Workflow-chain shortcuts for a saved asset. */
  const nextSteps = existing
    ? {
        depreciate: () =>
          workbench.openEntryExisting("asset-depreciate", existing.id, `Depreciate · ${existing.code}`, existing.status),
        dispose: () =>
          workbench.openEntryExisting("asset-dispose", existing.id, `Dispose · ${existing.code}`, existing.status),
        revalue: () => setShowRevalue((v) => !v),
        impair: () => {
          impairPanelRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
        },
        close: () => workbench.close(tabId),
      }
    : null;

  const monthlyDepreciation = computeMonthlyPreview(
    parseInt(acquisitionCost, 10) || 0,
    parseInt(salvageValue, 10) || 0,
    parseInt(usefulLifeMonths, 10) || 0,
  );

  const canAct = existing && existing.status === "ACTIVE";

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Fixed Asset"}</div>
          <div className="entrytab__header-number">
            {isExisting ? existing?.code : draftNumber("fixed-assets-entry")}
          </div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Asset Code *</span>
              <input
                className="input"
                type="text"
                value={code}
                onChange={(e) => { setCode(e.target.value); markDirty(); }}
                disabled={isExisting}
                placeholder="e.g. VEH-001"
              />
            </label>
            <label className="field">
              <span className="field__label">Asset Name *</span>
              <input
                className="input"
                type="text"
                value={name}
                onChange={(e) => { setName(e.target.value); markDirty(); }}
                disabled={isExisting}
                placeholder="e.g. Delivery Truck"
              />
            </label>
          </div>

          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Acquisition Date *</span>
              <input
                className="input"
                type="date"
                value={acquisitionDate}
                onChange={(e) => { setAcquisitionDate(e.target.value); markDirty(); }}
                disabled={isExisting}
              />
            </label>
            <label className="field">
              <span className="field__label">Payment Account (Cr)</span>
              <select
                className="input"
                value={paymentAccountCode}
                onChange={(e) => { setPaymentAccountCode(e.target.value); markDirty(); }}
                disabled={isExisting}
              >
                <option value="1101">1101 · Cash</option>
                <option value="1102">1102 · Bank</option>
                <option value="2101">2101 · Accounts Payable</option>
              </select>
            </label>
          </div>

          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Acquisition Cost (IDR) *</span>
              <input
                className="input input--narrow right"
                type="number"
                value={acquisitionCost}
                onChange={(e) => { setAcquisitionCost(e.target.value); markDirty(); }}
                disabled={isExisting}
              />
            </label>
            <label className="field">
              <span className="field__label">Salvage Value (IDR)</span>
              <input
                className="input input--narrow right"
                type="number"
                value={salvageValue}
                onChange={(e) => { setSalvageValue(e.target.value); markDirty(); }}
                disabled={isExisting}
              />
            </label>
          </div>

          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Useful Life (months) *</span>
              <input
                className="input input--narrow"
                type="number"
                value={usefulLifeMonths}
                onChange={(e) => { setUsefulLifeMonths(e.target.value); markDirty(); }}
                disabled={isExisting}
                placeholder="e.g. 60"
              />
            </label>
            <label className="field">
              <span className="field__label">Depreciation Method *</span>
              <select
                className="input"
                value={method}
                onChange={(e) => { setMethod(e.target.value as DepreciationMethod); markDirty(); }}
                disabled={isExisting}
              >
                {METHODS.map((m) => (
                  <option key={m.value} value={m.value}>{m.label}</option>
                ))}
              </select>
            </label>
          </div>

          {method === "declining_balance" && (
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Rate (e.g. 0.4 for 40%) *</span>
                <input
                  className="input input--narrow"
                  type="text"
                  value={rate}
                  onChange={(e) => { setRate(e.target.value); markDirty(); }}
                  disabled={isExisting}
                  placeholder="0.4"
                />
              </label>
            </div>
          )}

          {method === "units_of_production" && (
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Total Units *</span>
                <input
                  className="input input--narrow"
                  type="number"
                  value={unitsTotal}
                  onChange={(e) => { setUnitsTotal(e.target.value); markDirty(); }}
                  disabled={isExisting}
                  placeholder="e.g. 100000"
                />
              </label>
            </div>
          )}

          <label className="field">
            <span className="field__label">Description</span>
            <textarea
              className="input"
              rows={2}
              value={description}
              onChange={(e) => { setDescription(e.target.value); markDirty(); }}
              disabled={isExisting}
            />
          </label>

          {!isExisting && monthlyDepreciation > 0 && (
            <div className="entrytab__total">
              <span className="entrytab__total-label">Monthly depreciation (straight-line preview)</span>
              <span className="entrytab__total-value">{formatIDR(monthlyDepreciation)}</span>
            </div>
          )}

          {isExisting && existing && nextSteps && (
            <>
              <NextStepsBar number={existing.code} hint={existing.status}>
                {existing.status === "ACTIVE" && (
                  <>
                    <button type="button" className="next-steps__btn next-steps__btn--primary" onClick={nextSteps.depreciate}>
                      Depreciate
                    </button>
                    <button type="button" className="next-steps__btn next-steps__btn--danger" onClick={nextSteps.dispose}>
                      Dispose
                    </button>
                    <button type="button" className="next-steps__btn" onClick={nextSteps.revalue}>
                      Revalue
                    </button>
                    <button type="button" className="next-steps__btn" onClick={nextSteps.impair}>
                      Impair
                    </button>
                  </>
                )}
                <button type="button" className="next-steps__btn" onClick={nextSteps.close}>
                  Close
                </button>
              </NextStepsBar>

              <div className="entrytab__detail-title">Asset Summary</div>
              <div className="detail-grid detail-grid--quote">
                <div className="detail-grid__head">
                  <div>Cost</div>
                  <div className="right">Accum. Dep.</div>
                  <div className="right">Book Value</div>
                  <div className="right">Salvage</div>
                  <div>Status</div>
                </div>
                <div className="detail-grid__row">
                  <div>{formatIDR(existing.acquisition_cost_cents)}</div>
                  <div className="right">{formatIDR(existing.accum_dep_cents)}</div>
                  <div className="right">{formatIDR(existing.book_value_cents)}</div>
                  <div className="right">{formatIDR(existing.salvage_value_cents)}</div>
                  <div>{existing.status}</div>
                </div>
              </div>

              {existing.journal_entry_id ? (
                <div className="entrytab__total">
                  <span className="entrytab__total-label">Acquisition Journal Entry</span>
                  <span className="entrytab__total-value">#{existing.journal_entry_id}</span>
                </div>
              ) : null}

              {existing.schedule && existing.schedule.length > 0 && (
                <>
                  <div className="entrytab__detail-title">Depreciation Schedule</div>
                  <div className="detail-grid detail-grid--quote">
                    <div className="detail-grid__head">
                      <div>Period</div>
                      <div className="right">Depreciation</div>
                      <div>Posted</div>
                      <div>Journal</div>
                    </div>
                    {existing.schedule.map((s) => (
                      <div className="detail-grid__row" key={s.id}>
                        <div>{s.period_year}-{String(s.period_month).padStart(2, "0")}</div>
                        <div className="right">{formatIDR(s.depreciation_cents)}</div>
                        <div>{s.posted ? "Yes" : "No"}</div>
                        <div>{s.journal_entry_id ? `#${s.journal_entry_id}` : "—"}</div>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {existing.transactions && existing.transactions.length > 0 && (
                <>
                  <div className="entrytab__detail-title">Transactions</div>
                  <div className="detail-grid detail-grid--quote">
                    <div className="detail-grid__head">
                      <div>Date</div>
                      <div>Type</div>
                      <div className="right">Amount</div>
                      <div>Journal</div>
                    </div>
                    {existing.transactions.map((t) => (
                      <div className="detail-grid__row" key={t.id}>
                        <div>{t.tx_date}</div>
                        <div>{t.tx_type}</div>
                        <div className="right">{formatIDR(t.amount_cents)}</div>
                        <div>{t.journal_entry_id ? `#${t.journal_entry_id}` : "—"}</div>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {canAct && showRevalue && (
                <>
                  <div className="entrytab__detail-title">Record Revaluation (PSAK 16)</div>
                  <p className="field-note" style={{ marginBottom: 8 }}>
                    Adjust the asset to a new fair value. Posts the adjustment through the revaluation accounts. Current book value: {formatIDR(existing.book_value_cents)}.
                  </p>
                  <div className="detail-grid detail-grid--quote" style={{ gridTemplateColumns: "1fr 1fr 2fr" }}>
                    <div className="field">
                      <span className="field__label">Entry date</span>
                      <input className="input" type="date" value={revalDate} onChange={(e) => { setRevalDate(e.target.value); setRevalError(""); }} />
                    </div>
                    <div className="field">
                      <span className="field__label">New fair value (IDR)</span>
                      <input
                        className="amount input"
                        type="text"
                        inputMode="numeric"
                        value={centsInput(revalAmount)}
                        onChange={(e) => { setRevalAmount(parseCents(e.target.value)); setRevalError(""); }}
                        placeholder="0"
                      />
                    </div>
                    <div className="field">
                      <span className="field__label">Description</span>
                      <input className="input" type="text" value={revalDesc} onChange={(e) => { setRevalDesc(e.target.value); setRevalError(""); }} placeholder="Valuation reference" />
                    </div>
                  </div>
                  <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 8 }}>
                    <Button
                      variant="outlined"
                      size="sm"
                      disabled={postingReval}
                      onClick={() => void handlePostRevaluation()}
                    >
                      {postingReval ? "Posting..." : "Post Revaluation"}
                    </Button>
                  </div>
                  <FormError message={revalError} />
                </>
              )}

              {canAct && (
                <div ref={impairPanelRef}>
                  <div className="entrytab__detail-title">Record Impairment (PSAK 16)</div>
                  <p className="field-note" style={{ marginBottom: 8 }}>
                    Write the asset down to its impaired value. Posts Dr 5207 Impairment Loss / Cr 1401 Accumulated Impairment. Current book value: {formatIDR(existing.book_value_cents)}.
                  </p>
                  <div className="detail-grid detail-grid--quote" style={{ gridTemplateColumns: "1fr 1fr 2fr" }}>
                    <div className="field">
                      <span className="field__label">Entry date</span>
                      <input
                        className="input"
                        type="date"
                        value={impairDate}
                        onChange={(e) => { setImpairDate(e.target.value); setImpairSuccess(""); setImpairError(""); }}
                      />
                    </div>
                    <div className="field">
                      <span className="field__label">Impaired value (IDR)</span>
                      <input
                        className="amount input"
                        type="text"
                        inputMode="numeric"
                        value={centsInput(impairAmount)}
                        onChange={(e) => { setImpairAmount(parseCents(e.target.value)); setImpairSuccess(""); setImpairError(""); }}
                        placeholder="0"
                      />
                    </div>
                    <div className="field">
                      <span className="field__label">Description</span>
                      <input
                        className="input"
                        type="text"
                        value={impairDesc}
                        onChange={(e) => { setImpairDesc(e.target.value); setImpairSuccess(""); setImpairError(""); }}
                        placeholder="Reason for impairment"
                      />
                    </div>
                  </div>
                  <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 8 }}>
                    <Button
                      variant="outlined"
                      size="sm"
                      disabled={postingImpair}
                      onClick={() => void handlePostImpairment()}
                    >
                      {postingImpair ? "Posting..." : "Post Impairment"}
                    </Button>
                    {impairSuccess ? (
                      <span role="status" style={{ fontSize: 12, color: "var(--md-sys-color-on-surface-variant)" }}>{impairSuccess}</span>
                    ) : null}
                  </div>
                  <FormError message={impairError} />
                </div>
              )}
            </>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!isExisting && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Saving..." : "Register Asset"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function computeMonthlyPreview(cost: number, salvage: number, months: number): number {
  if (months <= 0 || cost <= 0) return 0;
  const base = cost - salvage;
  if (base <= 0) return 0;
  return Math.round(base / months);
}

/** Strip non-digits from a typed amount, returning cents as an integer. */
function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

/** Format a cents integer with thousands separators for display in the input. */
function centsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(cents);
}
