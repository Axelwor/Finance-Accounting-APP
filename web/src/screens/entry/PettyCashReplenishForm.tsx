import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import type { PettyCashFund, AccountItem } from "../../types";
import { formatIDR, formatDate } from "../../lib/format";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

export function PettyCashReplenishForm({ tabId, entryId }: Props) {
  const workbench = useWorkbench();
  
  const [funds, setFunds] = useState<PettyCashFund[]>([]);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [selectedFundId, setSelectedFundId] = useState<number | undefined>(entryId ? Number(entryId) : undefined);
  const [cashAccountId, setCashAccountId] = useState<number | undefined>(undefined);
  const [fundDetails, setFundDetails] = useState<{
    fund: PettyCashFund;
    available_cents: number;
    vouchers_count: number;
  } | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<{ id: number; message: string } | null>(null);

  useEffect(() => {
    Promise.all([api.listPettyCashFunds(), api.listAccounts()]).then(([fundsData, accountsData]) => {
      setFunds(fundsData);
      setAccounts(accountsData);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (selectedFundId && funds.length > 0) {
      const fund = funds.find((f) => f.id === selectedFundId);
      if (!fund) return;
      
      api.listPettyCashVouchers(selectedFundId).then((vouchers) => {
        const spentCents = vouchers
          .filter((v) => v.status === "posted")
          .reduce((sum, v) => sum + v.amount_cents, 0);
        const availableCents = fund.imprest_amount_cents - spentCents;
        
        setFundDetails({
          fund,
          available_cents: availableCents,
          vouchers_count: vouchers.length,
        });
      }).catch(() => {
        setFundDetails({
          fund,
          available_cents: fund.imprest_amount_cents,
          vouchers_count: 0,
        });
      });
    } else {
      setFundDetails(null);
    }
  }, [selectedFundId]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSaved(null);

    if (!selectedFundId || !cashAccountId) {
      setError("Please select a fund and replenishment account.");
      return;
    }

    if (!fundDetails) {
      setError("No fund details available.");
      return;
    }

    if (fundDetails.available_cents <= 0) {
      setError("Fund already fully replenished or no balance needed.");
      return;
    }

    setSaving(true);
    try {
      const result = await api.replenishPettyCashFund(selectedFundId, cashAccountId);
      setSaved(result);
      workbench.replaceDraft(tabId, `${fundDetails.fund.id} - Replenished`, "CREATED");
      window.setTimeout(() => {
        workbench.close(tabId);
      }, 2000);
    } catch (err: any) {
      setError(err.message || "Failed to replenish fund.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab__body__inner form--card" onSubmit={handleSubmit}>
      <div className="form__header">
        <h3>Replenish Petty Cash Fund</h3>
      </div>

      <div className="form__sections">
        <div className="form__section">
          <label className="field">
            <span className="field__label">Fund *</span>
            <select
              className="input"
              value={selectedFundId || ""}
              onChange={(e) => setSelectedFundId(Number(e.target.value))}
              disabled={!!saved}
            >
              <option value="">Select fund...</option>
              {funds.map((fund) => (
                <option key={fund.id} value={fund.id}>
                  {fund.id} - {fund.name}
                </option>
              ))}
            </select>
          </label>

          {fundDetails && (
            <>
              <div className="form__notice form__notice--info">
                <div className="form__notice__content">
                  <strong>Fund Details:</strong><br />
                  Imprest Amount: {formatIDR(fundDetails.fund.imprest_amount_cents / 100)}<br />
                  Available Balance: {formatIDR(fundDetails.available_cents / 100)}<br />
                  Vouchers: {fundDetails.vouchers_count}
                </div>
              </div>

              <div className="form__notice form__notice--success">
                <div className="form__notice__content">
                  <strong>Replenishment Needed:</strong> {formatIDR(fundDetails.available_cents / 100)}
                </div>
              </div>
            </>
          )}

          <label className="field">
            <span className="field__label">Replenishment Source Account *</span>
            <select
              className="input"
              value={cashAccountId || ""}
              onChange={(e) => setCashAccountId(Number(e.target.value))}
              disabled={!!saved}
            >
              <option value="">Select cash/bank account...</option>
              {accounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.id} - {acc.name}
                </option>
              ))}
            </select>
            {!cashAccountId && <small>Select the account to transfer funds from.</small>}
          </label>
        </div>
      </div>

      {saved && (
        <aside className="action-rail action-rail--success" aria-label="Success actions">
          <button type="button" className="action-rail__btn" onClick={() => workbench.close(tabId)}>
            Close
          </button>
          <button type="button" className="action-rail__btn" onClick={() => window.print()}>
            Print
          </button>
        </aside>
      )}

      <aside className="action-rail" aria-label="Form actions">
        <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving || !!saved}>
          <span>{saving ? "Processing..." : "Replenish Fund"}</span>
        </button>
      </aside>

      <FormError message={error} />
    </form>
  );
}
