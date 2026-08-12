import type { ReactNode } from "react";

/**
 * NextStepsBar — workflow-chain action strip shown after a document is saved.
 *
 * Renders a success marker plus the contextual next-step buttons for the
 * saved document (e.g. Quotation → [Convert to Sales Order] [Print]
 * [Send to Customer] [Close]). Buttons are plain children so each form keeps
 * full control of its handlers.
 */
export function NextStepsBar({
  number,
  hint,
  children,
}: {
  /** Saved document number shown next to the success marker. */
  number?: string;
  /** Optional secondary text under the marker. */
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="next-steps" role="group" aria-label="Next steps">
      <div className="next-steps__saved">
        <span className="next-steps__check" aria-hidden="true">
          ✓
        </span>
        <span className="next-steps__saved-text">
          Saved{number ? <strong> {number}</strong> : ""}
          {hint ? <small> · {hint}</small> : ""}
        </span>
      </div>
      <div className="next-steps__actions">{children}</div>
    </div>
  );
}

export default NextStepsBar;
