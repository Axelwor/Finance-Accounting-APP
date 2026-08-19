import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import type { EntryTab } from "../../workbench/types";
import { api } from "../../api";
import { Combobox } from "../../components/Combobox";
import type { BankAccountListItem } from "../../types";
import { Button } from "../../components/m3";

interface ChequeFormData {
  cheque_number: string;
  type: "RECEIVED" | "ISSUED";
  direction: "INBOUND" | "OUTBOUND";
  bank_account_id: number | null;
  amount_cents: number;
  counterparty_name: string;
  date: string;
}

export function ChequeForm({ id, entryId, title }: EntryTab) {
  const workbench = useWorkbench();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [banks, setBanks] = useState<BankAccountListItem[]>([]);
  const [formData, setFormData] = useState<ChequeFormData>({
    cheque_number: "",
    type: "RECEIVED",
    direction: "INBOUND",
    bank_account_id: null,
    amount_cents: 0,
    counterparty_name: "",
    date: new Date().toISOString().slice(0, 10),
  });

  useEffect(() => {
    void loadBanks();
  }, []);

  const loadBanks = async () => {
    try {
      const data = await api.listBankAccounts();
      setBanks(data.map((a) => ({ id: a.id, account_name: a.name, code: a.code })));
    } catch (err) {
      console.error("Failed to load banks:", err);
    }
  };

  const handleSubmit = async () => {
    setSaving(true);
    setError(null);
    try {
      if (entryId) {
        await api.updateCheque(entryId.toString(), {
          ...formData,
          bank_account_id: formData.bank_account_id ?? undefined,
        });
      } else {
        await api.createCheque(formData);
      }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (window as any).workbench?.markUnsaved(id, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save cheque.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="form">
      <div className="form__head">
        <h2 className="form__title">{entryId ? "Edit Cheque" : "Register New Cheque"}</h2>
      </div>
      <div className="form__body">
        <div className="form__grid">
          <label className="form__field">
            <span className="form__label">Cheque Number *</span>
            <input
              className="form__input"
              type="text"
              value={formData.cheque_number}
              onChange={(e) => setFormData({ ...formData, cheque_number: e.target.value })}
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Type *</span>
            <select
              className="form__select"
              value={formData.type}
              onChange={(e) => {
                const type = e.target.value as "RECEIVED" | "ISSUED";
                setFormData({
                  ...formData,
                  type,
                  direction: type === "RECEIVED" ? "INBOUND" : "OUTBOUND",
                });
              }}
            >
              <option value="RECEIVED">Received</option>
              <option value="ISSUED">Issued</option>
            </select>
          </label>
          <label className="form__field">
            <span className="form__label">Direction *</span>
            <select
              className="form__select"
              value={formData.direction}
              onChange={(e) => setFormData({ ...formData, direction: e.target.value as "INBOUND" | "OUTBOUND" })}
            >
              <option value="INBOUND">Inbound</option>
              <option value="OUTBOUND">Outbound</option>
            </select>
          </label>
          <label className="form__field">
            <span className="form__label">Bank Account *</span>
            <select
              className="form__select"
              value={formData.bank_account_id ?? ""}
              onChange={(e) => setFormData({ ...formData, bank_account_id: e.target.value ? Number(e.target.value) : null })}
            >
              <option value="">Select bank account...</option>
              {banks.map((b) => (
                <option key={b.id} value={b.id}>{b.account_name} ({b.code})</option>
              ))}
            </select>
          </label>
          <label className="form__field">
            <span className="form__label">Amount (IDR) *</span>
            <input
              className="form__input"
              type="number"
              value={formData.amount_cents / 100 || ""}
              onChange={(e) => setFormData({ ...formData, amount_cents: Math.round(Number(e.target.value || "0") * 100) })}
              min="0"
              step="0.01"
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Counterparty Name *</span>
            <input
              className="form__input"
              type="text"
              value={formData.counterparty_name}
              onChange={(e) => setFormData({ ...formData, counterparty_name: e.target.value })}
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Date *</span>
            <input
              className="form__input"
              type="date"
              value={formData.date}
              onChange={(e) => setFormData({ ...formData, date: e.target.value })}
              required
            />
          </label>
        </div>
        {error && <p className="form__error">{error}</p>}
      </div>
      <div className="form__foot">
        <Button variant="text" onClick={() => workbench.close(id)}>Cancel</Button>
        <Button
          variant="filled"
          onClick={handleSubmit}
          disabled={saving || !formData.cheque_number || !formData.bank_account_id || !formData.amount_cents || !formData.counterparty_name}
        >
          {saving ? "Saving..." : "Save"}
        </Button>
      </div>
    </div>
  );
}
