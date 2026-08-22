export interface TrendPillProps {
  deltaPct: number | null;
  label?: string;
}

export function TrendPill({ deltaPct, label }: TrendPillProps) {
  if (deltaPct === null || Number.isNaN(deltaPct) || !Number.isFinite(deltaPct)) {
    return null;
  }

  const isPositive = deltaPct > 0;
  const isZero = deltaPct === 0;

  const bg = isZero
    ? "var(--bg-surface-tertiary)"
    : isPositive
    ? "var(--color-success-bg)"
    : "var(--color-danger-bg)";
  const color = isZero
    ? "var(--text-secondary)"
    : isPositive
    ? "var(--color-success-text)"
    : "var(--color-danger-text)";
  const border = isZero
    ? "var(--border-color)"
    : isPositive
    ? "var(--color-success-border)"
    : "var(--color-danger-border)";

  const symbol = isPositive ? "▲ +" : isZero ? "• " : "▼ ";
  const formatted = `${symbol}${Math.abs(deltaPct).toFixed(1)}%`;

  return (
    <span
      className="trend-pill font-mono"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "4px",
        fontSize: "11px",
        fontWeight: 700,
        padding: "1px 6px",
        borderRadius: "var(--radius-full)",
        backgroundColor: bg,
        color,
        border: `1px solid ${border}`,
        lineHeight: 1.4,
      }}
      title={label ? `${label}: ${formatted}` : formatted}
    >
      <span>{formatted}</span>
      {label && <span style={{ fontWeight: 500, fontSize: "10px", opacity: 0.9 }}>{label}</span>}
    </span>
  );
}
