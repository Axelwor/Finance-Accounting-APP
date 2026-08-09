import { useEffect, useMemo, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { Dimension, ListSubKind, ReportFrameworkRecord } from "../../types";

interface ReportConfig {
  listKind: ListSubKind;
  title: string;
  description: string;
  fetcher: () => Promise<unknown>;
  /** Renders the data shape. */
  render: (data: any) => React.ReactNode;
  emptyMessage: string;
}

/**
 * Generic reports tab. Reports are read-only — no entry form, no
 * column picker. Each report owns its own fetcher + render so the
 * data shape can vary (single-line P&L vs multi-row trial balance).
 */
export function ReportTab({ config }: { config: ReportConfig }) {
  const [data, setData] = useState<unknown | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await config.fetcher();
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the report.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="listtab">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{config.title}</span>
          <small>{config.description}</small>
        </div>
        <div className="listtab__toolbar">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Computing..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : data ? (
          config.render(data)
        ) : (
          <EmptyState title="No data" message={config.emptyMessage} />
        )}
      </div>
    </div>
  );
}

/* ---------------------- Individual report renderers ---------------------- */

function fmtIDR(cents: number): string {
  return formatIDR(cents);
}

export function TrialBalanceReport() {
  return (
    <ReportTab
      config={{
        listKind: "report-trial-balance",
        title: "Trial Balance",
        description: "All accounts, debit vs credit",
        fetcher: () => api.getTrialBalance(),
        emptyMessage: "No journal entries yet.",
        render: (data: any) => {
          const rows: { account_code: string; account_name: string; debit_cents: number; credit_cents: number }[] = data.rows ?? [];
          return (
            <div className="ledger-table">
              <div className="ledger-table__head">
                <span>Code</span>
                <span>Account</span>
                <span className="right">Debit</span>
                <span className="right">Credit</span>
                <span aria-hidden="true" />
              </div>
              {rows.length === 0 ? (
                <div className="empty-state">
                  <p className="empty-state__title">Balanced</p>
                  <p className="empty-state__message">All accounts are at zero. Rule your first entry from the Cash & Bank tab.</p>
                </div>
              ) : (
                rows.map((r, i) => (
                  <div className="ledger-table__row" key={i}>
                    <span className="ledger-table__date" style={{ fontFamily: "var(--font-mono)" }}>{r.account_code}</span>
                    <div className="ledger-table__desc">
                      <div className="ledger-table__desc-text">
                        <span className="ledger-table__desc-title">{r.account_name}</span>
                      </div>
                    </div>
                    <span className={`ledger-table__amount ${r.debit_cents > 0 ? "" : "is-muted"}`}>
                      {r.debit_cents > 0 ? fmtIDR(r.debit_cents) : "—"}
                    </span>
                    <span className={`ledger-table__amount ${r.credit_cents > 0 ? "" : "is-muted"}`}>
                      {r.credit_cents > 0 ? fmtIDR(r.credit_cents) : "—"}
                    </span>
                    <span aria-hidden="true" />
                  </div>
                ))
              )}
            </div>
          );
        },
      }}
    />
  );
}

const FRAMEWORK_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "Default" },
  { value: "EMKM", label: "EMKM (UMKM)" },
  { value: "ETAP", label: "ETAP (Menengah)" },
  { value: "SAK_UMUM", label: "SAK Umum (PSAK)" },
];

/**
 * Profit & Loss report with optional framework presentation and dimension
 * filter (US-090A + US-093). The same posted totals are re-presented by the
 * selected framework; the dimension filter narrows to journal lines tagged
 * with a cabang/proyek dimension.
 */
