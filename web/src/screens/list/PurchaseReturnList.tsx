import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { PurchaseReturnListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

const PR_STATUS_TONE: Record<string, string> = {
  APPLIED: "is-positive",
  VOID: "is-negative",
};

const REFUND_METHOD_LABEL: Record<string, string> = {
  deduct: "Deduct AP",
  refund: "Cash Refund",
  credit_balance: "Credit Balance",
};

export function PurchaseReturnList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<PurchaseReturnListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await api.listPurchaseReturns();
    setItems(data);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  useTabRefresh(load);

  const totalReturns = useMemo(
    () => items.filter((i) => i.status !== "VOID").reduce((acc, it) => acc + it.total_cents, 0),
    [items],
  );
  const openEntry = (item: PurchaseReturnListItem) =>
    workbench.openEntryExisting("purchase-return-entry", item.id, item.number, item.status);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Purchase Returns</span>
          <small>Retur Pembelian. Posts Dr Accounts Payable / Cr Inventory + Cr Input VAT.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("purchase-return-entry")}
          >
            + New Purchase Return
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
          <LoadingState label="Loading purchase returns..." />
        ) : items.length === 0 ? (
          <EmptyState
            title="No purchase returns yet"
            message="Return goods to a supplier against a supplier invoice. Each return posts a journal (Dr AP / Cr Inventory + Cr Input VAT) and records stock movements."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("purchase-return-entry")}>
                New Purchase Return
              </Button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Supplier</span>
              <span>Date</span>
              <span>Refund Method</span>
              <span className="right">Total</span>
              <span className="right">VAT Reversed</span>
              <span className="right">AP Deducted</span>
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
                <span className="ledger-table__cat">{it.supplier_name ?? `#${it.supplier_id}`}</span>
                <span className="ledger-table__date">{it.return_date}</span>
                <span className="ledger-table__memo">{REFUND_METHOD_LABEL[it.refund_method] ?? it.refund_method}</span>
                <span className="ledger-table__amount right">{formatIDR(it.total_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.vat_reversed_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(it.ap_deducted_cents)}</span>
                <span><span className={`kind-mark ${PR_STATUS_TONE[it.status] ?? "is-muted"}`}>{it.status}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>Total returns <strong>{formatIDR(totalReturns)}</strong></span>
        <span className="listtab__footer-count">{items.length} purchase return(s)</span>
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
