import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR, formatDate } from "../../lib/format";
import type { BankStatementListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const STATUS_TONE: Record<string, string> = {
  IMPORTED: "is-muted",
  RECONCILING: "is-info",
  RECONCILED: "is-positive",
  VOID: "is-negative",
};

export function BankStatementList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<BankStatementListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listBankStatements();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Bank Statements</span>
          <small>Imported bank statements ready for reconciliation (US-050).</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("bank-reconciliation-entry")}
          >
            + Import Statement
          </Button>
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
          >
            <ReloadIcon />
          </IconButton>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading bank statements..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No bank statements imported yet"
            message="Import a bank statement (CSV pasted and parsed) to start reconciling recorded cash transactions against the bank."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("bank-reconciliation-entry")}>
                Import Statement
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <span>Date</span>
              <span>Bank Account</span>
              <span>Status</span>
              <span>Lines</span>
              <span className="right">Opening</span>
              <span className="right">Closing</span>
            </div>
            {items.map((it) => (
              <button
                key={it.id}
                type="button"
                className="ledger-table__row ledger-table__row--btn"
                onClick={() => workbench.openEntryExisting("bank-reconciliation-entry", it.id, `Stmt ${formatDate(it.statement_date)}`, it.status)}
              >
                <span>{formatDate(it.statement_date)}</span>
                <span className="ledger-table__cat">
                  {it.bank_account_name ?? `#${it.bank_account_id}`}
                  {it.bank_account_code ? <small> · {it.bank_account_code}</small> : null}
                </span>
                <span><span className={`kind-mark ${STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span>{it.line_count}</span>
                <span className="ledger-table__amount right">{formatIDR(it.opening_balance_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.closing_balance_cents)}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} statement(s)</span>
      </div>
    </div>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
