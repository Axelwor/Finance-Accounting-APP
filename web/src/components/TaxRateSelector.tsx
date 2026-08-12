import { useId } from "react";

/**
 * PPN / tax rate picker for sales & purchase document forms.
 *
 * Combobox-style control with the two Indonesian presets plus a free-form
 * rate:
 *   - Non-PKP  → PPN 0%  (business not registered as PKP)
 *   - PKP      → PPN 11% (standard rate)
 *   - Custom   → any rate typed by the user
 *
 * The parent form owns the numeric rate; this component only renders the
 * preset dropdown plus the numeric input when a custom rate is active.
 */
export interface TaxRateSelectorProps {
  /** Tax rate in percent, e.g. 0, 11 or any custom value. */
  value: number;
  onChange: (rate: number) => void;
  disabled?: boolean;
  /** Optional label override; defaults to "PPN / Tax". */
  label?: string;
}

export function TaxRateSelector({ value, onChange, disabled, label = "PPN / Tax" }: TaxRateSelectorProps) {
  const id = useId();
  const selectValue = value === 0 ? "non-pkp" : value === 11 ? "pkp" : "custom";

  const handleSelect = (next: string) => {
    if (next === "non-pkp") onChange(0);
    else if (next === "pkp") onChange(11);
    // "custom": keep the current value so the numeric input shows it for editing.
  };

  return (
    <div className="field">
      <span className="field__label" id={id}>
        {label}
      </span>
      <div style={{ display: "flex", gap: 6, alignItems: "center" }} aria-labelledby={id}>
        <select
          className="input"
          value={selectValue}
          onChange={(e) => handleSelect(e.target.value)}
          disabled={disabled}
          aria-label={`${label} preset`}
        >
          <option value="non-pkp">Non-PKP · PPN 0%</option>
          <option value="pkp">PKP · PPN 11%</option>
          <option value="custom">Custom…</option>
        </select>
        {selectValue === "custom" && (
          <input
            className="input"
            type="number"
            min={0}
            max={100}
            step="any"
            style={{ width: 84, flex: "0 0 auto" }}
            value={Number.isFinite(value) ? value : 0}
            onChange={(e) => onChange(clampRate(e.target.value))}
            disabled={disabled}
            aria-label="Custom tax rate percent"
          />
        )}
        <span className="field__label" style={{ whiteSpace: "nowrap" }}>%</span>
      </div>
    </div>
  );
}

function clampRate(raw: string): number {
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, Math.min(100, parsed));
}

/** PPN amount for one line: round(lineTotal * rate / 100). */
export function taxForLine(lineTotalCents: number, taxRate: number): number {
  if (!taxRate || taxRate <= 0 || !lineTotalCents) return 0;
  return Math.round((lineTotalCents * taxRate) / 100);
}
