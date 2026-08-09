import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { FixedAsset } from "../../types";

interface Props {
  tabId: string;
  assetId: number;
  initialTitle?: string;
}

export function AssetDisposeForm({ tabId, assetId, initialTitle }: Props) {
  const workbench = useWorkbench();

  const [asset, setAsset] = useState<FixedAsset | null>(null);
  const [disposalDate, setDisposalDate] = useState(new Date().toISOString().slice(0, 10));
  const [proceedsCents, setProceedsCents] = useState("0");
  const [cashAccountCode, setCashAccountCode] = useState("1101");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<{ journal_entry_id?: number; gain_loss_cents: number } | null>(null);

  useEffect(() => {
    api.getFixedAsset(assetId).then((fa) => {
      setAsset(fa);
      setProceedsCents(String(fa.book_value_cents));
    }).catch(() => {});
  }, [assetId]);

  const proceeds = parseInt(proceedsCents) || 0;
  const bookValue = asset?.book_value_cents ?? 0;
  const gainLoss = proceeds - bookValue;
  const gainLossLabel = gainLoss >= 0 ? "Gain" : "Loss";

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!asset) return;
    if (proceeds <= 0) {
      setError("Proceeds must be greater than 0.");
      return;
    }
    if (!disposalDate) {
      setError("Disposal date is required.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const res = await api.disposeAsset(asset.id, {
        disposal_date: disposalDate,
        proceeds_cents: proceeds,
        cash_account_code: cashAccountCode,
        description: notes.trim() || undefined,
      });
      setResult({ journal_entry_id: res.journal_entry_id, gain_loss_cents: res.gain_loss_cents });
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to dispose asset.");
    } finally {
      setSaving(false);
    }
  }

  if (!asset) {
    return (
      <form className="entrytab">
        <div className="entrytab__header">
          <div className="entrytab__header-info">
            <div className="entrytab__header-title">{initialTitle || "Dispose Asset"}</div>
          </div>
        </div>
        <div className="entrytab__body">
          <div className="entrytab__detail">Loading asset...</div>
        </div>
      </form>
    );
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Dispose Asset"}</div>
          <div className="entrytab__header-number">{asset.code} · {asset.name}</div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <div className="field">
              <span className="field__label">Acquisition Cost</span>
              <div className="field__static">{formatIDR(asset.acquisition_cost_cents)}</div>
            </div>
            <div className="field">
              <span className="field__label">Accumulated Depreciation</span>
              <div className="field__static">{formatIDR(asset.accum_dep_cents)}</div>
            </div>
            <div className="field">
              <span className="field__label">Book Value (NBV)</span>
              <div className="field__static">{formatIDR(asset.book_value_cents)}</div>
            </div>
            <div className="field">
              <span className="field__label">Status</span>
              <div className="field__static">{asset.status}</div>
            </div>
          </div>

          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Disposal Date *</span>
              <input
                className="input"
                type="date"
                value={disposalDate}
                onChange={(e) => setDisposalDate(e.target.value)}
                disabled={!!result}
              />
            </label>
            <label className="field">
              <span className="field__label">Proceeds (Cash Received) *</span>
              <input
                className="input input--narrow"
                type="number"
                min="0"
                step="1"
                value={proceedsCents}
                onChange={(e) => setProceedsCents(e.target.value)}
                disabled={!!result}
              />
            </label>
            <label className="field">
              <span className="field__label">Cash Account Code</span>
              <input
                className="input input--narrow"
                type="text"
                value={cashAccountCode}
                onChange={(e) => setCashAccountCode(e.target.value)}
                disabled={!!result}
                placeholder="1101"
              />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Notes</span>
            <textarea
              className="input"
              rows={2}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              disabled={!!result}
              placeholder="Buyer / reason for disposal..."
            />
          </label>

          <div className="entrytab__total">
            <span className="entrytab__total-label">Expected {gainLossLabel}</span>
            <span className="entrytab__total-value">{formatIDR(Math.abs(gainLoss))}</span>
          </div>

          {result && (
            <>
              <div className="entrytab__total">
                <span className="entrytab__total-label">Journal Entry</span>
                <span className="entrytab__total-value">#{result.journal_entry_id}</span>
              </div>
              <div className="entrytab__total">
                <span className="entrytab__total-label">Realised {result.gain_loss_cents >= 0 ? "Gain" : "Loss"}</span>
                <span className="entrytab__total-value">{formatIDR(Math.abs(result.gain_loss_cents))}</span>
              </div>
            </>
          )}
        </div>

        <aside className="action-rail" aria-label="Form actions">
          {!result && (
            <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
              <span>{saving ? "Disposing..." : "Dispose & Post"}</span>
            </button>
          )}
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}
