import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { GoodsReceivedNoteListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

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
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("grn-entry")}
          >
            + New GRN
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
          <LoadingState label="Loading goods received notes..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No goods received yet"
            message="Receive goods against a purchase order. Each GRN posts a journal (Dr Inventory / Cr Accrued Payables) and records stock movements."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("grn-entry")}>
                New GRN
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Goods received notes list">
            <thead>
              <tr>
                <th scope="col">Number</th>
                <th scope="col">Date</th>
                <th scope="col">Supplier</th>
                <th scope="col">PO</th>
                <th scope="col">Status</th>
                <th scope="col" className="right">Total</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <GRNRow key={it.id} item={it} onOpen={() => openEntry(it)} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function GRNRow({ item, onOpen }: { item: GoodsReceivedNoteListItem; onOpen: () => void }) {
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.number}</th>
      <td>{item.grn_date}</td>
      <td>{item.supplier_name ?? `#${item.supplier_id}`}</td>
      <td>PO #{item.purchase_order_id}</td>
      <td><span className={`kind-mark ${GRN_STATUS_TONE[item.status] ?? "is-muted"}`}>{item.status}</span></td>
      <td className="right">{formatIDR(item.total_cents)}</td>
    </tr>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
