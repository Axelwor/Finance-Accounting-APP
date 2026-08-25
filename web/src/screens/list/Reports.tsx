import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { EmptyState, ErrorState, LoadingState, MultiSelectCombobox } from "../../components/ui";
import { api } from "../../api";
import { formatIDRFromCents } from "../../lib/format";
import { showToast } from "../../lib/toast";
import { useAppState } from "../../state";
import type { Dimension, ListSubKind, ReportFrameworkRecord, ReportFramework } from "../../types";
import { Button } from "../../components/m3";

/* --------------------------- Date range helpers --------------------------- */

function iso(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function endOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0);
}

/** Start month (0-based) of the calendar quarter containing `date`. */
function quarterStartMonth(date: Date): number {
  return Math.floor(date.getMonth() / 3) * 3;
}

export type QuickRange =
  | "this-month"
  | "this-quarter"
  | "ytd"
  | "last-month"
  | "last-quarter"
  | "fiscal-year";

export interface ReportRange {
  /** YYYY-MM-DD; "" = no lower bound. */
  from: string;
  /** YYYY-MM-DD; "" = no upper bound. */
  to: string;
  /** Quick preset that produced the current range (null = picked manually). */
  preset: QuickRange | null;
}

/**
 * The date range is shared by all four report tabs through a tiny
 * module-level store: pick a window once and Trial Balance, P&L, Balance
 * Sheet and Cash Flow all report on the same period — and keep it when the
 * user switches between report tabs. Defaults: current month start → today.
 */
let currentRange: ReportRange = {
  from: iso(startOfMonth(new Date())),
  to: iso(new Date()),
  preset: null,
};
const rangeListeners = new Set<() => void>();

function subscribeRange(listener: () => void): () => void {
  rangeListeners.add(listener);
  return () => {
    rangeListeners.delete(listener);
  };
}

function getRangeSnapshot(): ReportRange {
  return currentRange;
}

export function useReportRange(): ReportRange {
  return useSyncExternalStore(subscribeRange, getRangeSnapshot);
}

/** Merges a patch into the shared range. Manual edits clear the preset. */
export function setReportRange(patch: Partial<ReportRange>): void {
  currentRange = { ...currentRange, preset: null, ...patch };
  rangeListeners.forEach((listener) => listener());
}

const QUICK_RANGES: { id: QuickRange; label: string }[] = [
  { id: "this-month", label: "This Month" },
  { id: "this-quarter", label: "This Quarter" },
  { id: "ytd", label: "YTD" },
  { id: "last-month", label: "Last Month" },
  { id: "last-quarter", label: "Last Quarter" },
  { id: "fiscal-year", label: "Fiscal Year" },
];

/**
 * Computes the bounds of a quick preset. Month / quarter / fiscal presets are
 * full periods; YTD runs from 1 January to today. The fiscal year is the one
 * containing today, based on the business's configured start month.
 */
function quickRangeBounds(preset: QuickRange, fiscalYearStartMonth: number): { from: Date; to: Date } {
  const now = new Date();
  switch (preset) {
    case "this-month":
      return { from: startOfMonth(now), to: endOfMonth(now) };
    case "last-month": {
      const prev = new Date(now.getFullYear(), now.getMonth() - 1, 1);
      return { from: startOfMonth(prev), to: endOfMonth(prev) };
    }
    case "this-quarter": {
      const startMonth = quarterStartMonth(now);
      return {
        from: new Date(now.getFullYear(), startMonth, 1),
        to: new Date(now.getFullYear(), startMonth + 3, 0),
      };
    }
    case "last-quarter": {
      const startMonth = quarterStartMonth(now) - 3;
      return {
        from: new Date(now.getFullYear(), startMonth, 1),
        to: new Date(now.getFullYear(), startMonth + 3, 0),
      };
    }
    case "ytd":
      return { from: new Date(now.getFullYear(), 0, 1), to: now };
    case "fiscal-year": {
      const startYear =
        now.getMonth() + 1 >= fiscalYearStartMonth ? now.getFullYear() : now.getFullYear() - 1;
      return {
        from: new Date(startYear, fiscalYearStartMonth - 1, 1),
        to: new Date(startYear + 1, fiscalYearStartMonth - 1, 0),
      };
    }
  }
}

