import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR, formatDate } from "../../lib/format";
import type { BankStatement, BankReconciliation as Reconciliation } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

export function ReconciliationForm({ tabId, entryId, initialTitle }: Props) {
  void initialTitle;
  const workbench = useWorkbench();
  const statementId = entryId ? Number(entryId) : NaN;

  const [statement, setStatement] = useState<BankStatement | null>(null);
  const [recon, setRecon] = useState<Reconciliation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    if (!Number.isFinite(statementId)) {
      setError("No statement selected.");
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const stmt = await api.getBankStatement(statementId);
      setStatement(stmt);
      // If a reconciliation already exists for this statement, load it.
      try {
        const r = await api.startReconciliation(statementId);
        setRecon(r);
      } catch {
        setRecon(null);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load statement");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-match: match unmatched statement lines to unmatched book lines by
  // amount + date proximity (±3 days), mirroring the backend auto-match.
  function autoMatch() {
    setBusy(true);
    setError("");
    void (async () => {
      try {
        if (!statement) return;
        const result = await api.startReconciliation(statement.id);
        setRecon(result);
        const fresh = await api.getBankStatement(statement.id);
        setStatement(fresh);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Auto-match failed");
      } finally {
        setBusy(false);
      }
    })();
  }

  function matchLine(statementLineId: number, journalLineId: number) {
    setBusy(true);
    setError("");
    void (async () => {
      try {
        if (!recon) throw new Error("Start reconciliation first");
        await api.matchReconciliationLine(recon.id, {
          statement_line_id: statementLineId,
          journal_line_id: journalLineId,
        });
        const fresh = await api.getBankStatement(statement!.id);
        setStatement(fresh);
        const r = await api.getReconciliation(recon.id);
        setRecon(r);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Match failed");
      } finally {
        setBusy(false);
      }
    })();
  }

  function unmatchLine(statementLineId: number) {
    setBusy(true);
    setError("");
    void (async () => {
      try {
        if (!recon) throw new Error("Start reconciliation first");
        await api.unmatchReconciliationLine(recon.id, { statement_line_id: statementLineId });
        const fresh = await api.getBankStatement(statement!.id);
        setStatement(fresh);
        const r = await api.getReconciliation(recon.id);
        setRecon(r);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Unmatch failed");
      } finally {
        setBusy(false);
      }
    })();
  }

  function complete() {
    setBusy(true);
    setError("");
    void (async () => {
      try {
        if (!recon) throw new Error("Start reconciliation first");
        const result = await api.completeReconciliation(recon.id);
        setRecon(result);
        const fresh = await api.getBankStatement(statement!.id);
        setStatement(fresh);
        workbench.replaceDraft(tabId, `RECON #${result.statement_id}`, result.status);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Complete failed");
      } finally {
        setBusy(false);
      }
    })();
  }

  if (loading) {
    return (
      <div className="entrytab entrytab--accurate">
        <LoadingState label="Loading reconciliation..." />
      </div>
    );
  }

  if (!statement) {
    return (
      <div className="entrytab entrytab--accurate">
        <EmptyState title="Statement not found" message={error || "The selected statement could not be loaded."} />
      </div>
    );
  }

  const lines = statement.lines ?? [];
  const book = recon?.book_candidates ?? [];
  const matchedLines = lines.filter((l) => l.match_status === "MATCHED" || l.match_status === "MANUAL");
  const unmatchedLines = lines.filter((l) => l.match_status === "UNMATCHED");

  // Summary figures.
  const bookBalanceCents = recon?.book_balance_cents ?? 0;
  const statementBalanceCents = recon?.statement_balance_cents ?? statement.closing_balance_cents;
  const adjustedBookCents = recon?.adjusted_book_cents ?? bookBalanceCents;
  const adjustedStatementCents = recon?.adjusted_statement_cents ?? statementBalanceCents;
  const diffCents = recon?.diff_cents ?? adjustedStatementCents - adjustedBookCents;
  const isBalanced = diffCents === 0;
  const reconStatus = recon?.status ?? "DRAFT";

  return (
    <div className="entrytab entrytab--accurate">
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>Bank Reconciliation</span>
          <small>
            {statement.bank_account_name ?? `#${statement.bank_account_id}`}
            {statement.bank_account_code ? ` · ${statement.bank_account_code}` : ""} &middot; Statement{" "}
            {formatDate(statement.statement_date)} &middot;{" "}
            <span className={`kind-mark ${reconStatus === "RECONCILED" ? "is-positive" : "is-muted"}`}>
              {reconStatus}
            </span>
          </small>
        </div>
      </div>

      <div className="recon-grid">
        <section className="recon-panel">
          <header className="recon-panel__head">
            <span>Bank Statement Lines</span>
            <small>{lines.length} line(s) &middot; {unmatchedLines.length} unmatched</small>
          </header>
          <div className="recon-panel__body">
            {lines.length === 0 ? (
              <p className="recon-empty">No statement lines.</p>
            ) : (
              lines.map((line) => {
                const linked = book.find((b) => b.journal_line_id === line.matched_journal_line_id);
                return (
                  <div key={line.id} className={`recon-row ${line.match_status === "MATCHED" || line.match_status === "MANUAL" ? "is-matched" : ""}`}>
                    <div className="recon-row__main">
                      <span className="recon-row__date">{line.tx_date}</span>
                      <span className="recon-row__desc">{line.description || "—"}</span>
                      <span className="recon-row__ref">{line.reference || ""}</span>
                    </div>
                    <span className={`recon-row__amount ${line.amount_cents >= 0 ? "is-pos" : "is-neg"}`}>
                      {formatIDR(line.amount_cents)}
                    </span>
                    <div className="recon-row__action">
                      {line.match_status === "MATCHED" || line.match_status === "MANUAL" ? (
                        <>
                          <span className="kind-mark is-positive">{line.match_status.toLowerCase()}</span>
                          {linked && (
                            <small className="recon-row__link">
                              {linked.entry_number} · {linked.entry_date}
                            </small>
                          )}
                          <button
                            type="button"
                            className="btn btn--ghost btn--sm"
                            disabled={busy || reconStatus !== "DRAFT"}
                            onClick={() => unmatchLine(line.id)}
                          >
                            Unmatch
                          </button>
                        </>
                      ) : (
                        <span className="kind-mark is-muted">unmatched</span>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </section>

        <section className="recon-panel">
          <header className="recon-panel__head">
            <span>Book Transactions (Cash/Bank)</span>
            <small>{book.length} candidate(s)</small>
          </header>
          <div className="recon-panel__body">
            {book.length === 0 ? (
              <p className="recon-empty">
                {recon ? "No book transactions found for this account." : "Start reconciliation to load book candidates."}
              </p>
            ) : (
              book.map((b) => {
                const taken = lines.some((l) => l.matched_journal_line_id === b.journal_line_id);
                return (
                  <div key={b.journal_line_id} className={`recon-row ${taken ? "is-taken" : ""}`}>
                    <div className="recon-row__main">
                      <span className="recon-row__date">{b.entry_date}</span>
                      <span className="recon-row__desc">{b.description || b.entry_number}</span>
                      <span className="recon-row__ref">{b.entry_number}</span>
                    </div>
                    <span className={`recon-row__amount ${b.amount_cents >= 0 ? "is-pos" : "is-neg"}`}>
                      {formatIDR(b.amount_cents)}
                    </span>
                    <div className="recon-row__action">
                      {taken ? (
                        <span className="kind-mark is-positive">matched</span>
                      ) : (
                        <select
                          className="recon-row__select"
                          defaultValue=""
                          disabled={busy || reconStatus !== "DRAFT"}
                          onChange={(e) => {
                            const lineId = Number(e.target.value);
                            if (lineId > 0) matchLine(lineId, b.journal_line_id);
                            e.target.value = "";
                          }}
                        >
                          <option value="">Match to…</option>
                          {unmatchedLines.map((l) => (
                            <option key={l.id} value={l.id}>
                              {l.tx_date} · {formatIDR(l.amount_cents)}
                            </option>
                          ))}
                        </select>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </section>
      </div>

      <div className="recon-summary">
        <div className="recon-summary__row">
          <span>Book balance</span>
          <span>{formatIDR(bookBalanceCents)}</span>
        </div>
        <div className="recon-summary__row">
          <span>Statement balance</span>
          <span>{formatIDR(statementBalanceCents)}</span>
        </div>
        <div className="recon-summary__row">
          <span>Adjusted book</span>
          <span>{formatIDR(adjustedBookCents)}</span>
        </div>
        <div className="recon-summary__row">
          <span>Adjusted statement</span>
          <span>{formatIDR(adjustedStatementCents)}</span>
        </div>
        <div className={`recon-summary__row recon-summary__row--diff ${isBalanced ? "is-balanced" : "is-unbalanced"}`}>
          <span>Difference</span>
          <span>{formatIDR(diffCents)}</span>
        </div>
      </div>

      <FormError message={error} />

      <aside className="action-rail" aria-label="Reconciliation actions">
        {!recon ? (
          <button
            type="button"
            className="action-rail__btn action-rail__btn--primary"
            disabled={busy}
            onClick={autoMatch}
          >
            <span>{busy ? "Working..." : "Start & Auto-match"}</span>
          </button>
        ) : recon.status === "DRAFT" ? (
          <button
            type="button"
            className="action-rail__btn action-rail__btn--primary"
            disabled={busy || !isBalanced}
            onClick={complete}
            title={isBalanced ? "Complete reconciliation" : "Difference must be zero to complete"}
          >
            <span>{busy ? "Working..." : "Complete"}</span>
          </button>
        ) : (
          <span className="kind-mark is-positive">Reconciled</span>
        )}
      </aside>
    </div>
  );
}
