import { type ReactNode } from "react";

export type StatusKind = "success" | "warning" | "danger" | "neutral" | "info";

const STATUS_MAP: Record<string, StatusKind> = {
  POSTED: "success",
  PAID: "success",
  CLEARED: "success",
  APPROVED: "success",
  ACTIVE: "success",
  SETTLED: "success",
  PENDING: "info",
  DRAFT: "neutral",
  OPEN: "info",
  ISSUED: "info",
  PARTIALLY_PAID: "warning",
  PARTIAL: "warning",
  REVIEW: "warning",
  OVERDUE: "danger",
  BOUNCED: "danger",
  FAILED: "danger",
  REJECTED: "danger",
  VOID: "neutral",
  CANCELLED: "neutral",
  INACTIVE: "neutral",
  CLOSED: "neutral",
};

function resolveKind(status: string, override?: StatusKind): StatusKind {
  if (override) return override;
  const upper = status.toUpperCase();
  return STATUS_MAP[upper] ?? "neutral";
}

interface StatusBadgeProps {
  status: string;
  kind?: StatusKind;
  children?: ReactNode;
}

export function StatusBadge({ status, kind, children }: StatusBadgeProps) {
  const resolved = resolveKind(status, kind);
  const label = children ?? status;
  return (
    <span className={`status-badge status-badge--${resolved}`} role="status">
      <span className="status-badge__dot" aria-hidden="true" />
      {label}
    </span>
  );
}
