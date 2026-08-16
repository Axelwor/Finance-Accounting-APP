import { ChartIcon } from "./ChartIcon";

type Tone = "pos" | "neg" | "acc" | "warn" | "neutral";
type DeltaTone = "pos" | "neg" | "neutral";

/** Flat gradient bar when there are not enough points for a real line. */
function Spark({ values, tone }: { values?: number[]; tone: "pos" | "neg" | "acc" }) {
  if (!values || values.length < 2) {
    const cls =
      tone === "neg" ? "spark spark--neg spark--flat" : tone === "acc" ? "spark spark--acc spark--flat" : "spark spark--flat";
    return <div className={cls} aria-hidden="true" />;
  }
  const width = 220;
  const height = 24;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = width / (values.length - 1);
  const path = values
    .map((v, i) => {
      const x = i * stepX;
      const y = height - ((v - min) / range) * height;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  const cls = tone === "neg" ? "spark spark--neg" : tone === "acc" ? "spark spark--acc" : "spark";
  return (
    <div className={cls}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-hidden="true">
        <path d={path} />
      </svg>
    </div>
  );
}

export interface KpiCardProps {
  label: string;
  value: string;
  delta?: string;
  deltaTone?: DeltaTone;
  tone?: Tone;
  spark?: number[];
  sparkTone?: "pos" | "neg" | "acc";
  lead?: boolean;
  meta?: string;
  onDetails?: () => void;
}

/** Reusable KPI card — extracted from the old DashboardScreen StatusCell. */
export function KpiCard({
  label,
  value,
  delta,
  deltaTone = "neutral",
  tone = "neutral",
  spark,
  sparkTone,
  lead,
  meta,
  onDetails,
}: KpiCardProps) {
  const dotClass =
    tone === "pos" ? "" : tone === "neg" ? "dot--neg" : tone === "warn" ? "dot--warn" : tone === "acc" ? "dot--acc" : "";
  const valueCls =
    tone === "pos" ? "is-positive" : tone === "neg" ? "is-negative" : tone === "warn" ? "is-warning" : "";
  return (
    <div className={`status-cell${lead ? " status-cell--lead" : ""}`}>
      <div className="status-cell__label">
        <ChartIcon />
        <span className={`dot ${dotClass}`} aria-hidden="true" />
        <span>{label}</span>
      </div>
      <p className={`status-cell__value${valueCls ? " " + valueCls : ""}`}>{value}</p>
      {delta || meta ? (
        <div className="status-cell__delta">
          <span>{delta}</span>
          {meta ? <strong className={`is-${deltaTone}`}>{meta}</strong> : null}
        </div>
      ) : null}
      {spark && sparkTone ? <Spark values={spark} tone={sparkTone} /> : null}
      {onDetails ? (
        <button type="button" className="status-cell__details" onClick={onDetails}>
          View details
        </button>
      ) : null}
    </div>
  );
}
