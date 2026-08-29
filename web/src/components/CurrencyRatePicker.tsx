import { useEffect, useState } from "react";
import { api } from "../api";
import type { Currency } from "../types";

interface CurrencyRatePickerProps {
  /** Document currency code (controlled). */
  value: string;
  /** Exchange rate to base (controlled). */
  rate: number;
  onChange: (currency: string, rate: number) => void;
  /** Document date used for the latest-rate lookup. */
  docDate?: string;
  disabled?: boolean;
  id?: string;
}

/**
 * Currency + exchange-rate selector for commercial documents (SET-001).
 * IDR locks the rate at 1; foreign currencies auto-fill from the latest
 * manual rate (GET /exchange-rates/latest) and stay overridable.
 */
export function CurrencyRatePicker({ value, rate, onChange, docDate, disabled, id }: CurrencyRatePickerProps) {
  const [currencies, setCurrencies] = useState<Currency[]>([]);

  useEffect(() => {
    api.listCurrencies().then(setCurrencies).catch(() => undefined);
  }, []);

  useEffect(() => {
    if (value && value !== "IDR" && (!rate || rate <= 0)) {
      api
        .latestExchangeRate(value, "IDR")
        .then((latest) => {
          if (latest && latest.rate > 0) onChange(value, latest.rate);
        })
        .catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  return (
    <div className="currency-rate-picker" style={{ display: "flex", gap: 8, alignItems: "flex-end" }}>
      <label className="form-field" style={{ flex: "0 0 110px" }}>
        <span className="form-field__label">Currency</span>
        <select
          className="input input--compact"
          id={id}
          value={value}
          disabled={disabled}
          onChange={(e) => {
            const next = e.target.value;
            onChange(next, next === "IDR" ? 1 : 0);
          }}
        >
          {currencies.length === 0 && <option value="IDR">IDR</option>}
          {currencies.map((c) => (
            <option key={c.code} value={c.code}>
              {c.code}
            </option>
          ))}
        </select>
      </label>
      {value !== "IDR" && (
        <label className="form-field" style={{ flex: "0 0 140px" }}>
          <span className="form-field__label">Kurs (ke IDR)</span>
          <input
            className="input input--compact"
            type="number"
            min={0}
            step="any"
            value={rate || ""}
            placeholder="auto"
            disabled={disabled}
            onChange={(e) => onChange(value, Number(e.target.value))}
          />
        </label>
      )}
      {docDate && <span className="form-hint">Tanggal dok. {docDate}</span>}
    </div>
  );
}
