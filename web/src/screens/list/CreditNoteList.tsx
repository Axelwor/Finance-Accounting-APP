import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { CreditNoteListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const CN_STATUS_TONE: Record<string, string> = {
  DRAFT: "is-muted",
  APPLIED: "is-positive",
  VOID: "is-negative",
};

export function CreditNoteList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<CreditNoteListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listCreditNotes();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const totalReturns = useMemo(
    () => items.filter((i) => i.status !== "VOID").reduce((acc, it) => acc + it.total_cents, 0),
    [items],
  );
  const openEntry = (item: CreditNoteListItem) =>
    workbench.openEntryExisting("credit-note-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Credit Notes</span>
          <small>Sales returns and credit memos (CN). Posts return + COGS reversal.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("credit-note-entry")}
          >
            + New Credit Note
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
          <LoadingState label="Loading credit notes..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No credit notes yet"
            message="Issue a credit note when a customer returns goods. Each CN posts a return journal (Dr 4201 / Cr AR) and reverses COGS (Dr Inventory / Cr COGS)."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("credit-note-entry")}>
                New Credit Note
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Customer</span>
              <span>Invoice</span>
              <span>Status</span>
              <span className="right">Total</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row"
                role="button"
                tabIndex={0}
                onClick={() => openEntry(it)}
                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); openEntry(it); } }}
                style={{ cursor: "pointer" }}
              >
                <span className="ledger-table__no">{it.number}</span>
                <span className="ledger-table__date">{it.cn_date}</span>
                <span className="ledger-table__cat">{it.customer_name ?? `#${it.customer_id}`}</span>
                <span className="ledger-table__memo">INV #{it.invoice_id}</span>
                <span><span className={`kind-mark ${CN_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>Total returns <strong>{formatIDR(totalReturns)}</strong></span>
        <span className="listtab__footer-count">{items.length} credit note(s)</span>
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
