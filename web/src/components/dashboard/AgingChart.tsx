import { formatIDR } from "../../lib/format";

export interface AgingBuckets {
  total_cents: number;
  current_cents?: number;
  bucket_1_30_cents?: number;
  bucket_31_60_cents?: number;
  bucket_61_90_cents?: number;
  bucket_90_plus_cents?: number;
}

const BUCKET_LABELS: Array<{ key: keyof Omit<AgingBuckets, "total_cents">; label: string; tone: string }> = [
  { key: "current_cents", label: "Current", tone: "pos" },
  { key: "bucket_1_30_cents", label: "1-30", tone: "acc" },
  { key: "bucket_31_60_cents", label: "31-60", tone: "warn" },
  { key: "bucket_61_90_cents", label: "61-90", tone: "warn" },
  { key: "bucket_90_plus_cents", label: "90+", tone: "neg" },
];

/** Inline SVG horizontal bar chart for AR/AP aging buckets. No deps. */
export function AgingChart({ data, title }: { data: AgingBuckets | null; title: string }) {
  const total = data?.total_cents ?? 0;
  const buckets = BUCKET_LABELS.map((b) => ({
    label: b.label,
    tone: b.tone,
    value: data?.[b.key] ?? 0,
  }));
  const maxVal = Math.max(...buckets.map((b) => b.value), 1);

  return (
    <div className="aging-chart">
      <div className="aging-chart__head">
        <span className="aging-chart__title">{title}</span>
        <span className={`aging-chart__total${total > 0 ? " is-warn" : ""}`}>{formatIDR(total)}</span>
      </div>
      {total === 0 ? (
        <p className="aging-chart__empty">No outstanding receivables.</p>
      ) : (
        <div className="aging-chart__bars">
          {buckets.map((b) => {
            const pct = maxVal > 0 ? (b.value / maxVal) * 100 : 0;
            return (
              <div key={b.label} className="aging-chart__row">
                <span className="aging-chart__label">{b.label}</span>
                <div className="aging-chart__track">
                  <div
                    className={`aging-chart__bar aging-chart__bar--${b.tone}`}
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <span className="aging-chart__value">{b.value > 0 ? formatIDR(b.value) : "—"}</span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
