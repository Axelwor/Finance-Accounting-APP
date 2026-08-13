import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FieldShell, AmountField, DateField, TextareaField, FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { fmtCurrencyIDR, fmtDateIDR } from "../../lib/format";
import type { CreateRecurringTransactionInput, RecurringIntentType, RecurringFrequency, AccountItem, RecurringTransactionListItem } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

export function RecurringTransactionForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [intentType, setIntentType] = useState<RecurringIntentType>("CASH_IN");
  const [frequency, setFrequency] = useState<RecurringFrequency>("monthly");
  const [nextDate, setNextDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [amountCents, setAmountCents] = useState("0");
  const [fromAccountId, setFromAccountId] = useState<number | undefined>();
  const [toAccountId, setToAccountId] = useState<number | undefined>();
  const [paymentDescription, setPaymentDescription] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(!!entryId);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);

  const intentTypes: RecurringIntentType[] = ["CASH_IN", "CASH_OUT", "TRANSFER", "MANUAL_JOURNAL"];
  const frequencies: RecurringFrequency[] = ["daily", "weekly", "monthly", "quarterly", "yearly"];

  useEffect(() => {
    void loadAccounts();
    if (!entryId) {
      setLoading(false);
      return;
    }
    void loadEntity(Number(entryId));
  }, [entryId]);

  const loadAccounts = async () => {
    try {
      const accs = await api.listAccounts();
      setAccounts(accs);
    } catch {
      // ignore
    }
  };

  const loadEntity = async (id: number) => {
    try {
      const list = await api.listRecurring();
      const entity = list.find((i: RecurringTransactionListItem) => i.id === id);
      if (entity) {
        setName(entity.name);
        setCode(entity.code);
        setDescription(entity.description || "");
        setIntentType(entity.intent_type as RecurringIntentType);
        setFrequency(entity.frequency);
        setNextDate(entity.next_date);
        setEndDate(entity.end_date || "");
        setAmountCents(String(entity.amount_cents));
        setFromAccountId(entity.from_account_id);
        setToAccountId(entity.to_account_id);
        setPaymentDescription(entity.payment_description || "");
        setIsActive(entity.is_active);
      }
    } catch {
      setError("Failed to load recurring transaction.");
    } finally {
      setLoading(false);
    }
  };

  const computeOccurrences = (): string[] => {
    const dates: string[] = [];
    let current = nextDate ? new Date(nextDate + "T00:00:00") : new Date();
    for (let i = 0; i < 6; i++) {
      const yyyy = current.getFullYear();
      const mm = String(current.getMonth() + 1).padStart(2, "0");
      const dd = String(current.getDate()).padStart(2, "0");
      dates.push(`${yyyy}-${mm}-${dd}`);
      switch (frequency) {
        case "daily": current.setDate(current.getDate() + 1); break;
        case "weekly": current.setDate(current.getDate() + 7); break;
        case "monthly": current.setMonth(current.getMonth() + 1); break;
        case "quarterly": current.setMonth(current.getMonth() + 3); break;
        case "yearly": current.setFullYear(current.getFullYear() + 1); break;
      }
    }
    return dates;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!code || !name || !nextDate) {
      setError("Code, name, and next date are required.");
      return;
    }
    const amount = parseInt(amountCents.replace(/[^0-9]/g, ""), 10) || 0;
    if (amount <= 0) {
      setError("Amount must be greater than zero.");
      return;
    }
    const input: CreateRecurringTransactionInput = {
      code,
      name,
      description: description || undefined,
      intent_type: intentType,
      frequency,
      next_date: nextDate,
      end_date: endDate || undefined,
      amount_cents: amount,
      from_account_id: fromAccountId,
      to_account_id: toAccountId,
      payment_description: paymentDescription || undefined,
    };
    setSaving(true);
    try {
      if (entryId) {
        alert("Update not yet supported. Delete and recreate.");
      } else {
        await api.createRecurring(input);
      }
      workbench.close(tabId);
    } catch {
      setError("Save failed.");
    } finally {
      setSaving(false);
    }
  };

  const occurrences = computeOccurrences();
  const accountOptions = accounts.map((a: AccountItem) => ({ value: a.id, label: `${a.name}` }));

  if (loading) return <LoadingState label="Loading form..." />;

  return (
    <form className="form-body form-body--narrow" onSubmit={handleSubmit}>
      <div className="form-body__section">
        <h3>Basic Information</h3>
        <FieldShell label="Code" htmlFor="code">
          <input id="code" className="input" value={code} onChange={(e) => setCode(e.target.value)} placeholder="REC-001" autoFocus />
        </FieldShell>
        <FieldShell label="Name" htmlFor="name">
          <input id="name" className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Office Rent" />
        </FieldShell>
        <FieldShell label="Description" htmlFor="desc">
          <textarea id="desc" className="input" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </FieldShell>
      </div>

      <div className="form-body__section">
        <h3>Posting Template</h3>
        <FieldShell label="Intent Type" htmlFor="intent">
          <select id="intent" className="input" value={intentType} onChange={(e) => setIntentType(e.target.value as RecurringIntentType)}>
            {intentTypes.map((t) => (<option key={t} value={t}>{t}</option>))}
          </select>
        </FieldShell>
        <FieldShell label="Frequency" htmlFor="freq">
          <select id="freq" className="input" value={frequency} onChange={(e) => setFrequency(e.target.value as RecurringFrequency)}>
            {frequencies.map((f) => (<option key={f} value={f}>{f.charAt(0).toUpperCase() + f.slice(1)}</option>))}
          </select>
        </FieldShell>
        <AmountField label="Amount (cents)" value={amountCents} onChange={setAmountCents} />
        <DateField label="Next Date" value={nextDate} onChange={setNextDate} />
        <DateField label="End Date (optional)" value={endDate} onChange={setEndDate} />
        {intentType === "TRANSFER" && (
          <>
            <FieldShell label="Source Account" htmlFor="from-acct">
              <select id="from-acct" className="input" value={fromAccountId ?? ""} onChange={(e) => setFromAccountId(parseInt(e.target.value) || undefined)}>
                <option value="">Select source...</option>
                {accountOptions.map((opt) => (<option key={opt.value} value={opt.value}>{opt.label}</option>))}
              </select>
            </FieldShell>
            <FieldShell label="Destination Account" htmlFor="to-acct">
              <select id="to-acct" className="input" value={toAccountId ?? ""} onChange={(e) => setToAccountId(parseInt(e.target.value) || undefined)}>
                <option value="">Select destination...</option>
                {accountOptions.map((opt) => (<option key={opt.value} value={opt.value}>{opt.label}</option>))}
              </select>
            </FieldShell>
          </>
        )}
        <FieldShell label="Payment Description" htmlFor="pay-desc">
          <input id="pay-desc" className="input" value={paymentDescription} onChange={(e) => setPaymentDescription(e.target.value)} placeholder="E.g., Monthly office rent payment" />
        </FieldShell>
      </div>

      <div className="form-body__section">
        <h3>Preview (Next 6 Occurrences)</h3>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(6, 1fr)", gap: "8px" }}>
          {occurrences.map((d, i) => (
            <div key={i} style={{ padding: "8px", border: "1px solid #ddd", borderRadius: "4px", textAlign: "center" }}>
              <div style={{ fontWeight: "bold" }}>{fmtDateIDR(d)}</div>
              <div style={{ fontSize: "12px", color: "#666" }}>{fmtCurrencyIDR(parseInt(amountCents.replace(/[^0-9]/g, ""), 10))}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="form-body__section">
        <label className="field field--checkbox">
          <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
          <span className="field__label">Active</span>
        </label>
      </div>

      <aside className="action-rail" aria-label="Form actions">
        <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
          {saving ? "Saving..." : "Save"}
        </button>
        <button type="button" className="action-rail__btn" onClick={() => workbench.close(tabId)}>
          Cancel
        </button>
      </aside>
      <FormError message={error} />
    </form>
  );
}
