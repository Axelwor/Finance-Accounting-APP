import { useEffect, useMemo, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { Button, EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { PPNReconciliationResult, PPNReconciliationRecord, PPNSummary } from "../../types";
import { Button as M3Button } from "../../components/m3";

/**
 * PPN Reconciliation (US-080).
 *
 * Shows the monthly PPN (SPT Masa PPN) reconciliation: output VAT
 * (keluaran, 2202 credits) vs input VAT (masukan, 1203 debits), the net
 * payable, and every posted VAT movement in the period. The "File
 * reconciliation" button upserts a ppn_reconciliations row marked FILED.
 */
export function PPNReconciliation() {
  const now = new Date();
  const [periodYear, setPeriodYear] = useState(String(now.getFullYear()));
  const [periodMonth, setPeriodMonth] = useState(String(now.getMonth() + 1));
  const [summary, setSummary] = useState<PPNSummary | null>(null);
  const [recon, setRecon] = useState<PPNReconciliationResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filing, setFiling] = useState(false);
  const [filedRecord, setFiledRecord] = useState<PPNReconciliationRecord | null>(null);
  const [fileMsg, setFileMsg] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    setFileMsg(null);
    const year = Number(periodYear);
    const month = Number(periodMonth);
    const [sum, detail] = await Promise.all([
      api.getPPNSummary(`${year}-${String(month).padStart(2, "0")}-01`, `${year}-${String(month).padStart(2, "0")}-28`),
      api.getPPNReconciliation(year, month),
    ]);
    setSummary(sum);
    setRecon(detail);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [periodYear, periodMonth]);
  useTabRefresh(load);

  const handleFile = async () => {
    const year = Number(periodYear);
    const month = Number(periodMonth);
    setFiling(true);
    setFileMsg(null);
    try {
      const rec = await api.createPPNReconciliation({ period_year: year, period_month: month });
      setFiledRecord(rec);
      setFileMsg(`Filed: ${formatIDR(rec.net_ppn_cents)} net PPN for ${year}-${String(month).padStart(2, "0")}.`);
    } catch (err) {
      setFileMsg(err instanceof Error ? err.message : "Failed to file reconciliation.");
    } finally {
      setFiling(false);
    }
  };

  const monthLabel = useMemo(() => {
    const months = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
    const m = Number(periodMonth);
    return `${months[m - 1] ?? ""} ${periodYear}`;
  }, [periodYear, periodMonth]);

  const keluaran = recon?.ppn_keluaran_cents ?? summary?.ppn_keluaran_cents ?? 0;
  const masukan = recon?.ppn_masukan_cents ?? summary?.ppn_masukan_cents ?? 0;
  const net = recon?.net_ppn_cents ?? summary?.net_ppn_cents ?? 0;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>PPN Reconciliation</span>
          <small>SPT Masa PPN — output VAT (keluaran) vs input VAT (masukan) per month.</small>
        </div>
        <div className="listtab__toolbar">
          <label className="field-inline">
            <span>Year</span>
            <input className="input" type="number" value={periodYear} min={2000} max={2100} onChange={(e) => setPeriodYear(e.target.value)} style={{ width: 90 }} />
          </label>
          <label className="field-inline">
            <span>Month</span>
            <select className="input" value={periodMonth} onChange={(e) => setPeriodMonth(e.target.value)} style={{ width: 130 }}>
              {["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"].map((m, i) => (
                <option key={m} value={String(i + 1)}>{m}</option>
              ))}
            </select>
          </label>
          <M3Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >Reload</M3Button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Computing PPN..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div className="kpi-list" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12 }}>
              <PPNStat label={`PPN Keluaran — ${monthLabel}`} value={formatIDR(keluaran)} tone="neg" note="Output VAT (2202)" />
              <PPNStat label={`PPN Masukan — ${monthLabel}`} value={formatIDR(masukan)} tone="pos" note="Input VAT (1203)" />
              <PPNStat label="Net PPN Payable" value={formatIDR(Math.abs(net))} tone={net >= 0 ? "neg" : "pos"} note={net >= 0 ? "PAYABLE" : "EXCESS (carry forward)"} />
            </div>

            <section>
              <h3 style={{ margin: "0 0 8px", fontSize: 14, fontWeight: 600 }}>Detail Movements</h3>
              {recon && recon.lines.length > 0 ? (
                <div style={{ overflowX: "auto" }}>
                  <table className="table" style={{ width: "100%", fontSize: 13 }}>
                    <thead>
                      <tr>
                        <th style={{ textAlign: "left" }}>Date</th>
                        <th style={{ textAlign: "left" }}>Number</th>
                        <th style={{ textAlign: "left" }}>Description</th>
                        <th style={{ textAlign: "left" }}>Account</th>
                        <th style={{ textAlign: "left" }}>Type</th>
                        <th style={{ textAlign: "right" }}>Debit</th>
                        <th style={{ textAlign: "right" }}>Credit</th>
                      </tr>
                    </thead>
                    <tbody>
                      {recon.lines.map((l) => (
                        <tr key={`${l.entry_id}-${l.account_code}`}>
                          <td>{l.entry_date}</td>
                          <td>{l.entry_number}</td>
                          <td>{l.description || "—"}</td>
                          <td>{l.account_code} · {l.account_name}</td>
                          <td>{l.direction}</td>
                          <td style={{ textAlign: "right" }}>{l.debit_cents ? formatIDR(l.debit_cents) : "—"}</td>
                          <td style={{ textAlign: "right" }}>{l.credit_cents ? formatIDR(l.credit_cents) : "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <EmptyState title="No VAT movements" message={`No posted PPN transactions found for ${monthLabel}.`} />
              )}
            </section>

            <section style={{ borderTop: "1px solid var(--md-sys-color-outline-variant)", paddingTop: 16 }}>
              <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
                <Button onClick={() => void handleFile()} disabled={filing}>
                  {filing ? "Filing..." : "File Reconciliation"}
                </Button>
                <span style={{ fontSize: 13, color: "var(--md-sys-color-on-surface-variant)" }}>
                  Files the SPT Masa PPN record for {monthLabel} as FILED.
                </span>
              </div>
              {fileMsg ? (
                <p style={{ marginTop: 8, fontSize: 13, color: filedRecord ? "var(--md-sys-color-success)" : "var(--md-sys-color-error)" }}>{fileMsg}</p>
              ) : null}
            </section>
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{recon?.lines.length ?? 0} Movement(s)</span>
      </div>
    </div>
  );
}

function PPNStat({ label, value, tone, note }: { label: string; value: string; tone: "pos" | "neg" | "acc"; note: string }) {
  return (
    <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
      <div className="kpi-list__label">
        <span className="kpi-list__label-title">{label}</span>
        <span className="kpi-list__label-note">{note}</span>
      </div>
      <span className={`kpi-list__value is-${tone}`}>{value}</span>
      <span className={`kpi-list__dot is-${tone === "pos" ? "pos" : tone === "neg" ? "neg" : "warn"}`} aria-hidden="true" />
    </div>
  );
}