/**
 * Shared date-range toolbar rendered at the top of every report tab: From/To
 * pickers plus quick presets. Writes to the shared range store, so all
 * report tabs stay on the same window.
 */
export function ReportRangeBar() {
  const range = useReportRange();
  const { business } = useAppState();
  const fiscalYearStartMonth = business?.fiscalYearStart ?? 1;
  const invalid = range.from !== "" && range.to !== "" && range.from > range.to;

  return (
    <div className="report-range" role="group" aria-label="Report date range">
      <div className="report-range__dates">
        <label>
          From
          <input
            type="date"
            value={range.from}
            onChange={(e) => setReportRange({ from: e.target.value })}
            aria-label="From date"
          />
        </label>
        <label>
          To
          <input
            type="date"
            value={range.to}
            onChange={(e) => setReportRange({ to: e.target.value })}
            aria-label="To date"
          />
        </label>
      </div>
      <div className="report-range__presets">
        {QUICK_RANGES.map((preset) => (
          <button
            key={preset.id}
            type="button"
            className={`report-range__preset${range.preset === preset.id ? " is-active" : ""}`}
            aria-pressed={range.preset === preset.id}
            onClick={() => {
              const bounds = quickRangeBounds(preset.id, fiscalYearStartMonth);
              setReportRange({ from: iso(bounds.from), to: iso(bounds.to), preset: preset.id });
            }}
          >
            {preset.label}
          </button>
        ))}
      </div>
      {invalid ? (
        <p className="report-range__warn" role="alert">
          From date is after To date.
        </p>
      ) : null}
    </div>
  );
}

/* -------------------------------- Export --------------------------------- */

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  // Give the browser a beat to start the download before revoking the URL.
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function exportFilename(title: string, format: "pdf" | "xlsx", range: ReportRange): string {
  const base = title.replace(/[^a-z0-9]+/gi, "_");
  const span =
    range.from && range.to
      ? `${range.from}_${range.to}`
      : range.to
        ? `as_of_${range.to}`
        : range.from
          ? `from_${range.from}`
          : "all";
  return `${base}_${span}.${format}`;
}

/**
 * Export PDF / Export Excel buttons shown in each report card header. They
 * export the same window + filters currently on screen, show a busy state
 * while the file is generated, and toast on failure.
 */
function ExportButtons({
  reportType,
  title,
  framework,
  dimensionIds,
}: {
  /** URL slug of the report, e.g. "trial-balance". */
  reportType: string;
  title: string;
  framework?: string;
  dimensionIds?: number[];
}) {
  const range = useReportRange();
  const [busy, setBusy] = useState<"pdf" | "xlsx" | null>(null);

  const run = async (format: "pdf" | "xlsx") => {
    if (busy) return;
    setBusy(format);
    try {
      const blob = await api.exportReport(reportType, {
        format,
        from_date: range.from || undefined,
        to_date: range.to || undefined,
        framework: framework || undefined,
        dimension_ids: dimensionIds,
      });
      downloadBlob(blob, exportFilename(title, format, range));
    } catch (err) {
      showToast(extractErrorMessage(err, `Export to ${format.toUpperCase()} failed.`), "error");
    } finally {
      setBusy(null);
    }
  };

  return (
    <>
      <Button
        variant="outlined"
        size="sm"
        disabled={busy !== null}
        onClick={() => void run("pdf")}
      >
        {busy === "pdf" ? "Exporting..." : "Export PDF"}
      </Button>
      <Button
        variant="outlined"
        size="sm"
        disabled={busy !== null}
        onClick={() => void run("xlsx")}
      >
        {busy === "xlsx" ? "Exporting..." : "Export Excel"}
      </Button>
    </>
  );
}

