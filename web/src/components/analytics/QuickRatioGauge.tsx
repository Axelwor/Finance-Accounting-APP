export interface QuickRatioGaugeProps {
  value: number;
}

export function QuickRatioGauge({ value }: QuickRatioGaugeProps) {
  const safeVal = Number.isFinite(value) ? Math.max(0, value) : 0;
  // Maximum gauge boundary = 3.0
  const markerPosPct = Math.min(100, Math.max(0, (safeVal / 3.0) * 100));

  const isSafe = safeVal >= 1.2;
  const isWarning = safeVal >= 1.0 && safeVal < 1.2;
  const isDanger = safeVal < 1.0;

  const statusText = isSafe ? "Sehat (>1.2x)" : isWarning ? "Waspada (1.0–1.2x)" : "Kritis (<1.0x)";
  const statusColor = isSafe ? "var(--color-success)" : isWarning ? "var(--color-warning)" : "var(--color-danger)";

  return (
    <div className="quick-ratio-gauge" style={{ width: "100%", display: "flex", flexDirection: "column", gap: "6px" }}>
      <div
        style={{
          position: "relative",
          width: "100%",
          height: "10px",
          borderRadius: "var(--radius-full)",
          overflow: "visible",
          display: "flex",
          backgroundColor: "var(--bg-surface-secondary)",
          border: "1px solid var(--border-color)",
        }}
      >
        {/* Strip 1: Danger (<1.0 => 0 to 33.33%) */}
        <div
          style={{
            width: "33.33%",
            backgroundColor: "#fca5a5",
            borderTopLeftRadius: "var(--radius-full)",
            borderBottomLeftRadius: "var(--radius-full)",
            height: "100%",
          }}
          title="Zona Kritis (< 1.0x)"
        />
        {/* Strip 2: Warning (1.0 - 1.2 => 6.67% width) */}
        <div
          style={{
            width: "6.67%",
            backgroundColor: "#fde68a",
            height: "100%",
          }}
          title="Zona Waspada (1.0x - 1.2x)"
        />
        {/* Strip 3: Safe (>1.2 => 60% width) */}
        <div
          style={{
            width: "60%",
            backgroundColor: "#86efac",
            borderTopRightRadius: "var(--radius-full)",
            borderBottomRightRadius: "var(--radius-full)",
            height: "100%",
          }}
          title="Zona Sehat (> 1.2x)"
        />

        {/* Marker Needle */}
        <div
          style={{
            position: "absolute",
            left: `${markerPosPct}%`,
            top: "-3px",
            width: "4px",
            height: "16px",
            backgroundColor: "var(--text-primary)",
            borderRadius: "2px",
            transform: "translateX(-50%)",
            boxShadow: "0 0 2px rgba(0,0,0,0.5)",
            zIndex: 2,
          }}
          title={`Nilai Rasio: ${safeVal.toFixed(2)}x`}
        />
      </div>

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: "11px" }}>
        <span style={{ color: "var(--text-secondary)" }}>Status Likuiditas:</span>
        <span style={{ fontWeight: 700, color: statusColor }}>{statusText}</span>
      </div>
    </div>
  );
}
