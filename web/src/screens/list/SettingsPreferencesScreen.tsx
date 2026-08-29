import { useEffect, useState } from "react";
import { api } from "../../api";
import type { TenantSettings } from "../../types";
import { LoadingState, FormError, EmptyState } from "../../components/ui";
import { Button } from "../../components/m3";
import { configureFormatters } from "../../lib/format";
import { showToast } from "../../lib/toast";

/** Format preferences editor (SET-001): date format + number formatting,
 *  applied globally (lists, forms, print) per tenant. */
export function SettingsPreferencesScreen() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    date_format: "DD/MM/YYYY" as TenantSettings["preferences"]["date_format"],
    thousand_separator: ".",
    decimal_separator: ",",
    amount_decimal_places: 2,
    qty_decimal_places: 2,
  });

  useEffect(() => {
    api
      .getSettings()
      .then((s) => {
        setSettings(s);
        setForm({ ...s.preferences });
      })
      .catch(() => setError("Failed to load settings"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading preferences..." />;
  if (error) return <FormError message={error} />;
  if (!settings) return <EmptyState title="No settings" message="Settings could not be loaded." />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Preferences</span>
          <small>Date and number formatting applied across the whole workbench.</small>
        </div>
      </div>
      <div className="listtab__body">
        <div className="form-card">
          <div className="form-card__title">Date Format</div>
          <div className="form-grid form-grid-2col">
            <label className="form-field">
              <span className="form-field__label">Date Format</span>
              <select
                className="input input--compact"
                value={form.date_format}
                onChange={(e) => setForm((f) => ({ ...f, date_format: e.target.value as typeof f.date_format }))}
              >
                <option value="DD/MM/YYYY">DD/MM/YYYY (28/08/2026)</option>
                <option value="MM/DD/YYYY">MM/DD/YYYY (08/28/2026)</option>
                <option value="YYYY-MM-DD">YYYY-MM-DD (2026-08-28)</option>
              </select>
            </label>
          </div>
        </div>
        <div className="form-card">
          <div className="form-card__title">Number Format</div>
          <div className="form-grid form-grid-2col">
            <label className="form-field">
              <span className="form-field__label">Thousand Separator</span>
              <input
                className="input input--compact"
                type="text"
                maxLength={1}
                value={form.thousand_separator}
                onChange={(e) => setForm((f) => ({ ...f, thousand_separator: e.target.value }))}
              />
            </label>
            <label className="form-field">
              <span className="form-field__label">Decimal Separator</span>
              <input
                className="input input--compact"
                type="text"
                maxLength={1}
                value={form.decimal_separator}
                onChange={(e) => setForm((f) => ({ ...f, decimal_separator: e.target.value }))}
              />
            </label>
            <label className="form-field">
              <span className="form-field__label">Amount Decimal Places</span>
              <input
                className="input input--compact"
                type="number"
                min={0}
                max={4}
                value={form.amount_decimal_places}
                onChange={(e) => setForm((f) => ({ ...f, amount_decimal_places: Number(e.target.value) }))}
              />
            </label>
            <label className="form-field">
              <span className="form-field__label">Quantity Decimal Places</span>
              <input
                className="input input--compact"
                type="number"
                min={0}
                max={4}
                value={form.qty_decimal_places}
                onChange={(e) => setForm((f) => ({ ...f, qty_decimal_places: Number(e.target.value) }))}
              />
            </label>
          </div>
        </div>
      </div>
      <div className="listtab__footer">
        <Button
          variant="filled"
          disabled={saving}
          onClick={async () => {
            setSaving(true);
            try {
              const preferences = await api.updatePreferences(form);
              setSettings((s) => (s ? { ...s, preferences } : s));
              configureFormatters({
                amountDecimalPlaces: preferences.amount_decimal_places,
                thousandSeparator: preferences.thousand_separator,
                decimalSeparator: preferences.decimal_separator,
                dateFormat: preferences.date_format,
              });
              showToast("Preferences saved");
            } catch (err) {
              showToast(err instanceof Error ? err.message : "Failed to save preferences", "error");
            } finally {
              setSaving(false);
            }
          }}
        >
          {saving ? "Saving..." : "Save Preferences"}
        </Button>
      </div>
    </div>
  );
}
