import { type ReactNode } from "react";

interface EmptyStateProps {
  /** Entity name, e.g. "invoice", "customer". Used to build copy when title/message omitted. */
  entity?: string;
  title?: string;
  message?: string;
  action?: ReactNode;
  /** When true, the list has data but filters removed everything. */
  filtered?: boolean;
  /** Optional custom illustration; defaults to a document/inbox icon. */
  illustration?: ReactNode;
}

function defaultIllustration() {
  return (
    <svg
      className="empty-state__art"
      viewBox="0 0 96 96"
      width="72"
      height="72"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="20" y="26" width="56" height="50" rx="4" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M32 26v-8h32v8" fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
      <path d="M32 48h32M32 60h22" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function EmptyState({
  entity,
  title,
  message,
  action,
  filtered = false,
  illustration,
}: EmptyStateProps) {
  const resolvedTitle =
    title ??
    (filtered
      ? "No matches"
      : entity
        ? `No ${entity} yet`
        : "Nothing here yet");
  const resolvedMessage =
    message ??
    (filtered
      ? "Try adjusting your filters or search to find what you're looking for."
      : entity
        ? `Add your first ${entity} to get started.`
        : "Get started by creating your first record.");
  return (
    <div className="empty-state" role="status">
      <div className="empty-state__art-wrap">{illustration ?? defaultIllustration()}</div>
      <h3 className="empty-state__title">{resolvedTitle}</h3>
      <p className="empty-state__message">{resolvedMessage}</p>
      {!filtered && action ? <div className="empty-state__action">{action}</div> : null}
    </div>
  );
}
