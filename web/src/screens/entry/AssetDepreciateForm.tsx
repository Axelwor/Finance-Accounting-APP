import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { FixedAsset } from "../../types";

interface Props {
  tabId: string;
  assetId: string | number;
  initialTitle?: string;
}

/**
 * AssetDepreciateForm: post one depreciation period for a fixed asset.
 * Backend: POST /fixed-assets/{id}/depreciate (Idempotency-Key required).
 * Idempotent per (asset, period_year, period_month) — replaying the same
 * Idempotency-Key or hitting an already-posted period returns the prior
 * schedule row instead of duplicating the journal.
 */
export function AssetDepreciateForm({ tabId, assetId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const id = Number(assetId);

  const [asset, setAsset] = useState<FixedAsset | null>(null);
  const [periodYear, setPeriodYear] = useState(new Date().getFullYear());
  const [periodMonth, setPeriodMonth] = useState(new Date().getMonth() + 1);
  const [entryDate, setEntryDate] = useState(new Date().toISOString().slice(0, 10));
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<string>("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!Number.isFinite(id)) return;
    api.getFixedAsset(id).then((a) => {
      setAsset(a);
      setPeriodYear(new Date().getFullYear());
      setPeriodMonth(new Date().getMonth() + 1);
    }).catch(() => {});
  }, [id]);

  // Estimated straight-line depreciation for display.
  const estimatedDep = (() => {
    if (!asset) return 0;
    if (asset.depreciation_method === "straight_line") {
      const depreciableBase = asset.acquisition_cost_cents - asset.salvage_value_cents;
      const remainingMonths = Math.max(asset.useful_life_months, 1);
      return Math.max(0, Math.floor(depreciableBase / remainingMonths));
    }
    return 0;
  })();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!asset) return;
    setSaving(true);
    setError("");
    setResult("");
    try {
      const res = await api.depreciateAsset(id, {
        period_year: periodYear,
        period_month: periodMonth,
        entry_date: entryDate,
        description,
      });
      if (res.already_posted) {
        setResult(`Depreciation for ${res.period_year}-${String(res.period_month).padStart(2, "0")} was already posted. Amount: ${formatIDR(res.depreciation_cents)}. Journal #${res.journal_entry_id}.`);
      } else {
        setResult(`Depreciation posted for ${res.period_year}-${String(res.period_month).padStart(2, "0")}: ${formatIDR(res.depreciation_cents)}. Journal #${res.journal_entry_id}. New book value: ${formatIDR(res.book_value_cents)}.`);
      }
      workbench.markUnsaved(tabId, false);
      // Refresh asset detail.
      api.getFixedAsset(id).then(setAsset).catch(() => {});
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post depreciation");
    } finally {
      setSaving(false);
    }
  }

  if (!asset) {
    return (
      <form className="entrytab">
        <div className="entrytab__header">
          <div className="entrytab__header-info">
            <div className="entrytab__header-title">{initialTitle || "Post Depreciation"}</div>
          </div>
        </div>
        <div className="entrytab__body">
          <div className="entrytab__detail"><p className="entrytab__hint">Loading asset...</p></div>
        </div>
      </form>
    );
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">Depreciation · {asset.code} — {asset.name}</div>
          <div className="entrytab__header-number">Book value: {formatIDR(asset.book_value_cents)} · Accum. dep: {formatIDR(asset.accum_dep_cents)}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Period Year *</span>
              <input className="input input--narrow" type="number" value={periodYear} min={2000} max={2100} onChange={(e) => setPeriodYear(parseInt(e.target.value) || 0)} />
            </label>
            <label className="field">
              <span className="field__label">Period Month *</span>
              <select className="input" value={periodMonth} onChange={(e) => setPeriodMonth(parseInt(e.target.value))}>
                {Array.from({ length: 12 }, (_, i) => (
                  <option key={i + 1} value={i + 1}>{String(i + 1).padStart(2, "0")} · {["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"][i]}</option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">Entry Date *</span>
              <input className="input" type="date" value={entryDate} onChange={(e) => setEntryDate(e.target.value)} />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Description</span>
            <input className="input" type="text" value={description} placeholder={`Depreciation ${asset.code} ${periodYear}-${String(periodMonth).padStart(2, "0")}`} onChange={(e) => setDescription(e.target.value)} />
          </label>

          <div className="entrytab__detail-title">Depreciation parameters</div>
          <div className="detail-grid detail-grid--quote">
            <div className="detail-grid__head">
              <div>Method</div>
              <div className="right">Useful Life (mo)</div>
              <div className="right">Cost</div>
              <div className="right">Salvage</div>
              <div className="right">Est. SL Amount</div>
            </div>
            <div className="detail-grid__row">
              <div>{asset.depreciation_method.replace("_", " ")}</div>
              <div className="right">{asset.useful_life_months}</div>
              <div className="right">{formatIDR(asset.acquisition_cost_cents)}</div>
              <div className="right">{formatIDR(asset.salvage_value_cents)}</div>
              <div className="right">{formatIDR(estimatedDep)}</div>
            </div>
          </div>

          <p className="entrytab__hint">
            Straight-line: (cost − salvage) ÷ useful life months. Declining balance: book value × rate.
            The actual amount is computed server-side and posted as Dr 5206 Depreciation Expense / Cr 1402 Accumulated Depreciation.
          </p>

          {result && (
            <div className="entrytab__total">
              <span className="entrytab__total-label">Result</span>
              <span className="entrytab__total-value">{result}</span>
            </div>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving || asset.status !== "ACTIVE"}>
            <span>{saving ? "Posting..." : "Post Depreciation"}</span>
          </button>
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