/**
 * Extract a human-readable message from a thrown value. The API layer throws
 * ApiError ({ code, message }) objects rather than Error instances, so a plain
 * `err instanceof Error` check would hide the real backend message behind a
 * generic fallback. Handle both shapes.
 */
function extractErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === "object" && "message" in err && typeof (err as { message: unknown }).message === "string") {
    return (err as { message: string }).message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

/* ----------------------- Framework & dimension filters ------------------- */

const FRAMEWORK_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "Default" },
  { value: "EMKM", label: "EMKM (UMKM)" },
  { value: "ETAP", label: "ETAP (Menengah)" },
  { value: "SAK_UMUM", label: "SAK Umum (PSAK)" },
];

/**
 * Framework selector state for P&L, Balance Sheet and Cash Flow. Loads the
 * tenant's configured default on mount (US-090A) and persists any change as
 * the new tenant default, so every report stays on the same presentation.
 */
function useReportFramework() {
  const [framework, setFramework] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [message, setMessage] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  useEffect(() => {
    void api
      .listReportFrameworks()
      .then((records: ReportFrameworkRecord[]) => {
        const def = records.find((r) => r.is_default) ?? records[0];
        if (def?.framework) setFramework(def.framework);
      })
      .catch(() => {})
      .finally(() => setLoaded(true));
  }, []);

  const changeFramework = async (next: string) => {
    setFramework(next);
    setMessage(null);
    if (!next) return; // "Default" — nothing to persist.
    try {
      await api.setReportFramework({ framework: next as ReportFramework, is_default: true });
      setMessage({ kind: "ok", text: `Default framework set to ${next}.` });
    } catch (err) {
      setMessage({
        kind: "err",
        text: extractErrorMessage(err, "Could not save the default framework."),
      });
    }
  };

  return { framework, loaded, message, changeFramework };
}

/** Dimension master list + selected dimension ids (empty = all), per US-093. */
function useDimensionFilter() {
  const [dimensions, setDimensions] = useState<Dimension[]>([]);
  const [dimensionIds, setDimensionIds] = useState<number[]>([]);

  useEffect(() => {
    void api.listDimensions().then(setDimensions).catch(() => setDimensions([]));
  }, []);

  return { dimensions, dimensionIds, setDimensionIds };
}

