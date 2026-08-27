export interface SegmentedAgingBuckets {
  b0_30: number;
  b31_60: number;
  b61_90: number;
  over90: number;
}

export interface SegmentedAgingBarProps {
  buckets: SegmentedAgingBuckets;
  formatCurrency?: (cents: number) => string;
}

export function SegmentedAgingBar({ buckets, formatCurrency = (c) => `Rp ${c.toLocaleString("id-ID")}` }: SegmentedAgingBarProps) {
  const { b0_30, b31_60, b61_90, over90 } = buckets;
  const safe0_30 = Math.max(0, b0_30);
  const safe31_60 = Math.max(0, b31_60);
  const safe61_90 = Math.max(0, b61_90);
  const safeOver90 = Math.max(0, over90);

  const total = safe0_30 + safe31_60 + safe61_90 + safeOver90;

  if (total <= 0) {
    return (
      <div
        className="aging-stacked-bar aging-stacked-bar--empty"
        style={{
          width: "100%",
          height: "8px",
          backgroundColor: "var(--bg-surface-tertiary)",
          borderRadius: "var(--radius-full)",
          overflow: "hidden",
        }}
        role="progressbar"
        aria-valuenow={0}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Tidak ada saldo outstanding"
        title="Tidak ada saldo outstanding (Rp 0)"
      />
    );
  }

  const p0_30 = (safe0_30 / total) * 100;
  const p31_60 = (safe31_60 / total) * 100;
  const p61_90 = (safe61_90 / total) * 100;
  const pOver90 = (safeOver90 / total) * 100;

  const segments = [
    { label: "0-30 Hari", cents: safe0_30, pct: p0_30, color: "var(--color-success)" },
    { label: "31-60 Hari", cents: safe31_60, pct: p31_60, color: "var(--color-info)" },
    { label: "61-90 Hari", cents: safe61_90, pct: p61_90, color: "var(--color-warning)" },
    { label: ">90 Hari", cents: safeOver90, pct: pOver90, color: "var(--color-danger)" },
  ];

  return (
    <div
      className="aging-stacked-bar"
      style={{
        display: "flex",
        width: "100%",
        height: "8px",
        borderRadius: "var(--radius-full)",
        overflow: "hidden",
        backgroundColor: "var(--bg-surface-secondary)",
      }}
      role="progressbar"
      aria-label="Distribusi Umur Saldo"
    >
      {segments.map((seg, idx) => {
        if (seg.pct <= 0) return null;
        return (
          <div
            key={idx}
            style={{
              width: `${seg.pct}%`,
              backgroundColor: seg.color,
              height: "100%",
            }}
            title={`${seg.label}: ${formatCurrency(seg.cents)} • ${seg.pct.toFixed(1)}%`}
          />
        );
      })}
    </div>
  );
}
