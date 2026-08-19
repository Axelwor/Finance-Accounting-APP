import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { StockTransferListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const TRANSFER_STATUS_TONE: Record<string, string> = {
  COMPLETED: "is-positive",
  VOID: "is-negative",
};

export function StockTransferList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<StockTransferListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listStockTransfers();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const openEntry = (item: StockTransferListItem) =>
    workbench.openEntryExisting("stock-transfer-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Stock Transfers</span>
          <small>Move stock between locations. Records TRANSFER_OUT / TRANSFER_IN movements (no journal posted).</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("stock-transfer-entry")}
          >
            + New Transfer
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
          <LoadingState label="Loading stock transfers..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No stock transfers yet"
            message="Move stock between warehouses. Each transfer records TRANSFER_OUT and TRANSFER_IN inventory movements with no journal posted (same inventory account)."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("stock-transfer-entry")}>
                New Transfer
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
                <span className="ledger-table__date">{it.transfer_date}</span>
                <span className="ledger-table__memo">{it.notes ?? "—"}</span>
                <span><span className={`kind-mark ${TRANSFER_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Transfer(s)</span>
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