function FrameworkSelect({
  framework,
  loaded,
  message,
  onChange,
}: {
  framework: string;
  loaded: boolean;
  message: { kind: "ok" | "err"; text: string } | null;
  onChange: (next: string) => void;
}) {
  return (
    <>
      <label className="report-toolbar__label">
        Framework
        <select
          className="report-toolbar__select"
          value={framework}
          onChange={(e) => onChange(e.target.value)}
          disabled={!loaded}
          aria-label="Report framework"
        >
          {FRAMEWORK_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </label>
      {message ? (
        <span
          className={`report-toolbar__note${message.kind === "err" ? " is-err" : ""}`}
          role={message.kind === "err" ? "alert" : undefined}
        >
          {message.text}
        </span>
      ) : null}
    </>
  );
}

/**
 * Dimension multi-select combobox listing all dimensions; hidden when none
 * exist. Empty selection = all dimensions (no filter).
 */
function DimensionMultiSelect({
  dimensions,
  dimensionIds,
  onChange,
}: {
  dimensions: Dimension[];
  dimensionIds: number[];
  onChange: (next: number[]) => void;
}) {
  if (dimensions.length === 0) return null;
  return (
    <MultiSelectCombobox
      ariaLabel="Filter by dimension"
      placeholder="All dimensions"
      options={dimensions.map((d) => ({ value: d.id, label: `${d.code} · ${d.name}` }))}
      selected={dimensionIds}
      onChange={onChange}
    />
  );
}

/* ------------------------------ Report shell ------------------------------ */

interface ReportConfig {
  listKind: ListSubKind;
  title: string;
  description: string;
  /** Report slug used by the export endpoint, e.g. "trial-balance". */
  exportSlug: string;
  /** Fetches the report for the active date range + report-specific filters. */
  fetcher: (range: { from?: string; to?: string }) => Promise<unknown>;
  /** Renders the data shape. */
  render: (data: any) => ReactNode;
  emptyMessage: string;
  /** Extra toolbar controls (framework / dimension selectors). */
  filters?: ReactNode;
  /** Framework / dimensions currently on screen, echoed into exports. */
  exportFramework?: string;
  exportDimensionIds?: number[];
  /** Refetch whenever one of these report-specific filter values changes. */
  deps?: unknown[];
}

/**
 * Generic reports tab. Reports are read-only — no entry form, no column
 * picker. The shell owns the shared date-range bar, the export buttons and
 * the fetch lifecycle; each report supplies its own fetcher + renderer so
 * the data shape can vary (single-line P&L vs multi-row trial balance).
 */
export function ReportTab({ config }: { config: ReportConfig }) {
  const range = useReportRange();
  const [data, setData] = useState<unknown | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const invalidRange = range.from !== "" && range.to !== "" && range.from > range.to;

  const load = async () => {
    if (invalidRange) {
      setLoading(false);
      setData(null);
      setError("The From date is after the To date. Adjust the date range to load the report.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const result = await config.fetcher({
        from: range.from || undefined,
        to: range.to || undefined,
      });
      setData(result);
    } catch (err) {
      setError(extractErrorMessage(err, "Failed to load the report."));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.from, range.to, invalidRange, ...(config.deps ?? [])]);

  return (
    <div className="listtab">
      <ReportRangeBar />
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{config.title}</span>
          <small>{config.description}</small>
        </div>
        <div className="listtab__toolbar report-toolbar">
          {config.filters}
          <ExportButtons
            reportType={config.exportSlug}
            title={config.title}
            framework={config.exportFramework}
            dimensionIds={config.exportDimensionIds}
          />
          <Button
            variant="outlined"
            size="sm"
            onClick={() => void load()}
          >
            Reload
          </Button>
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

export function TrialBalanceReport() {
  return (
    <ReportTab
      config={{
        listKind: "report-trial-balance",
        title: "Trial Balance",
        description: "All accounts, debit vs credit",
        exportSlug: "trial-balance",
        fetcher: ({ from, to }) => api.getTrialBalance(from, to),
        emptyMessage: "No journal entries in the selected date range.",
        render: (data: any) => {
          const rows: { account_code: string; account_name: string; debit_cents: number; credit_cents: number }[] = data.rows ?? [];
          const totalDebitCents: number = data.total_debit_cents
            ?? rows.reduce((sum, r) => sum + (r.debit_cents || 0), 0);
          const totalCreditCents: number = data.total_credit_cents
            ?? rows.reduce((sum, r) => sum + (r.credit_cents || 0), 0);
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
                <>
                  {rows.map((r, i) => (
                    <div className="ledger-table__row" key={i}>
                      <span className="ledger-table__date" style={{ fontFamily: "var(--md-ref-typeface-plain)" }}>{r.account_code}</span>
                      <div className="ledger-table__desc">
                        <div className="ledger-table__desc-text">
                          <span className="ledger-table__desc-title">{r.account_name}</span>
                        </div>
                      </div>
                      <span className={`ledger-table__amount ${r.debit_cents > 0 ? "" : "is-muted"}`}>
                        {r.debit_cents > 0 ? formatIDRFromCents(r.debit_cents) : "—"}
                      </span>
                      <span className={`ledger-table__amount ${r.credit_cents > 0 ? "" : "is-muted"}`}>
                        {r.credit_cents > 0 ? formatIDRFromCents(r.credit_cents) : "—"}
                      </span>
                      <span aria-hidden="true" />
                    </div>
                  ))}
                  <div className="ledger-table__row total-rule-top total-double" style={{ fontWeight: 700 }}>
                    <span />
                    <div className="ledger-table__desc">
                      <div className="ledger-table__desc-text">
                        <span className="ledger-table__desc-title">TOTAL</span>
                      </div>
                    </div>
                    <span className="ledger-table__amount">{formatIDRFromCents(totalDebitCents)}</span>
                    <span className="ledger-table__amount">{formatIDRFromCents(totalCreditCents)}</span>
                    <span aria-hidden="true" />
                  </div>
                </>
              )}
            </div>
          );
        },
      }}
    />
  );
}

