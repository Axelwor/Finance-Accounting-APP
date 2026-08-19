import { useEffect, useState } from "react";
import { EmptyState, ErrorState, ListSkeleton } from "../../components/ui";
import { api } from "../../api";
import { formatIDR, todayISO, formatDate } from "../../lib/format";
import { showToast } from "../../lib/toast";
import type { AgingReport } from "../../types";
import { Button } from "../../components/m3";

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
      setData(await api.getARAging(date || undefined));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load AR aging.");
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

  const total = data?.summary.total_receivable_cents ?? 0;
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
            <AgingSummaryCards report={data} />
            <AgingTable report={data} percent={percent} />
          </>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">
          {data ? `As of ${formatDate(data.asOf)} · ${data.summary.total_invoices} invoices` : ""}
        </span>
      </div>
    </div>
  );
}

function AgingSummaryCards({ report }: { report: AgingReport }) {
  const { total_receivable_cents, overdue_amount_cents, total_invoices } = report.summary;
  return (
    <div className="aging-summary">
      <div className="aging-summary__card">
        <span className="aging-summary__label">Total Receivable</span>
        <span className="aging-summary__value is-green">{formatIDR(total_receivable_cents)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Overdue</span>
        <span className="aging-summary__value is-red">{formatIDR(overdue_amount_cents)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Current</span>
        <span className="aging-summary__value is-green">{formatIDR(total_receivable_cents - overdue_amount_cents)}</span>
      </div>
      <div className="aging-summary__card">
        <span className="aging-summary__label">Invoices</span>
        <span className="aging-summary__value">{total_invoices}</span>
      </div>
    </div>
  );
}

function AgingTable({ report, percent }: { report: AgingReport; percent: (amount: number) => string }) {
  return (
    <table className="table table--striped">
      <thead>
        <tr>
          <th>Bucket</th>
          <th className="right">Amount</th>
          <th className="right">Count</th>
          <th className="right">% of Total</th>
        </tr>
      </thead>
      <tbody>
        {report.buckets.map((b) => (
          <tr key={b.bucket}>
            <td>
              <span className={`badge badge--success`}>{b.bucket} days</span>
            </td>
            <td className="right">{formatIDR(b.amount_cents)}</td>
            <td className="right">{b.count}</td>
            <td className="right">{percent(b.amount_cents)}</td>
          </tr>
        ))}
      </tbody>
    </table>
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
