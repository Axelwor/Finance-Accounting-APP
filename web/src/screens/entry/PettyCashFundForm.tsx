import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { draftNumber } from "../../workbench/modules";
import type { PettyCashFund, AccountItem } from "../../types";
import { formatIDR, todayISO } from "../../lib/format";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

export function PettyCashFundForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = entryId !== undefined;

  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [cashAccountId, setCashAccountId] = useState<number | undefined>(undefined);
  const [imprestAmountCents, setImprestAmountCents] = useState<string>("");
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<{ id: number; code: string; journal_number: string } | null>(null);

  useEffect(() => {
    api.listAccounts().then(setAccounts).catch(() => {});
    if (isExisting && entryId) {
      // For edit mode: fetch existing fund details
      const id = Number(entryId);
      if (!Number.isFinite(id)) return;
      // Future: implement getPettyCashFund to load existing data
      setSaved(null);
    }
  }, [entryId, isExisting]);

  function handleChange(field: "code" | "name" | "cashAccountId" | "imprestAmountCents", value: string | number | undefined) {
    if (field === "code") setCode(String(value));
    else if (field === "name") setName(String(value));
    else if (field === "cashAccountId") setCashAccountId(Number(value) || undefined);
    else if (field === "imprestAmountCents") setImprestAmountCents(String(value));
    if (!isExisting) workbench.markUnsaved(tabId, true);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSaved(null);

    if (!code || !name || cashAccountId === undefined || !imprestAmountCents) {
      setError("All fields are required.");
      return;
    }

    const imprestCents = Math.round(parseFloat(imprestAmountCents) * 100);
    if (imprestCents <= 0) {
      setError("Imprest amount must be greater than zero.");
      return;
    }

    setSaving(true);
    try {
      const result = await api.createPettyCashFund({
        code,
        name,
        cash_account_id: cashAccountId,
        imprest_amount_cents: imprestCents,
      });
      setSaved(result);
      workbench.replaceDraft(tabId, result.code, "CREATED");
      window.setTimeout(() => {
        workbench.close(tabId);
      }, 2000);
    } catch (err: any) {
      setError(err.message || "Failed to save fund.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab__body__inner form--card" onSubmit={handleSubmit}>
      <div className="form__header">
        <h3>{isExisting ? "Edit Petty Cash Fund" : "New Petty Cash Fund"}</h3>
        {!isExisting && <small>{draftNumber("pc-fund-entry")}</small>}
      </div>

      <div className="form__sections">
        <div className="form__section">
          <label className="field">
            <span className="field__label">Code *</span>
            <input
              className="input"
              type="text"
              placeholder="PC-001"
              value={code}
              onChange={(e) => handleChange("code", e.target.value)}
              disabled={!!saved}
            />
          </label>

          <label className="field">
            <span className="field__label">Name *</span>
            <input
              className="input"
              type="text"
              placeholder="Main Office Petty Cash"
              value={name}
              onChange={(e) => handleChange("name", e.target.value)}
              disabled={!!saved}
            />
          </label>

          <label className="field">
            <span className="field__label">Cash Account *</span>
            <select
              className="input"
              value={cashAccountId || ""}
              onChange={(e) => handleChange("cashAccountId", e.target.value)}
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
            <span className="field__label">Imprest Amount (IDR) *</span>
            <input
              className="input"
              type="number"
              step="0.01"
              min="0"
              placeholder="0.00"
              value={imprestAmountCents}
              onChange={(e) => handleChange("imprestAmountCents", e.target.value)}
              disabled={!!saved}
            />
            {imprestAmountCents && <small>{formatIDR(parseFloat(imprestAmountCents))}</small>}
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
          <span>{saving ? "Saving..." : isExisting ? "Update" : "Save"}</span>
        </button>
      </aside>

      <FormError message={error} />
    </form>
  );
}
