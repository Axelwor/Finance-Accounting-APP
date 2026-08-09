import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { BOMListItem } from "../../types";

const BOM_STATUS_TONE: Record<string, string> = {
  ACTIVE: "is-positive",
  VOID: "is-negative",
};

export function BOMList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<BOMListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listBOMs();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Bill of Materials</span>
          <small>Recipe for producing finished goods: inputs, output qty, and standard costs.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("bom-entry")}>
            + New BOM
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading BOMs..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No BOMs yet"
            message="Create a Bill of Materials to define the materials, labor, and overhead that go into producing a finished good. Production jobs use BOMs to prefill costs."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("bom-entry")}>
                + New BOM
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <span>Code</span>
              <span>Name</span>
              <span>Finished Good</span>
              <span>Output Qty</span>
              <span>Status</span>
            </div>
            {items.map((it) => (
              <div className="ledger-table__row ledger-table__row--link" key={it.id} onClick={() => workbench.openEntryExisting("bom-entry", it.id, it.code, it.status)}>
                <span className="ledger-table__memo">{it.code}</span>
                <span>{it.name}</span>
                <span className="ledger-table__memo">{it.finished_good_name ?? `#${it.finished_good_item_id}`}</span>
                <span className="ledger-table__amount right">{it.output_qty}</span>
                <span><span className={`kind-mark ${BOM_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} BOM(s)</span>
      </div>
    </div>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
