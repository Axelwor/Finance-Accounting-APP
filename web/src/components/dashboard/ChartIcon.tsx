/** Small inline chart icon — keeps the dashboard free of icon dependencies. */
export function ChartIcon() {
  return (
    <svg
      className="status-cell__icon"
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2 12 L6 7 L9 10 L14 4" />
      <path d="M2 14 L14 14" opacity="0.4" />
    </svg>
  );
}
