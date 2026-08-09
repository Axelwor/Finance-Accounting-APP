import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { ProductionJobListItem } from "../../types";

const JOB_STATUS_TONE: Record<string, string> = {
  OPEN: "is-muted",
  IN_PROGRESS: "is-info",
  COMPLETED: "is-positive",
  CANCELLED: "is-negative",
};

export function ProductionJobList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<ProductionJobListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listProductionJobs();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Production Jobs</span>
          <small>Job-order costing: accumulate material/labor/overhead into WIP, then complete to Finished Goods.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("production-job-entry")}>
            + New Job
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading production jobs..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No production jobs yet"
            message="Create a production job to track costs (material, labor, overhead) into Work in Progress, then complete it to move the accumulated cost into Finished Goods."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("production-job-entry")}>
                + New Job
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <span>Number</span>
              <span>Finished Good</span>
              <span className="right">Target Qty</span>
              <span className="right">Total Cost</span>
              <span>Status</span>
            </div>
            {items.map((it) => (
              <div className="ledger-table__row ledger-table__row--link" key={it.id} onClick={() => workbench.openEntryExisting("production-job-entry", it.id, it.number, it.status)}>
                <span className="ledger-table__memo">{it.number}</span>
                <span>{it.finished_good_name ?? `#${it.finished_good_item_id}`}</span>
                <span className="ledger-table__amount right">{it.target_qty}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cost_cents)}</span>
                <span><span className={`kind-mark ${JOB_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Job(s)</span>
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
