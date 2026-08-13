import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { api } from "../../api";
import type { PettyCashVoucher, PettyCashFund } from "../../types";
import { formatIDR, formatDate } from "../../lib/format";

interface Props {
  selectedFundId?: number;
  onFundChange?: (fundId: number | undefined) => void;
}

export function PettyCashVoucherList({ selectedFundId, onFundChange }: Props) {
  const workbench = useWorkbench();
  const [funds, setFunds] = useState<PettyCashFund[]>([]);
  const [vouchers, setVouchers] = useState<PettyCashVoucher[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filterFundId, setFilterFundId] = useState<number | undefined>(selectedFundId);
  const [filterDateFrom, setFilterDateFrom] = useState<string>("");
  const [filterDateTo, setFilterDateTo] = useState<string>("");
  const [filterStatus, setFilterStatus] = useState<string>("all");

  useEffect(() => {
    api.listPettyCashFunds().then(setFunds).catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    const buildQuery = () => {
      const params = new URLSearchParams();
      if (filterFundId) params.append("fund_id", String(filterFundId));
      if (filterDateFrom) params.append("date_from", filterDateFrom);
      if (filterDateTo) params.append("date_to", filterDateTo);
      return params.toString();
    };
    const query = buildQuery();
    api.listPettyCashVouchers(filterFundId ?? undefined).then((data) => {
        let filtered = data;
        if (filterStatus !== "all") {
          filtered = filtered.filter((v) => v.status === filterStatus);
        }
        if (filterDateFrom || filterDateTo) {
          filtered = filtered.filter((v) => {
            const d = new Date(v.voucher_date);
            if (filterDateFrom && new Date(filterDateFrom) > d) return false;
            if (filterDateTo && new Date(filterDateTo) < d) return false;
            return true;
          });
        }
        setVouchers(filtered);
      })
      .catch(() => setError("Failed to load vouchers"))
      .finally(() => setLoading(false));
  }, [filterFundId, filterDateFrom, filterDateTo, filterStatus]);

  function handleFundChange(fundId: string) {
    const id = fundId ? Number(fundId) : undefined;
    setFilterFundId(id);
    onFundChange?.(id);
  }

  function handleNewVoucher() {
    workbench.openEntryDraft("pc-voucher-entry");
  }

  if (loading) return <LoadingState label="Loading vouchers..." />;
  if (error) return <FormError message={error} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Petty Cash Vouchers</span>
          <small>Expense vouchers against imprest funds.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <div className="form-inline">
            <label className="field field--inline">
              <span>Fund</span>
              <select
                className="input"
                value={filterFundId || ""}
                onChange={(e) => handleFundChange(e.target.value)}
              >
                <option value="">All Funds</option>
                {funds.map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.code} - {f.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="field field--inline">
              <span>Date From</span>
              <input
                type="date"
                className="input"
                value={filterDateFrom}
                onChange={(e) => setFilterDateFrom(e.target.value)}
              />
            </label>
            <label className="field field--inline">
              <span>Date To</span>
              <input
                type="date"
                className="input"
                value={filterDateTo}
                onChange={(e) => setFilterDateTo(e.target.value)}
              />
            </label>
            <label className="field field--inline">
              <span>Status</span>
              <select
                className="input"
                value={filterStatus}
                onChange={(e) => setFilterStatus(e.target.value)}
              >
                <option value="all">All</option>
                <option value="draft">Draft</option>
                <option value="posted">Posted</option>
              </select>
            </label>
          </div>
        </div>
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={handleNewVoucher}>
            + New Voucher
          </button>
          <span className="listtab__count">{vouchers.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {vouchers.length === 0 ? (
          <EmptyState
            title="No vouchers found"
            message="Add vouchers to track petty cash expenses."
            action={
              <button type="button" className="btn btn--primary" onClick={handleNewVoucher}>
                New Voucher
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Number</span>
              <span>Fund</span>
              <span>Date</span>
              <span>Amount</span>
              <span>Description</span>
              <span>Recipient</span>
              <span>Status</span>
            </div>
            {vouchers.map((voucher) => {
              const fund = funds.find((f) => f.id === voucher.fund_id);
              return (
                <div key={voucher.id} className="ledger-table__row">
                  <span className="ledger-table__no">{voucher.number}</span>
                  <span className="ledger-table__cat">{fund?.code || "?"}</span>
                  <span className="ledger-table__memo">{formatDate(voucher.voucher_date)}</span>
                  <span className="ledger-table__amount">{formatIDR(voucher.amount_cents / 100)}</span>
                  <span className="ledger-table__memo">{voucher.description}</span>
                  <span className="ledger-table__memo">{voucher.recipient || "—"}</span>
                  <span>
                    <span
                      className={`kind-mark ${
                        voucher.status === "posted" ? "is-positive" : "is-attention"
                      }`}
                    >
                      {voucher.status}
                    </span>
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{vouchers.length} voucher(s)</span>
      </div>
    </div>
  );
}
