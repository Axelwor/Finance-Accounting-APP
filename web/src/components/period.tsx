import { useState } from "react";
import { api } from "../api";
import { Button, FormError } from "./ui";
import type { PeriodResult } from "../types";

type PeriodAction = "close" | "reopen";

const CONFIRMATIONS: Record<PeriodAction, string> = {
  close:
    "Close the current book period? Once closed, no new entries can be ruled until the period is reopened.",
  reopen:
    "Reopen a closed period? The closing journal will be reversed automatically by the system.",
};

/** Small "Book period" card: closes / reopens the period from the dashboard. */
export function PeriodCard() {
  const [busy, setBusy] = useState<PeriodAction | null>(null);
  const [result, setResult] = useState<PeriodResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: PeriodAction) => {
    if (!window.confirm(CONFIRMATIONS[action])) return;
    setError(null);
    setBusy(action);
    try {
      const res = action === "close" ? await api.closePeriod() : await api.unlockPeriod();
      setResult(res);
    } catch (err) {
      setResult(null);
      setError(err instanceof Error ? err.message : "Failed to process the period. Try again.");
    } finally {
      setBusy(null);
    }
  };

  const processing = busy !== null;

  return (
    <div className="period-card section">
      <div className="period-card__head">
        <h3 className="period-card__title">Book period</h3>
        <span className="period-card__meta">Close / reopen</span>
      </div>
      <p className="period-card__desc">
        Close the books when a period ends, or reopen when a correction is needed. The closing journal moves the period's profit into retained earnings.
      </p>
      <div className="quick-actions">
        <Button variant="primary" disabled={processing} onClick={() => void run("close")}>
          {busy === "close" ? "Closing books..." : "Close books"}
        </Button>
        <Button variant="secondary" disabled={processing} onClick={() => void run("reopen")}>
          {busy === "reopen" ? "Reopening..." : "Reopen period"}
        </Button>
      </div>

      {result ? (
        <p className="period-card__status" role="status">
          Period #{result.period_id} {result.status === "CLOSED" ? "closed" : "reopened"} — journal{" "}
          <code>{result.number}</code> ruled.
        </p>
      ) : null}
      <FormError message={error} />
    </div>
  );
}
