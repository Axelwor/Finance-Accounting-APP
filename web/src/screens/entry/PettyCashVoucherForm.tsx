import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import type { PettyCashVoucher, PettyCashFund, AccountItem } from "../../types";
import { formatIDR, todayISO, parseAmountInput } from "../../lib/format";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface FundDetails {
  imprest_cents: number;
  total_spent_cents: number;
  available_cents: number;
}

export function PettyCashVoucherForm({ tabId, entryId }: Props) {
  const workbench = useWorkbench();
  const isExisting = entryId !== undefined;

  const [funds, setFunds] = useState<PettyCashFund[]>([]);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [selectedFundId, setSelectedFundId] = useState<number | undefined>(undefined);
  const [voucherDate, setVoucherDate] = useState(todayISO());
  const [expenseAccountId, setExpenseAccountId] = useState<number | undefined>(undefined);
  const [amountDisplay, setAmountDisplay] = useState("");
  const [amountCents, setAmountCents] = useState(0);
  const [description, setDescription] = useState("");
  const [recipient, setRecipient] = useState("");
  const [fundDetails, setFundDetails] = useState<FundDetails | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<{ id: number; number: string; journal_number: string } | null>(null);

  useEffect(() => {
    Promise.all([api.listPettyCashFunds(), api.listAccounts()]).then(([fundsData, accountsData]) => {
      setFunds(fundsData);
      setAccounts(accountsData);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (selectedFundId && funds.length > 0) {
      fetchFundDetails(selectedFundId);
    } else {
      setFundDetails(null);
    }
  }, [selectedFundId]);

  async function fetchFundDetails(fundId: number) {
    try {
      const fund = funds.find((f) => f.id === fundId);
      if (!fund) return;
      
      const vouchers = await api.listPettyCashVouchers(fundId);
      const spentCents = vouchers
        .filter((v) => v.status === "posted")
        .reduce((sum, v) => sum + v.amount_cents, 0);
      
      setFundDetails({
        imprest_cents: fund.imprest_amount_cents,
        total_spent_cents: spentCents,
        available_cents: fund.imprest_amount_cents - spentCents,
      });
    } catch {
      // Ignore errors in fetching details
    }
  }

  function handleChange(field: string, value: any) {
    if (field === "selectedFundId") setSelectedFundId(Number(value));
    else if (field === "expenseAccountId") setExpenseAccountId(Number(value));
    else if (field === "voucherDate") setVoucherDate(String(value));
    else if (field === "description") setDescription(String(value));
    else if (field === "recipient") setRecipient(String(value));
    if (!isExisting) workbench.markUnsaved(tabId, true);
  }

  function handleAmountChange(value: string) {
    setAmountDisplay(value);
    const cents = Math.round(parseAmountInput(value) * 100);
    setAmountCents(cents);
    if (!isExisting) workbench.markUnsaved(tabId, true);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSaved(null);

    if (!selectedFundId || !expenseAccountId || !amountDisplay) {
      setError("All fields are required.");
      return;
    }

    if (amountCents <= 0) {
      setError("Amount must be greater than zero.");
      return;
    }

    if (fundDetails && amountCents > fundDetails.available_cents) {
      setError(`Exceeds available balance. Available: ${formatIDR(fundDetails.available_cents / 100)}`);
      return;
    }

    setSaving(true);
    try {
      const result = await api.createPettyCashVoucher({
        fund_id: selectedFundId,
        voucher_date: voucherDate,
        amount_cents: amountCents,
        expense_account_id: expenseAccountId,
        description,
        recipient: recipient || undefined,
      });
      setSaved(result);
      window.setTimeout(() => {
        workbench.close(tabId);
      }, 1500);
    } catch (err: any) {
      setError(err.message || "Failed to save voucher.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab__body__inner form--card" onSubmit={handleSubmit}>
      <div className="form__header">
        <h3>New Petty Cash Voucher</h3>
      </div>

      <div className="form__sections">
        <div className="form__section">
          <label className="field">
            <span className="field__label">Fund *</span>
            <select
              className="input"
              value={selectedFundId || ""}
              onChange={(e) => handleChange("selectedFundId", e.target.value)}
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
            <div className="form__notice form__notice--info">
              <div className="form__notice__content">
                <strong>Imprest Balance:</strong> {formatIDR(fundDetails.available_cents / 100)} / {formatIDR(fundDetails.imprest_cents / 100)}
                {fundDetails.total_spent_cents > 0 && (
                  <span> · Used: {formatIDR(fundDetails.total_spent_cents / 100)}</span>
                )}
              </div>
            </div>
          )}

          {fundDetails && amountCents > fundDetails.available_cents && (
            <div className="form__notice form__notice--warning">
              <div className="form__notice__content">
                Warning: Amount exceeds available balance by {formatIDR((amountCents - fundDetails.available_cents) / 100)}
              </div>
            </div>
          )}

          <label className="field">
            <span className="field__label">Date *</span>
            <input
              type="date"
              className="input"
              value={voucherDate}
              onChange={(e) => handleChange("voucherDate", e.target.value)}
              disabled={!!saved}
            />
          </label>

          <label className="field">
            <span className="field__label">Expense Account *</span>
            <select
              className="input"
              value={expenseAccountId || ""}
              onChange={(e) => handleChange("expenseAccountId", e.target.value)}
              disabled={!!saved}
            >
              <option value="">Select account...</option>
              {accounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.id} - {acc.name}
                </option>
              ))}
            </select>
          </label>

          <label className="field">
            <span className="field__label">Amount (IDR) *</span>
            <input
              type="text"
              className="input"
              placeholder="0.00"
              value={amountDisplay}
              onChange={(e) => handleAmountChange(e.target.value)}
              disabled={!!saved}
            />
            {amountDisplay && <small>{formatIDR(amountCents / 100)}</small>}
          </label>

          <label className="field">
            <span className="field__label">Description *</span>
            <textarea
              className="input"
              rows={3}
              placeholder="Expense description..."
              value={description}
              onChange={(e) => handleChange("description", e.target.value)}
              disabled={!!saved}
            />
          </label>

          <label className="field">
            <span className="field__label">Recipient</span>
            <input
              className="input"
              type="text"
              placeholder="Employee name or party..."
              value={recipient}
              onChange={(e) => handleChange("recipient", e.target.value)}
              disabled={!!saved}
            />
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
          <span>{saving ? "Saving..." : "Save"}</span>
        </button>
      </aside>

      <FormError message={error} />
    </form>
  );
}