/**
 * Profit & Loss report with optional framework presentation and dimension
 * filter (US-090A + US-093). The same posted totals are re-presented by the
 * selected framework; the dimension filter narrows to journal lines tagged
 * with a cabang/proyek dimension.
 */
export function ProfitLossReport() {
  const { framework, loaded, message, changeFramework } = useReportFramework();
  const { dimensions, dimensionIds, setDimensionIds } = useDimensionFilter();

  return (
    <ReportTab
      config={{
        listKind: "report-profit-loss",
        title: "Profit & Loss",
        description: `Revenue minus expenses${framework ? ` · ${framework} presentation` : ""}`,
        exportSlug: "profit-loss",
        exportFramework: framework || undefined,
        exportDimensionIds: dimensionIds,
        deps: [framework, dimensionIds],
        fetcher: ({ from, to }) =>
          api.getProfitLoss(from, to, framework || undefined, dimensionIds),
        filters: (
          <>
            <FrameworkSelect
              framework={framework}
              loaded={loaded}
              message={message}
              onChange={(next) => void changeFramework(next)}
            />
            <DimensionMultiSelect dimensions={dimensions} dimensionIds={dimensionIds} onChange={setDimensionIds} />
          </>
        ),
        emptyMessage: "No revenue or expenses in the selected date range.",
        render: (data: any) => {
          const r = data;
          const net = (r?.profit_cents ?? 0) as number;
          const isProfit = net >= 0;
          const sections: { code: string; label: string; amount_cents: number }[] = r?.sections ?? [];
          return (
            <>
              <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
                <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr" }}>
                  <Stat label="Revenue" value={formatIDRFromCents(r.revenue_cents ?? 0)} tone="pos" />
                  <Stat label="Expense" value={formatIDRFromCents(r.expense_cents ?? 0)} tone="neg" />
                  <Stat label="Net" value={formatIDRFromCents(Math.abs(net))} tone={isProfit ? "pos" : "neg"} suffix={isProfit ? "PROFIT" : "LOSS"} />
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
                          <tr key={s.code} style={{ borderBottom: "1px solid var(--md-sys-color-outline-variant)" }}>
                            <td style={{ padding: "8px 12px" }}>{s.label}</td>
                            <td style={{ padding: "8px 12px", textAlign: "right", fontFamily: "var(--md-ref-typeface-plain)" }}>
                              {formatIDRFromCents(s.amount_cents)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : null}
            </>
          );
        },
      }}
    />
  );
}

