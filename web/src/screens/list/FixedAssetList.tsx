import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { FixedAssetListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

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
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("fixed-assets-entry")}
          >
            + New Asset
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
          <LoadingState label="Loading fixed assets..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No fixed assets yet"
            message="Register a fixed asset to begin tracking depreciation, revaluation, and disposal (PSAK 16). Acquisition posts Dr Fixed Assets / Cr Cash."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("fixed-assets-entry")}>
                New Asset
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Fixed assets list">
            <thead>
              <tr>
                <th scope="col">Code</th>
                <th scope="col">Name</th>
                <th scope="col">Acquired</th>
                <th scope="col">Method</th>
                <th scope="col" className="right">Cost</th>
                <th scope="col" className="right">Accum. Dep.</th>
                <th scope="col" className="right">Book Value</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <FixedAssetRow key={it.id} item={it} onOpen={() => openEntry(it)} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function FixedAssetRow({ item, onOpen }: { item: FixedAssetListItem; onOpen: () => void }) {
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.code}</th>
      <td>{item.name}</td>
      <td>{item.acquisition_date}</td>
      <td>{labelMethod(item.depreciation_method)}</td>
      <td className="right">{formatIDR(item.acquisition_cost_cents)}</td>
      <td className="right">{formatIDR(item.accum_dep_cents)}</td>
      <td className="right">{formatIDR(item.book_value_cents)}</td>
      <td><span className={`kind-mark ${ASSET_STATUS_TONE[item.status] ?? "is-muted"}`}>{item.status}</span></td>
    </tr>
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
