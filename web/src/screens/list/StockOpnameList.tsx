import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { StockOpnameListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const OPNAME_STATUS_TONE: Record<string, string> = {
  DRAFT: "is-muted",
  COUNTED: "is-muted",
  APPROVED: "is-positive",
  VOID: "is-negative",
};

export function StockOpnameList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<StockOpnameListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listStockOpnames();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  useTabRefresh(load);

  const openEntry = (item: StockOpnameListItem) =>
    workbench.openEntryExisting("stock-opname-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Stock Opnames</span>
          <small>Physical count adjustments. Approval posts Dr/Cr Inventory vs Adjustment Gain/Loss.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("stock-opname-entry")}
          >
            + New Opname
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
          <LoadingState label="Loading stock opnames..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No stock opnames yet"
            message="Record a physical count to reconcile system stock with the actual count. Approving posts an adjustment journal and records OPNAME_IN/OUT movements."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("stock-opname-entry")}>
                New Opname
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Date</span>
              <span>Notes</span>
              <span>Status</span>
              <span className="right">Adjustment</span>
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
                <span className="ledger-table__date">{it.opname_date}</span>
                <span className="ledger-table__memo">{it.notes ?? "—"}</span>
                <span><span className={`kind-mark ${OPNAME_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{formatIDR(it.total_adjustment_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Opname(s)</span>
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