export function BalanceSheetReport() {
  const { framework, loaded, message, changeFramework } = useReportFramework();
  const { dimensions, dimensionIds, setDimensionIds } = useDimensionFilter();

  return (
    <ReportTab
      config={{
        listKind: "report-balance-sheet",
        title: "Balance Sheet",
        description: `Assets = Liabilities + Equity${framework ? ` · ${framework} presentation` : ""}`,
        exportSlug: "balance-sheet",
        exportFramework: framework || undefined,
        exportDimensionIds: dimensionIds,
        deps: [framework, dimensionIds],
        fetcher: ({ from, to }) =>
          api.getBalanceSheet(from, to, framework || undefined, dimensionIds),
        filters: (
          <>
            <FrameworkSelect
              framework={framework}
              loaded={loaded}
              message={message}
              onChange={(next) => void changeFramework(next)}
            />
            <DimensionMultiSelect dimensions={dimensions} dimensionIds={dimensionIds} onChange={setDimensionIds} />
          </>
        ),
        emptyMessage: "No balance sheet data in the selected date range.",
        render: (data: any) => {
          const r = data;
          const assets = r.asset_cents ?? 0;
          const liabilities = r.liability_cents ?? 0;
          const equity = r.equity_cents ?? 0;
          // F-06: current-period profit from the backend so the identity
          // A = L + E + Laba Berjalan is visible before period close. The
          // backend folds profit into equity, so `balanced` stays authoritative;
          // when absent, verify locally (folded or unfolded both accepted).
          const profit = r.profit_cents ?? 0;
          const localBalanced =
            assets === liabilities + equity || assets === liabilities + equity + profit;
          const balanced = typeof r.balanced === "boolean" ? r.balanced : localBalanced;
          return (
            <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr" }}>
                <Stat label="Assets" value={formatIDRFromCents(assets)} tone="pos" />
                <Stat label="Liabilities" value={formatIDRFromCents(liabilities)} tone="neg" />
                <Stat label="Equity" value={formatIDRFromCents(equity)} tone="acc" />
              </div>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr", marginTop: 12 }}>
                <Stat
                  label="Laba Berjalan (Current Profit)"
                  value={formatIDRFromCents(Math.abs(profit))}
                  tone={profit >= 0 ? "pos" : "neg"}
                  suffix={profit >= 0 ? "" : "LOSS"}
                />
                <Stat
                  label="Balance — A = L + E + Laba"
                  value={
                    balanced && assets === liabilities + equity + profit
                      ? "BALANCED"
                      : balanced
                        ? "BALANCED (profit di dalam Equity)"
                        : "OFF"
                  }
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
  const { framework, loaded, message, changeFramework } = useReportFramework();
  const { dimensions, dimensionIds, setDimensionIds } = useDimensionFilter();

  return (
    <ReportTab
      config={{
        listKind: "report-cash-flow",
        title: "Cash Flow",
        description: `Net cash movement across cash and bank accounts${framework ? ` · ${framework} presentation` : ""}`,
        exportSlug: "cash-flow",
        exportFramework: framework || undefined,
        exportDimensionIds: dimensionIds,
        deps: [framework, dimensionIds],
        fetcher: ({ from, to }) =>
          api.getCashFlow(from, to, framework || undefined, dimensionIds),
        filters: (
          <>
            <FrameworkSelect
              framework={framework}
              loaded={loaded}
              message={message}
              onChange={(next) => void changeFramework(next)}
            />
            <DimensionMultiSelect dimensions={dimensions} dimensionIds={dimensionIds} onChange={setDimensionIds} />
          </>
        ),
        emptyMessage: "No cash movement in the selected date range.",
        render: (data: any) => {
          const r = data;
          const net = r.net_cash_flow_cents ?? 0;
          return (
            <div className="entrytab__body" style={{ background: "transparent", border: 0 }}>
              <div className="entrytab__section" style={{ gridTemplateColumns: "1fr 1fr 1fr" }}>
                <Stat label="Inflow" value={formatIDRFromCents(r.inflow_cents ?? 0)} tone="pos" />
                <Stat label="Outflow" value={formatIDRFromCents(r.outflow_cents ?? 0)} tone="neg" />
                <Stat label="Net" value={formatIDRFromCents(Math.abs(net))} tone={net >= 0 ? "pos" : "neg"} suffix={net >= 0 ? "POSITIVE" : "NEGATIVE"} />
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
    <div className="kpi-list__row" style={{ background: "var(--md-sys-color-surface-container-lowest)", border: "1px solid var(--md-sys-color-outline-variant)", borderRadius: "var(--md-sys-shape-corner-extra-small)" }}>
      <div className="kpi-list__label">
        <span className="kpi-list__label-title">{label}</span>
        {suffix ? <span className="kpi-list__label-note">{suffix}</span> : null}
      </div>
      <span className={`kpi-list__value is-${tone}`}>{value}</span>
      <span className={`kpi-list__dot is-${tone === "pos" ? "pos" : tone === "neg" ? "neg" : "warn"}`} aria-hidden="true" />
    </div>
  );
}
