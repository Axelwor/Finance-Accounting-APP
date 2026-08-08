import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { GoodsReceivedNoteListItem } from "../../types";

const GRN_STATUS_TONE: Record<string, string> = {
  RECEIVED: "is-positive",
  RETURNED: "is-negative",
  CANCELLED: "is-negative",
};

export function GRNList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<GoodsReceivedNoteListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listGRNs();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const openEntry = (item: GoodsReceivedNoteListItem) =>
    workbench.openEntryExisting("grn-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Goods Received Notes</span>
          <small>Supplier deliveries (GRN). Posts Dr Inventory / Cr Accrued Payables.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("grn-entry")}>
            + New GRN
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading goods received notes..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No goods received yet"
            message="Receive goods against a purchase order. Each GRN posts a journal (Dr Inventory / Cr Accrued Payables) and records stock movements."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("grn-entry")}>
                New GRN
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Supplier</span>
              <span>PO</span>
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
                <span className="ledger-table__date">{it.grn_date}</span>
                <span className="ledger-table__cat">{it.supplier_name ?? `#${it.supplier_id}`}</span>
                <span className="ledger-table__memo">PO #{it.purchase_order_id}</span>
                <span><span className={`kind-mark ${GRN_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} GRN(s)</span>
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
