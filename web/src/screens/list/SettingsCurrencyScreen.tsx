import { useEffect, useState } from "react";
import { api } from "../../api";
import type { Currency, ExchangeRate, TenantSettings } from "../../types";
import { LoadingState, FormError, EmptyState } from "../../components/ui";
import { Button } from "../../components/m3";
import { todayISO } from "../../lib/format";
import { showToast } from "../../lib/toast";

/** Currency & exchange-rate maintenance (SET-001). Base currency is locked
 *  once journals exist; rates are manual per effective date. */
export function SettingsCurrencyScreen() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [rates, setRates] = useState<ExchangeRate[]>([]);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [baseCurrency, setBaseCurrency] = useState("");
  const [showRateForm, setShowRateForm] = useState(false);
  const [rateForm, setRateForm] = useState({ from_currency: "USD", to_currency: "IDR", rate: "", effective_date: todayISO() });
  const [saving, setSaving] = useState(false);

  const reload = () => {
    Promise.all([api.getSettings(), api.listExchangeRates(), api.listCurrencies()])
      .then(([s, r, c]) => {
        setSettings(s);
        setRates(r);
        setCurrencies(c);
        setBaseCurrency(s.company.base_currency);
      })
      .catch(() => setError("Failed to load currency settings"))
      .finally(() => setLoading(false));
  };

  useEffect(reload, []);

  if (loading) return <LoadingState label="Loading currency settings..." />;
  if (error) return <FormError message={error} />;
  if (!settings) return <EmptyState title="No settings" message="Settings could not be loaded." />;

  const locked = settings.company.has_journals;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Currency & Exchange Rates</span>
          <small>Base currency and manual rates for foreign-currency documents.</small>
        </div>
      </div>
      <div className="listtab__body">
        <div className="form-card">
          <div className="form-card__title">Base Currency</div>
          <div className="form-grid form-grid-2col">
            <label className="form-field">
              <span className="form-field__label">Base Currency {locked && <span className="kind-mark is-negative">Locked</span>}</span>
              <select
                className="input input--compact"
                value={baseCurrency}
                disabled={locked}
                onChange={(e) => setBaseCurrency(e.target.value)}
              >
                {currencies.map((c) => (
                  <option key={c.code} value={c.code}>
                    {c.code} — {c.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="form-field">
              <span className="form-field__label">&nbsp;</span>
              <Button
                variant="filled"
                disabled={locked || baseCurrency === settings.company.base_currency}
                onClick={async () => {
                  try {
                    const company = await api.updateBaseCurrency(baseCurrency);
                    setSettings((s) => (s ? { ...s, company } : s));
                    showToast(`Base currency set to ${baseCurrency}`);
                  } catch (err) {
                    showToast(err instanceof Error ? err.message : "Failed to change base currency", "error");
                  }
                }}
              >
                Change Base Currency
              </Button>
            </div>
          </div>
          {locked && (
            <p className="form-hint">
              The base currency is locked because journals already exist. Changing it would restate the whole ledger.
            </p>
          )}
        </div>

        <div className="form-card">
          <div className="form-card__title">Exchange Rates (manual)</div>
          <div className="listtab__toolbar">
            <span className="listtab__count">{rates.length} rate(s)</span>
            <Button variant="filled" size="sm" onClick={() => setShowRateForm((v) => !v)}>
              {showRateForm ? "Close" : "+ Add Rate"}
            </Button>
          </div>
          {showRateForm && (
            <div className="form-grid form-grid-2col" style={{ marginBottom: 12 }}>
              <label className="form-field">
                <span className="form-field__label">From</span>
                <select
                  className="input input--compact"
                  value={rateForm.from_currency}
                  onChange={(e) => setRateForm((f) => ({ ...f, from_currency: e.target.value }))}
                >
                  {currencies.filter((c) => c.code !== baseCurrency).map((c) => (
                    <option key={c.code} value={c.code}>{c.code}</option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span className="form-field__label">To (base)</span>
                <input className="input input--compact" type="text" value={baseCurrency} disabled />
              </label>
              <label className="form-field">
                <span className="form-field__label">Rate (1 foreign = ? base)</span>
                <input
                  className="input input--compact"
                  type="number"
                  min="0"
                  step="any"
                  value={rateForm.rate}
                  placeholder="15750"
                  onChange={(e) => setRateForm((f) => ({ ...f, rate: e.target.value }))}
                />
              </label>
              <label className="form-field">
                <span className="form-field__label">Effective Date</span>
                <input
                  className="input input--compact"
                  type="date"
                  value={rateForm.effective_date}
                  onChange={(e) => setRateForm((f) => ({ ...f, effective_date: e.target.value }))}
                />
              </label>
              <div className="form-field">
                <span className="form-field__label">&nbsp;</span>
                <Button
                  variant="filled"
                  disabled={saving || !rateForm.rate}
                  onClick={async () => {
                    setSaving(true);
                    try {
                      await api.createExchangeRate({
                        from_currency: rateForm.from_currency,
                        to_currency: baseCurrency,
                        rate: Number(rateForm.rate),
                        effective_date: rateForm.effective_date,
                      });
                      showToast("Exchange rate saved");
                      setShowRateForm(false);
                      reload();
                    } catch (err) {
                      showToast(err instanceof Error ? err.message : "Failed to save rate", "error");
                    } finally {
                      setSaving(false);
                    }
                  }}
                >
                  Save Rate
                </Button>
              </div>
            </div>
          )}
          {rates.length === 0 ? (
            <EmptyState title="No rates yet" message="Add a rate so foreign-currency documents can convert to the base currency." />
          ) : (
            <div className="ledger-table">
              <div className="ledger-table__head">
                <span>From</span>
                <span>To</span>
                <span>Rate</span>
                <span>Effective</span>
                <span>Source</span>
                <span>Actions</span>
              </div>
              {rates.map((r) => (
                <div key={r.id} className="ledger-table__row">
                  <span className="ledger-table__no">{r.from_currency}</span>
                  <span className="ledger-table__cat">{r.to_currency}</span>
                  <span className="ledger-table__amt">{r.rate}</span>
                  <span className="ledger-table__memo">{r.effective_date}</span>
                  <span className="ledger-table__memo">{r.source}</span>
                  <span className="ledger-table__action">
                    <Button
                      variant="outlined"
                      size="xs"
                      danger
                      onClick={() => {
                        if (!window.confirm(`Delete rate ${r.from_currency}/${r.to_currency} @ ${r.rate}?`)) return;
                        api
                          .deleteExchangeRate(r.id)
                          .then(() => {
                            setRates((prev) => prev.filter((x) => x.id !== r.id));
                            showToast("Rate deleted");
                          })
                          .catch((err) => showToast(err instanceof Error ? err.message : "Failed to delete rate"));
                      }}
                    >
                      Delete
                    </Button>
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
