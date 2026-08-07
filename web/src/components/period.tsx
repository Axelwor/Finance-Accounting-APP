import { useState } from "react";
import { api } from "../api";
import { Button, FormError } from "./ui";
import type { PeriodResult } from "../types";

type PeriodAction = "close" | "reopen";

const CONFIRMATIONS: Record<PeriodAction, string> = {
  close:
    "Close the current book period? Once closed, no new entries can be ruled until reopened.",
  reopen:
    "Reopen a closed period? The closing journal will be reversed automatically.",
};

/** Period control card on the dashboard. */
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
    <section className="section">
      <div className="section-head">
        <h2 className="section-head__title">
          <span className="dot dot--acc" aria-hidden="true" />
          Book period
        </h2>
        <span className="section-head__meta">CLOSE / REOPEN</span>
      </div>
      <div className="period-stamp">
        <p className="period-stamp__desc">
          Close the books when a period ends, or reopen when a correction is needed. The closing journal moves the period's profit into retained earnings.
        </p>
        <div className="quick-actions">
          <Button variant="primary" disabled={processing} onClick={() => void run("close")}>
            {busy === "close" ? "Closing..." : "Close books"}
          </Button>
          <Button variant="secondary" disabled={processing} onClick={() => void run("reopen")}>
            {busy === "reopen" ? "Reopening..." : "Reopen period"}
          </Button>
        </div>

        {result ? (
          <p className="period-stamp__status" role="status">
            Period #{result.period_id} {result.status === "CLOSED" ? "closed" : "reopened"} - journal <code>{result.number}</code> posted
          </p>
        ) : null}
        <FormError message={error} />
      </div>
    </section>
  );
}
