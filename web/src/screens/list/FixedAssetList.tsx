import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { FixedAssetListItem } from "../../types";

const ASSET_STATUS_TONE: Record<string, string> = {
  ACTIVE: "is-positive",
  DISPOSED: "is-negative",
  IMPAIRED: "is-muted",
};

export function FixedAssetList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<FixedAssetListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listFixedAssets();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);

  const openEntry = (item: FixedAssetListItem) =>
    workbench.openEntryExisting("fixed-assets-entry", item.id, `${item.code} · ${item.name}`, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Fixed Assets</span>
          <small>
            Aset tetap: registrasi, penyusutan multi-metode, revaluasi (ke ekuitas/OCI),
            disposisi & penjualan aset (PSAK 16).
          </small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("fixed-assets-entry")}
          >
            + New Asset
          </button>
          <button
            type="button"
            className="btn btn--icon btn--sm"
            onClick={() => void load()}
            aria-label="Reload"
          >
            <ReloadIcon />
          </button>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading fixed assets..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No fixed assets yet"
            message="Register a fixed asset to begin tracking depreciation, revaluation, and disposal (PSAK 16). Acquisition posts Dr Fixed Assets / Cr Cash."
            action={
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => workbench.openEntryDraft("fixed-assets-entry")}
              >
                New Asset
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <span>Code</span>
              <span>Name</span>
              <span>Acquired</span>
              <span>Method</span>
              <span className="right">Cost</span>
              <span className="right">Accum. Dep.</span>
              <span className="right">Book Value</span>
              <span>Status</span>
            </div>
            {items.map((it) => (
              <div
                key={it.id}
                className="ledger-table__row ledger-table__row--clickable"
                onClick={() => openEntry(it)}
                role="button"
                tabIndex={0}
              >
                <span className="ledger-table__ref">{it.code}</span>
                <span className="ledger-table__memo">{it.name}</span>
                <span className="ledger-table__date">{it.acquisition_date}</span>
                <span>{labelMethod(it.depreciation_method)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.acquisition_cost_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.accum_dep_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.book_value_cents)}</span>
                <span>
                  <span className={`kind-mark ${ASSET_STATUS_TONE[it.status] ?? "is-muted"}`}>
                    {it.status}
                  </span>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Asset(s)</span>
      </div>
    </div>
  );
}

function labelMethod(method: string): string {
  switch (method) {
    case "straight_line":
      return "Straight-line";
    case "declining_balance":
      return "Declining balance";
    case "units_of_production":
      return "Units of production";
    default:
      return method;
  }
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path
        d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10 3"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
    </svg>
  );
}
