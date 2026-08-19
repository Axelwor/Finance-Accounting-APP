import { useState } from "react";
import { ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { CalculateECLResult } from "../../types";
import { Button } from "../../components/m3";

/**
 * ECL Calculator (US-082, PSAK 48).
 *
 * Expected Credit Loss ages outstanding receivables into buckets and applies
 * a loss rate per bucket. The calculator takes an as-of date and entry date,
 * posts the provision (Dr 5205 / Cr 1202 to reach the target allowance), and
 * shows the aging breakdown. A write-off action removes a specific receivable
 * (Dr 1202 / Cr 1201).
 */
const DEFAULT_RATES: Record<string, number> = {
  "0-30": 1.0,
  "31-60": 2.5,
  "61-90": 5.0,
  ">90": 10.0,
};

export function ECLCalculator() {
  const today = new Date().toISOString().slice(0, 10);
  const [asOfDate, setAsOfDate] = useState(today);
  const [entryDate, setEntryDate] = useState(today);
  const [result, setResult] = useState<CalculateECLResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Write-off form state.
  const [woAmount, setWoAmount] = useState<string>("");
  const [woInvoiceId, setWoInvoiceId] = useState<string>("");
  const [woNotes, setWoNotes] = useState<string>("");
  const [woError, setWoError] = useState<string | null>(null);
  const [woMsg, setWoMsg] = useState<string | null>(null);
  const [writingOff, setWritingOff] = useState(false);

  const calculate = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const data = await api.calculateECL({ as_of_date: asOfDate, entry_date: entryDate });
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to calculate ECL.");
    } finally {
      setLoading(false);
    }
  };

  const writeOff = async () => {
    const amount = Number(woAmount);
    if (!amount || amount <= 0) {
      setWoError("Enter a write-off amount greater than zero.");
      return;
    }
    setWritingOff(true);
    setWoError(null);
    setWoMsg(null);
    try {
      const res = await api.writeOffReceivable({
        entry_date: entryDate,
        amount_cents: Math.round(amount),
        invoice_id: woInvoiceId ? Number(woInvoiceId) : undefined,
        notes: woNotes || undefined,
      });
      setWoMsg(`Wrote off ${formatIDR(res.amount_cents)} — posted ${res.number}.`);
      setWoAmount("");
      setWoInvoiceId("");
      setWoNotes("");
    } catch (err) {
      setWoError(err instanceof Error ? err.message : "Failed to write off receivable.");
    } finally {
      setWritingOff(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>ECL — Penyisihan Piutang</span>
          <small>Expected Credit Loss (PSAK 48): aging-based allowance for doubtful accounts.</small>
        </div>
      </div>
      <div className="listtab__body">
        <section className="form-section">
          <div className="form-row">
            <label className="form-field">
              <span className="form-field__label">As-of date</span>
              <input className="input" type="date" value={asOfDate} onChange={(e) => setAsOfDate(e.target.value)} />
            </label>
            <label className="form-field">
              <span className="form-field__label">Entry date</span>
              <input className="input" type="date" value={entryDate} onChange={(e) => setEntryDate(e.target.value)} />
            </label>
            <Button
              variant="filled"
              onClick={() => void calculate()}
              disabled={loading}
            >
              {loading ? "Calculating..." : "Calculate & Post"}
            </Button>
          </div>
        </section>

        {loading ? (
          <LoadingState label="Aging receivables..." />
        ) : error ? (
          <ErrorState message={error} />
        ) : result ? (
          <div className="result-block">
            <div className="kpi-list">
              <div className="kpi-list__row">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Target Allowance</span>
                  <span className="kpi-list__label-note">As of {result.as_of_date}</span>
                </div>
                <span className="kpi-list__value">{formatIDR(result.target_allowance_cents)}</span>
              </div>
              <div className="kpi-list__row">
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Current Allowance</span>
                  <span className="kpi-list__label-note">1202 balance</span>
                </div>
                <span className="kpi-list__value">{formatIDR(result.current_allowance_cents)}</span>
              </div>
              <div className={`kpi-list__row ${result.adjustment_cents >= 0 ? "is-pos" : "is-neg"}`}>
                <div className="kpi-list__label">
                  <span className="kpi-list__label-title">Adjustment</span>
                  <span className="kpi-list__label-note">
                    {result.adjustment_cents >= 0 ? "Dr 5205 / Cr 1202" : "Dr 1202 / Cr 5205 (release)"}
                  </span>
                </div>
                <span className="kpi-list__value">{formatIDR(Math.abs(result.adjustment_cents))}</span>
              </div>
            </div>

            {result.journal_entry_id > 0 ? (
              <div className="result-block__meta">
                <span className="meta-pill">
                  Posted {result.number} · {result.intent_type}
                </span>
                {result.description && <span className="meta-pill meta-pill--muted">{result.description}</span>}
              </div>
            ) : null}

            <section style={{ marginTop: 16 }}>
              <h3 style={{ margin: "0 0 8px", fontSize: 14, fontWeight: 600 }}>Aging Buckets</h3>
              <div style={{ overflowX: "auto" }}>
                <table className="table" style={{ width: "100%", fontSize: 13 }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: "left" }}>Bucket</th>
                      <th style={{ textAlign: "right" }}>Rate %</th>
                      <th style={{ textAlign: "right" }}>Balance</th>
                      <th style={{ textAlign: "right" }}>Provision</th>
                    </tr>
                  </thead>
                  <tbody>
                    {result.buckets.map((b) => (
                      <tr key={b.label}>
                        <td>{b.label} days</td>
                        <td style={{ textAlign: "right" }}>{b.rate_pct}%</td>
                        <td style={{ textAlign: "right" }}>{formatIDR(b.balance_cents)}</td>
                        <td style={{ textAlign: "right" }}>{formatIDR(b.provision_cents)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <section style={{ marginTop: 24, borderTop: "1px solid var(--md-sys-color-outline-variant)", paddingTop: 16 }}>
              <h3 style={{ margin: "0 0 8px", fontSize: 14, fontWeight: 600 }}>Write Off Receivable</h3>
              <p style={{ fontSize: 13, color: "var(--md-sys-color-on-surface-variant)", margin: "0 0 12px" }}>
                Remove a specific uncollectible receivable: Dr 1202 (Allowance) / Cr 1201 (AR).
              </p>
              <div className="form-row">
                <label className="form-field">
                  <span className="form-field__label">Amount (rupiah)</span>
                  <input
                    className="input"
                    type="number"
                    min="0"
                    step="1"
                    value={woAmount}
                    onChange={(e) => setWoAmount(e.target.value)}
                    placeholder="e.g. 500000"
                  />
                </label>
                <label className="form-field">
                  <span className="form-field__label">Invoice ID (optional)</span>
                  <input
                    className="input"
                    type="number"
                    min="0"
                    value={woInvoiceId}
                    onChange={(e) => setWoInvoiceId(e.target.value)}
                    placeholder="invoice id"
                  />
                </label>
                <label className="form-field">
                  <span className="form-field__label">Notes (optional)</span>
                  <input
                    className="input"
                    type="text"
                    value={woNotes}
                    onChange={(e) => setWoNotes(e.target.value)}
                    placeholder="reason"
                  />
                </label>
                <Button
                  variant="outlined"
                  onClick={() => void writeOff()}
                  disabled={writingOff}
                >
                  {writingOff ? "Posting..." : "Write Off"}
                </Button>
              </div>
              {woError ? <p style={{ marginTop: 8, fontSize: 13, color: "var(--md-sys-color-error)" }}>{woError}</p> : null}
              {woMsg ? <p style={{ marginTop: 8, fontSize: 13, color: "var(--md-sys-color-success)" }}>{woMsg}</p> : null}
            </section>
          </div>
        ) : (
          <div className="workarea__placeholder">
            <p>Pick an as-of date and press Calculate to age receivables and post the ECL provision.</p>
            <p style={{ fontSize: 13, color: "var(--md-sys-color-on-surface-variant)", marginTop: 8 }}>
              Default rates: {Object.entries(DEFAULT_RATES).map(([k, v]) => `${k}=${v}%`).join(", ")}.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