export function ProfitLossReport() {
  const [framework, setFramework] = useState("");
  const [dimensionId, setDimensionId] = useState(0);
  const [dimensions, setDimensions] = useState<Dimension[]>([]);
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.getProfitLoss(undefined, undefined, framework || undefined, dimensionId || undefined);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the report.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [framework, dimensionId]);

  useEffect(() => {
    void api.listDimensions().then(setDimensions).catch(() => setDimensions([]));
  }, []);

  const r = data;
  const net = (r?.profit_cents ?? 0) as number;
  const isProfit = net >= 0;
  const sections: { code: string; label: string; amount_cents: number }[] = r?.sections ?? [];

  return (
    <div className="listtab">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Profit &amp; Loss</span>
          <small>Revenue minus expenses {framework ? `· ${framework} presentation` : ""}</small>
        </div>
        <div className="listtab__toolbar" style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <label style={{ fontSize: 12, color: "var(--text-secondary)" }}>Framework</label>
          <select
            className="field__input"
            value={framework}
            onChange={(e) => setFramework(e.target.value)}
            style={{ minWidth: 180, padding: "4px 8px" }}
          >
            {FRAMEWORK_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {dimensions.length > 0 ? (
            <>
              <label style={{ fontSize: 12, color: "var(--text-secondary)", marginLeft: 8 }}>Dimension</label>
              <select
                className="field__input"
                value={dimensionId}
                onChange={(e) => setDimensionId(Number(e.target.value))}
                style={{ minWidth: 160, padding: "4px 8px" }}
              >
                <option value={0}>All</option>
                {dimensions.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.code} · {d.name}
                  </option>
                ))}
              </select>
            </>
          ) : null}
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Computing..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : r ? (
          <>
            <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr" }}>
                <Stat label="Revenue" value={fmtIDR(r.revenue_cents ?? 0)} tone="pos" />
                <Stat label="Expense" value={fmtIDR(r.expense_cents ?? 0)} tone="neg" />
                <Stat label="Net" value={fmtIDR(Math.abs(net))} tone={isProfit ? "pos" : "neg"} suffix={isProfit ? "PROFIT" : "LOSS"} />
              </div>
            </div>

            {sections.length > 0 ? (
              <div style={{ marginTop: 24 }}>
                <div className="listtab__title" style={{ marginBottom: 12 }}>
                  <span>Breakdown ({framework})</span>
                  <small>Same data, framework presentation</small>
                </div>
                <div style={{ overflowX: "auto" }}>
                  <table className="data-table" style={{ width: "100%", borderCollapse: "collapse" }}>
                    <thead>
                      <tr>
                        <th style={{ textAlign: "left", padding: "8px 12px" }}>Section</th>
                        <th style={{ textAlign: "right", padding: "8px 12px" }}>Amount</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sections.map((s) => (
                        <tr key={s.code} style={{ borderBottom: "1px solid var(--rule)" }}>
                          <td style={{ padding: "8px 12px" }}>{s.label}</td>
                          <td style={{ padding: "8px 12px", textAlign: "right", fontFamily: "var(--font-mono)" }}>
                            {fmtIDR(s.amount_cents)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : null}
          </>
        ) : (
          <EmptyState title="No data" message="No revenue or expenses recorded." />
        )}
      </div>
    </div>
  );
}

export function BalanceSheetReport() {
  return (
    <ReportTab
      config={{
        listKind: "report-balance-sheet",
        title: "Balance Sheet",
        description: "Assets = Liabilities + Equity at the current date",
        fetcher: () => api.getBalanceSheet(),
        emptyMessage: "No balance sheet yet.",
        render: (data: any) => {
          const r = data;
          const balanced = r.balanced === true;
          return (
            <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr 1fr" }}>
                <Stat label="Assets" value={fmtIDR(r.asset_cents ?? 0)} tone="pos" />
                <Stat label="Liabilities" value={fmtIDR(r.liability_cents ?? 0)} tone="neg" />
                <Stat label="Equity" value={fmtIDR(r.equity_cents ?? 0)} tone="acc" />
                <Stat
                  label="Balance"
                  value={balanced ? "BALANCED" : "OFF"}
                  tone={balanced ? "pos" : "neg"}
                  suffix=""
                />
              </div>
            </div>
          );
        },
      }}
    />
  );
}

export function CashFlowReport() {
  return (
    <ReportTab
      config={{
        listKind: "report-cash-flow",
        title: "Cash Flow",
        description: "Net cash movement across cash and bank accounts",
        fetcher: () => api.getCashFlow(),
        emptyMessage: "No cash movement yet.",
        render: (data: any) => {
          const r = data;
          const net = r.net_cash_flow_cents ?? 0;
          return (
            <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr" }}>
                <Stat label="Inflow" value={fmtIDR(r.inflow_cents ?? 0)} tone="pos" />
                <Stat label="Outflow" value={fmtIDR(r.outflow_cents ?? 0)} tone="neg" />
                <Stat label="Net" value={fmtIDR(Math.abs(net))} tone={net >= 0 ? "pos" : "neg"} suffix={net >= 0 ? "POSITIVE" : "NEGATIVE"} />
              </div>
            </div>
          );
        },
      }}
    />
  );
}

function Stat({ label, value, tone, suffix }: { label: string; value: string; tone: "pos" | "neg" | "acc"; suffix?: string }) {
  return (
    <div className="kpi-list__row" style={{ background: "var(--surface-panel)", border: "1px solid var(--rule)", borderRadius: "var(--radius-sm)" }}>
      <div className="kpi-list__label">
        <span className="kpi-list__label-title">{label}</span>
        {suffix ? <span className="kpi-list__label-note">{suffix}</span> : null}
      </div>
      <span className={`kpi-list__value is-${tone}`}>{value}</span>
      <span className={`kpi-list__dot is-${tone === "pos" ? "pos" : tone === "neg" ? "neg" : "warn"}`} aria-hidden="true" />
    </div>
  );
}
