import { useCallback, useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { AuditLog } from "../../types";

/**
 * Audit Log list (US-101 — Audit Trail).
 *
 * Shows every mutating action recorded in the audit trail (CREATE, POST,
 * VOID, CLOSE, UNLOCK, ...). Filters let the user narrow by entity type,
 * user, and date range. Each row shows the action, user, timestamp, and a
 * before/after diff so the full change can be reconstructed.
 */
export function AuditLogList() {
  const [items, setItems] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [entityType, setEntityType] = useState("");
  const [userId, setUserId] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listAuditLogs({
        entity_type: entityType || undefined,
        user_id: userId ? Number(userId) : undefined,
        from_date: fromDate || undefined,
        to_date: toDate || undefined,
      });
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load audit logs.");
    } finally {
      setLoading(false);
    }
  }, [entityType, userId, fromDate, toDate]);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const hasFilters = Boolean(entityType || userId || fromDate || toDate);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Audit Trail</span>
          <small>Log lengkap semua aksi yang mengubah data</small>
        </div>
        <div className="listtab__actions">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()} disabled={loading}>
            <ReloadIcon /> {loading ? "Loading..." : "Reload"}
          </button>
        </div>
      </div>

      <div className="listtab__filters">
        <input
          type="text"
          className="input"
          placeholder="Entity type (journal_entry, invoice, ...)"
          value={entityType}
          onChange={(e) => setEntityType(e.target.value)}
          onBlur={() => void load()}
        />
        <input
          type="text"
          className="input"
          placeholder="User ID"
          inputMode="numeric"
          value={userId}
          onChange={(e) => setUserId(e.target.value.replace(/[^\d]/g, ""))}
          onBlur={() => void load()}
        />
        <input
          type="date"
          className="input"
          value={fromDate}
          onChange={(e) => setFromDate(e.target.value)}
          onBlur={() => void load()}
        />
        <span style={{ alignSelf: "center" }}>to</span>
        <input
          type="date"
          className="input"
          value={toDate}
          onChange={(e) => setToDate(e.target.value)}
          onBlur={() => void load()}
        />
        {hasFilters ? (
          <button
            type="button"
            className="btn btn--ghost btn--sm"
            onClick={() => {
              setEntityType("");
              setUserId("");
              setFromDate("");
              setToDate("");
              setTimeout(() => void load(), 0);
            }}
          >
            Clear
          </button>
        ) : null}
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading audit trail..." />
        ) : error ? (
          <ErrorState title="Could not load audit trail" message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState title="No audit entries" message="No changes match the current filters." />
        ) : (
          <div className="audit-log-list">
            <div className="ledger-table">
              <div className="ledger-table__head">
                <span>Action</span>
                <span>Entity</span>
                <span>User</span>
                <span>Timestamp</span>
                <span />
              </div>
              {items.map((log) => (
                <AuditLogRow
                  key={log.id}
                  log={log}
                  expanded={expandedId === log.id}
                  onToggle={() => setExpandedId(expandedId === log.id ? null : log.id)}
                />
              ))}
            </div>
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Entr(ies)</span>
      </div>
    </div>
  );
}

function AuditLogRow({ log, expanded, onToggle }: { log: AuditLog; expanded: boolean; onToggle: () => void }) {
  return (
    <>
      <div className="ledger-table__row" onClick={onToggle} role="button">
        <span>
          <span className={`kind-mark ${actionClass(log.action)}`}>{log.action}</span>
        </span>
        <span className="ledger-table__party">
          {log.entity_type}
          {log.entity_id > 0 ? ` #${log.entity_id}` : ""}
        </span>
        <span className="ledger-table__cat">{log.user_name || (log.user_id ? `User #${log.user_id}` : "—")}</span>
        <span className="ledger-table__amount">{formatTimestamp(log.created_at)}</span>
        <span className="ledger-table__actions">
          <button type="button" className="btn btn--ghost btn--sm" onClick={(e) => { e.stopPropagation(); onToggle(); }}>
            {expanded ? "Hide" : "Details"}
          </button>
        </span>
      </div>
      {expanded ? (
        <div className="audit-log-list__diff">
          <div className="audit-log-list__diff-col">
            <h4>Before</h4>
            <pre>{log.before_data ? JSON.stringify(log.before_data, null, 2) : "—"}</pre>
          </div>
          <div className="audit-log-list__diff-col">
            <h4>After</h4>
            <pre>{log.after_data ? JSON.stringify(log.after_data, null, 2) : "—"}</pre>
          </div>
        </div>
      ) : null}
    </>
  );
}

function actionClass(action: string): string {
  switch (action) {
    case "CREATE":
    case "POST":
      return "is-positive";
    case "DELETE":
    case "VOID":
      return "is-negative";
    case "CLOSE":
    case "UNLOCK":
      return "";
    default:
      return "";
  }
}

function formatTimestamp(iso: string): string {
  if (!iso) return "—";
  return iso.replace("T", " ").slice(0, 19);
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path
        d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
    </svg>
  );
}
