import { useEffect, useState } from "react";
import { ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { PPhFinalResult } from "../../types";

interface CashAccount {
  id: number;
  code: string;
  name: string;
  account_type: string;
}

/**
 * PPh Final UMKM calculator (US-081).
 *
 * PPh Final 0,5% is computed on monthly sales turnover (account 4101 net
 * of returns). The calculator takes a period (year + month), fetches the
 * revenue, applies the configured UMKM rate from the tax_rates table, and
 * posts Dr 5208 Income Tax Expense / Cr 2203 Income Tax Payable. A second
 * action settles the payable: Dr 2203 / Cr Cash.
 */
export function PPhFinalCalculator() {
  const now = new Date();
  const [year, setYear] = useState<string>(String(now.getFullYear()));
  const [month, setMonth] = useState<string>(String(now.getMonth() + 1));
  const [result, setResult] = useState<PPhFinalResult | null>(null);
  const [cashAccounts, setCashAccounts] = useState<CashAccount[]>([]);
  const [cashAccountId, setCashAccountId] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [paying, setPaying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [payError, setPayError] = useState<string | null>(null);

  useEffect(() => {
    void api.listCashAccounts().then((accts) => {
      setCashAccounts(accts);
      if (accts.length > 0) setCashAccountId(String(accts[0].id));
    });
  }, []);

  const monthOptions = Array.from({ length: 12 }, (_, i) => ({
    value: String(i + 1),
    label: new Date(2000, i, 1).toLocaleString("en-US", { month: "long" }),
  }));

  const periodEntryDate = () => {
    const lastDay = new Date(Number(year), Number(month), 0).getDate();
    return `${year}-${String(month).padStart(2, "0")}-${String(lastDay).padStart(2, "0")}`;
  };

  const calculate = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const data = await api.calculatePPhFinal({
        period_year: Number(year),
        period_month: Number(month),
        entry_date: periodEntryDate(),
      });
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to calculate PPh Final.");
    } finally {
      setLoading(false);
    }
  };

  const pay = async () => {
    if (!result) return;
    const acctId = Number(cashAccountId);
    if (!acctId) {
      setPayError("Select a cash/bank account to settle the payable.");
      return;
    }
    setPaying(true);
    setPayError(null);
    try {
      const data = await api.payPPhFinal({
        entry_date: periodEntryDate(),
        cash_account_id: acctId,
        amount_cents: result.payable_balance_cents,
      });
      setResult(data);
    } catch (err) {
      setPayError(err instanceof Error ? err.message : "Failed to record tax payment.");
    } finally {
      setPaying(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>PPh Final UMKM</span>
          <small>0,5% tax on monthly sales turnover — calculate, accrue, and settle the payable.</small>
        </div>
      </div>
      <div className="listtab__body">
        <section className="form-section">
          <div className="form-row">
            <label className="form-field">
              <span className="form-field__label">Year</span>
              <input
                className="input"
                type="number"
                min="2000"
                max="2100"
                value={year}
                onChange={(e) => setYear(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span className="form-field__label">Month</span>
              <select className="input" value={month} onChange={(e) => setMonth(e.target.value)}>
                {monthOptions.map((m) => (
                  <option key={m.value} value={m.value}>
                    {m.label}
                  </option>
                ))}
              </select>
            </label>
            <button type="button" className="btn btn--primary" onClick={() => void calculate()} disabled={loading}>
              {loading ? "Calculating..." : "Calculate & Accrue"}
            </button>
          </div>
        </section>

        {loading ? (
          <LoadingState label="Computing PPh Final..." />
        ) : error ? (
          <ErrorState message={error} />
        ) : result ? (
          <div className="result-block">
            <div className="kpi-list">
              <div className="kpi-list__row">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Monthly Revenue</span>
                  <span className="kpi-list__label-note">Sales net of returns</span>
                </div>
                <span className="kpi-list__value">{formatIDR(result.revenue_cents)}</span>
              </div>
              <div className="kpi-list__row">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Tax Rate</span>
                  <span className="kpi-list__label-note">PPh Final UMKM</span>
                </div>
                <span className="kpi-list__value">{result.tax_rate}</span>
              </div>
              <div className="kpi-list__row is-pos">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Tax Payable</span>
                  <span className="kpi-list__label-note">Dr 5208 / Cr 2203</span>
                </div>
                <span className="kpi-list__value">{formatIDR(result.tax_cents)}</span>
              </div>
              <div className="kpi-list__row">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Payable Balance</span>
                  <span className="kpi-list__label-note">2203 outstanding</span>
                </div>
                <span className="kpi-list__value">{formatIDR(result.payable_balance_cents)}</span>
              </div>
            </div>

            <div className="result-block__meta">
              {result.journal_entry_id > 0 && (
                <span className="meta-pill">
                  Posted {result.number} · {result.intent_type}
                </span>
              )}
              {result.description && <span className="meta-pill meta-pill--muted">{result.description}</span>}
            </div>

            <div className="result-block__actions">
              <label className="form-field">
                <span className="form-field__label">Settle from account</span>
                <select
                  className="input"
                  value={cashAccountId}
                  onChange={(e) => setCashAccountId(e.target.value)}
                  disabled={cashAccounts.length === 0}
                >
                  {cashAccounts.length === 0 ? (
                    <option value="">No cash/bank accounts</option>
                  ) : (
                    cashAccounts.map((a) => (
                      <option key={a.id} value={String(a.id)}>
                        {a.code} · {a.name}
                      </option>
                    ))
                  )}
                </select>
              </label>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => void pay()}
                disabled={paying || result.payable_balance_cents <= 0 || cashAccounts.length === 0}
              >
                {paying ? "Settling..." : "Settle Payable (Dr 2203 / Cr Cash)"}
              </button>
            </div>
            {payError ? <p style={{ marginTop: 8, fontSize: 13, color: "var(--neg)" }}>{payError}</p> : null}
          </div>
        ) : (
          <div className="workarea__placeholder">
            <p>Pick a period and press Calculate to accrue the 0,5% PPh Final UMKM.</p>
          </div>
        )}
      </div>
    </div>
  );
}
