import { useEffect, useState } from "react";
import { EmptyState, ErrorState, ListSkeleton } from "../../components/ui";
import { api } from "../../api";
import { formatIDRFromCents, todayISO, formatDate } from "../../lib/format";
import { showToast } from "../../lib/toast";
import { Button } from "../../components/m3";

/**
 * F-04/F-05: GET /aging/ar returns a FLAT shape (internal/aging/handler.go):
 * { as_of_date, total_cents, current_cents, bucket_1_30_cents,
 *   bucket_31_60_cents, bucket_61_90_cents, bucket_90_plus_cents, rows[] }.
 * There is no nested `summary` object — reading data.summary.* crashes.
 */
interface AgingRow {
  party_id: number;
  party_name: string;
  invoice_number: string;
  invoice_date: string;
  due_date: string;
  outstanding_cents: number;
  bucket: string;
  days_overdue: number;
}

export interface AgingReport {
  as_of_date: string;
  total_cents: number;
  current_cents: number;
  bucket_1_30_cents: number;
  bucket_31_60_cents: number;
  bucket_61_90_cents: number;
  bucket_90_plus_cents: number;
  rows: AgingRow[];
}

const BUCKET_LABELS: { key: keyof AgingReport; label: string; range: string }[] = [
  { key: "current_cents", label: "Current", range: "Not yet due" },
  { key: "bucket_1_30_cents", label: "1–30 days", range: "1 - 30" },
  { key: "bucket_31_60_cents", label: "31–60 days", range: "31 - 60" },
  { key: "bucket_61_90_cents", label: "61–90 days", range: "61 - 90" },
  { key: "bucket_90_plus_cents", label: ">90 days", range: "90+" },
];

function bucketTone(range: string): string {
  switch (range) {
    case "1 - 30":
      return "badge--green";
    case "31 - 60":
      return "badge--amber";
    case "61 - 90":
      return "badge--orange";
    case "90+":
      return "badge--red";
    default:
      return "";
  }
}

export function ARAgingList() {
  const [asOf, setAsOf] = useState<string>(todayISO());
  const [data, setData] = useState<AgingReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = async (date: string) => {
    setLoading(true);
    setError(null);
    try {
      setData((await api.getARAging(date || undefined)) as unknown as AgingReport);
    } catch (err) {
      // Human-readable message — surface ApiError.message when present.
      const detail =
        typeof (err as { message?: unknown } | null | undefined)?.message === "string"
          ? (err as { message: string }).message
          : "Could not load the AR aging report.";
      setError(`${detail} Check your connection or pick another as-of date, then retry.`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load(asOf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [asOf]);

  const handleExport = async (format: "pdf" | "xlsx") => {
    setExporting(format);
    try {
      const blob = await api.exportReport("ar_aging", { format, as_of: asOf });
      downloadBlob(blob, `ar-aging-${asOf}.${format}`);
      showToast(`AR aging exported as ${format.toUpperCase()}.`);
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Export failed.", "error");
    } finally {
      setExporting(null);
    }
  };

  const total = data?.total_cents ?? 0;
  const overdue =
    (data?.bucket_1_30_cents ?? 0) +
    (data?.bucket_31_60_cents ?? 0) +
    (data?.bucket_61_90_cents ?? 0) +
    (data?.bucket_90_plus_cents ?? 0);
  const percent = (amount: number): string =>
    total === 0 ? "0%" : `${((amount / total) * 100).toFixed(1)}%`;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>AR Aging</span>
          <small>Receivables by age bucket as of the selected date.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">As of</span>
            <input
              type="date"
              className="filter-pill__input"
              value={asOf}
              onChange={(e) => setAsOf(e.target.value)}
            />
          </label>
        </div>
        <div className="listtab__actions" style={{ marginLeft: "auto" }}>
          <Button
            variant="outlined"
            size="sm"
            disabled={exporting !== null || loading}
            onClick={() => void handleExport("pdf")}
          >
            {exporting === "pdf" ? "Exporting..." : "Export PDF"}
          </Button>
          <Button
            variant="outlined"
            size="sm"
            disabled={exporting !== null || loading}
            onClick={() => void handleExport("xlsx")}
          >
            {exporting === "xlsx" ? "Exporting..." : "Export Excel"}
          </Button>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <ListSkeleton />
        ) : error ? (
          <ErrorState message={error} onRetry={() => load(asOf)} />
        ) : !data ? (
          <EmptyState title="No AR aging data" message="Pick an as-of date to view the report." />
        ) : (
          <>
            <AgingSummaryCards report={data} overdue={overdue} />
            <AgingTable report={data} percent={percent} />
          </>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">
          {data ? `As of ${formatDate(data.as_of_date)} · ${data.rows.length} invoices` : ""}
        </span>
      </div>
    </div>
  );
}

function AgingSummaryCards({ report, overdue }: { report: AgingReport; overdue: number }) {
  return (
    <div className="aging-summary">
      <div className="aging-summary__card">
        <span className="aging-summary__label">Total Receivable</span>
        <span className="aging-summary__value is-green">{formatIDRFromCents(report.total_cents)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Overdue</span>
        <span className="aging-summary__value is-red">{formatIDRFromCents(overdue)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Current</span>
        <span className="aging-summary__value is-green">{formatIDRFromCents(report.current_cents)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Invoices</span>
        <span className="aging-summary__value">{report.rows.length}</span>
      </div>
    </div>
  );
}

function AgingTable({ report, percent }: { report: AgingReport; percent: (amount: number) => string }) {
  return (
    <>
      <table className="table table--striped">
        <thead>
          <tr>
            <th>Bucket</th>
            <th className="right">Amount</th>
            <th className="right">% of Total</th>
          </tr>
        </thead>
        <tbody>
          {BUCKET_LABELS.map(({ key, label, range }) => {
            const amount = Number(report[key] ?? 0);
            return (
              <tr key={key}>
                <td>
                  <span className={`badge ${bucketTone(range)}`}>{label}</span>
                </td>
                <td className="right">{formatIDRFromCents(amount)}</td>
                <td className="right">{percent(amount)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {report.rows.length > 0 && (
        <table className="table table--striped" style={{ marginTop: 16 }}>
          <thead>
            <tr>
              <th>Customer</th>
              <th>Invoice</th>
              <th>Due Date</th>
              <th>Bucket</th>
              <th className="right">Outstanding</th>
              <th className="right">Days</th>
            </tr>
          </thead>
          <tbody>
            {report.rows.map((r, i) => (
              <tr key={`${r.party_id}-${r.invoice_number}-${i}`}>
                <td>{r.party_name || `#${r.party_id}`}</td>
                <td>{r.invoice_number}</td>
                <td>{formatDate(r.due_date)}</td>
                <td>{r.bucket}</td>
                <td className="right">{formatIDRFromCents(r.outstanding_cents)}</td>
                <td className="right">{Math.max(r.days_overdue, 0)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
