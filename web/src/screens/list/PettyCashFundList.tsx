import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { PettyCashFund } from "../../types";
import { formatIDR } from "../../lib/format";

export function PettyCashFundList() {
  const workbench = useWorkbench();
  const [funds, setFunds] = useState<PettyCashFund[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listPettyCashFunds().then(setFunds).catch(() => setError("Failed to load funds")).finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading petty cash funds..." />;
  if (error) return <FormError message={error} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Petty Cash Funds</span>
          <small>Imprest fund master data for small cash expenses.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("pc-fund-entry")}>
            + New Fund
          </button>
          <span className="listtab__count">{funds.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {funds.length === 0 ? (
          <EmptyState
            title="No petty cash funds yet"
            message="Add funds to track imprest balances and expense vouchers."
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("pc-fund-entry")}>
                New Fund
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Imprest Amount</span>
              <span>Status</span>
            </div>
            {funds.map((fund) => (
              <div key={fund.id} className="ledger-table__row">
                <span className="ledger-table__no">{fund.code}</span>
                <span className="ledger-table__cat">{fund.name}</span>
                <span className="ledger-table__memo">{formatIDR(fund.imprest_amount_cents / 100)}</span>
                <span><span className={`kind-mark ${fund.is_active ? "is-positive" : "is-negative"}`}>{fund.is_active ? "Active" : "Inactive"}</span></span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{funds.length} fund(s)</span>
      </div>
    </div>
  );
}
