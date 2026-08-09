import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { LeaseContractListItem } from "../../types";

const LEASE_STATUS_TONE: Record<string, string> = {
  ACTIVE: "is-positive",
  TERMINATED: "is-negative",
  EXPIRED: "is-muted",
};

export function LeaseContractList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<LeaseContractListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listLeaseContracts();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const openEntry = (item: LeaseContractListItem) =>
    workbench.openEntryExisting("lease-contract-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Lease Contracts</span>
          <small>PSAK 73 — right-of-use asset & lease liability from lease agreements.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("lease-contract-entry")}>
            + New Lease
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading lease contracts..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No lease contracts yet"
            message="Register a lease to automatically recognise the right-of-use asset (1701) and lease liability (2301) at present value."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("lease-contract-entry")}>
                New Lease Contract
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--header">
              <span>Number</span>
              <span>Lessor</span>
              <span>Period</span>
              <span>Status</span>
              <span className="right">ROU Asset</span>
            </div>
            {items.map((it) => (
              <div key={it.id} className="ledger-table__row ledger-table__row--clickable" onClick={() => openEntry(it)}>
                <span className="ledger-table__num">{it.number}</span>
                <span className="ledger-table__memo">{it.lessor_name ?? it.lessee_name}</span>
                <span className="ledger-table__date">{it.start_date} → {it.end_date}</span>
                <span><span className={`kind-mark ${LEASE_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
                <span className="ledger-table__amount right">{formatIDR(it.initial_rou_cents)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Lease(s)</span>
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
