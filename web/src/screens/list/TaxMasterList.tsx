import { useEffect, useState } from "react";
import { api } from "../../api";
import type { TaxMaster, AccountItem } from "../../types";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { Button } from "../../components/m3";
import { AccountPicker } from "../../components/AccountPicker";
import { showToast } from "../../lib/toast";

/** Tax master list (SET-001): name, rate, and COA mapping for sales/purchase
 *  tax posting. The active PPN row drives the posting engines; unmapped
 *  tenants fall back to the legacy 2202/1203 codes. */
export function TaxMasterList() {
  const [rows, setRows] = useState<TaxMaster[]>([]);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    code: "",
    name: "",
    rate: "",
    sales_account_id: null as string | null,
    purchase_account_id: null as string | null,
  });
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    Promise.all([api.listTaxes(), api.listAccounts()])
      .then(([taxRows, accountList]) => {
        setRows(taxRows);
        setAccounts(accountList);
      })
      .catch(() => setError("Failed to load taxes"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading taxes..." />;
  if (error) return <FormError message={error} />;

  const resetForm = () => {
    setForm({ code: "", name: "", rate: "", sales_account_id: null, purchase_account_id: null });
    setEditingId(null);
  };

  const submit = async () => {
    if (!form.code.trim() || !form.name.trim() || form.rate === "") return;
    setSaving(true);
    try {
      const payload = {
        code: form.code.trim().toUpperCase(),
        name: form.name.trim(),
        rate: Number(form.rate),
        sales_account_id: form.sales_account_id ? Number(form.sales_account_id) : null,
        purchase_account_id: form.purchase_account_id ? Number(form.purchase_account_id) : null,
      };
      if (editingId) {
        const updated = await api.updateTax(editingId, payload);
        setRows((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
        showToast("Tax updated");
      } else {
        const created = await api.createTax(payload);
        setRows((prev) => [...prev, created]);
        showToast("Tax created");
      }
      resetForm();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to save tax", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Tax Master</span>
          <small>Tax names, rates, and the COA used when posting sales/purchase tax.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Code</span>
            <input
              className="filter-pill__input"
              type="text"
              value={form.code}
              placeholder="PPN"
              style={{ width: 80 }}
              onChange={(e) => setForm((f) => ({ ...f, code: e.target.value.toUpperCase() }))}
            />
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">Name</span>
            <input
              className="filter-pill__input"
              type="text"
              value={form.name}
              placeholder="VAT"
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">Rate %</span>
            <input
              className="filter-pill__input"
              type="number"
              min={0}
              max={100}
              step="any"
              value={form.rate}
              placeholder="11"
              style={{ width: 80 }}
              onChange={(e) => setForm((f) => ({ ...f, rate: e.target.value }))}
            />
          </label>
        </div>
        <span className="listtab__count">{rows.length}</span>
      </div>
      <div className="form-card" style={{ marginTop: 12 }}>
        <div className="form-card__title">{editingId ? "Edit Tax" : "New Tax"}</div>
        <div className="form-grid form-grid-2col">
          <div className="form-field">
            <span className="form-field__label">Sales Tax Account (output)</span>
            <AccountPicker
              accounts={accounts}
              value={form.sales_account_id}
              onChange={(id) => setForm((f) => ({ ...f, sales_account_id: id || null }))}
            />
          </div>
          <div className="form-field">
            <span className="form-field__label">Purchase Tax Account (input)</span>
            <AccountPicker
              accounts={accounts}
              value={form.purchase_account_id}
              onChange={(id) => setForm((f) => ({ ...f, purchase_account_id: id || null }))}
            />
          </div>
        </div>
        <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
          <Button variant="filled" size="sm" disabled={saving || !form.code.trim() || !form.name.trim()} onClick={submit}>
            {editingId ? "Update Tax" : "Add Tax"}
          </Button>
          {editingId && (
            <Button variant="text" size="sm" onClick={resetForm}>
              Cancel
            </Button>
          )}
        </div>
      </div>
      <div className="listtab__body">
        {rows.length === 0 ? (
          <EmptyState title="No taxes yet" message="Add tax master rows (e.g. PPN 11%) used by documents." />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Rate %</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {rows.map((row) => (
              <div key={row.id} className="ledger-table__row">
                <span className="ledger-table__no">{row.code}</span>
                <span className="ledger-table__cat">{row.name}</span>
                <span className="ledger-table__amt">{row.rate}</span>
                <span>
                  <span className={`kind-mark ${row.is_active ? "is-positive" : "is-negative"}`}>
                    {row.is_active ? "Active" : "Inactive"}
                  </span>
                </span>
                <span className="ledger-table__action">
                  <Button
                    variant="text"
                    size="xs"
                    onClick={() => {
                      setEditingId(row.id);
                      setForm({
                        code: row.code,
                        name: row.name,
                        rate: String(row.rate),
                        sales_account_id: row.sales_account_id != null ? String(row.sales_account_id) : null,
                        purchase_account_id: row.purchase_account_id != null ? String(row.purchase_account_id) : null,
                      });
                    }}
                  >
                    Edit
                  </Button>
                  <Button
                    variant="outlined"
                    size="xs"
                    danger
                    disabled={!row.is_active}
                    onClick={() => {
                      if (!window.confirm(`Deactivate tax "${row.code}"?`)) return;
                      api
                        .deactivateTax(row.id)
                        .then(() => {
                          setRows((prev) => prev.map((r) => (r.id === row.id ? { ...r, is_active: false } : r)));
                          showToast("Tax deactivated");
                        })
                        .catch((err) => showToast(err instanceof Error ? err.message : "Failed to deactivate", "error"));
                    }}
                  >
                    Deactivate
                  </Button>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
