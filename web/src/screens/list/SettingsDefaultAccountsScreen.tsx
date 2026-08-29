import { useEffect, useState } from "react";
import { api } from "../../api";
import type { TenantSettings, AccountItem } from "../../types";
import { LoadingState, FormError, EmptyState } from "../../components/ui";
import { Button } from "../../components/m3";
import { AccountPicker } from "../../components/AccountPicker";
import { showToast } from "../../lib/toast";

type MappingKey = keyof TenantSettings["default_accounts"];

const MAPPINGS: { key: MappingKey; label: string; hint: string }[] = [
  { key: "default_sales_account_id", label: "Default Sales Account", hint: "Revenue posting when no item-level account" },
  { key: "default_purchase_account_id", label: "Default Purchase Account", hint: "Purchase posting fallback" },
  { key: "default_cogs_account_id", label: "Default COGS Account", hint: "Cost of goods sold fallback" },
  { key: "default_ar_account_id", label: "Accounts Receivable", hint: "AR posting on sales invoices" },
  { key: "default_ap_account_id", label: "Accounts Payable", hint: "AP posting on supplier invoices" },
  { key: "default_cash_account_id", label: "Default Cash/Bank", hint: "Default settlement account" },
  { key: "default_capital_account_id", label: "Capital Account", hint: "Owner capital postings" },
  { key: "retained_earnings_account_id", label: "Retained Earnings", hint: "Period-close profit target" },
  { key: "opening_balance_equity_account_id", label: "Opening Balance Equity", hint: "Opening balance counter-account" },
  { key: "fx_gain_account_id", label: "FX Gain Account", hint: "Realized gain on settlement (fallback 4904)" },
  { key: "fx_loss_account_id", label: "FX Loss Account", hint: "Realized loss on settlement (fallback 5905)" },
];

/** Default account mapping editor (SET-001): replaces the hardcoded codes
 *  previously scattered across the posting engines. */
export function SettingsDefaultAccountsScreen() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [mapping, setMapping] = useState<Record<MappingKey, string | null>>({} as Record<MappingKey, string | null>);

  useEffect(() => {
    Promise.all([api.getSettings(), api.listAccounts()])
      .then(([s, accountList]) => {
        setSettings(s);
        setAccounts(accountList);
        const next = {} as Record<MappingKey, string | null>;
        for (const { key } of MAPPINGS) {
          const id = s.default_accounts[key];
          next[key] = id != null && id !== undefined ? String(id) : null;
        }
        setMapping(next);
      })
      .catch(() => setError("Failed to load settings"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading default accounts..." />;
  if (error) return <FormError message={error} />;
  if (!settings) return <EmptyState title="No settings" message="Settings could not be loaded." />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Default Accounts</span>
          <small>Posting engines resolve these first; seeded chart-of-accounts codes remain the fallback.</small>
        </div>
      </div>
      <div className="listtab__body">
        <div className="form-card">
          <div className="form-card__title">Account Mapping</div>
          <div className="form-grid form-grid-2col">
            {MAPPINGS.map(({ key, label, hint }) => (
              <div key={key} className="form-field">
                <span className="form-field__label">{label}</span>
                <AccountPicker
                  accounts={accounts}
                  value={mapping[key] ?? null}
                  onChange={(value) => setMapping((m) => ({ ...m, [key]: value || null }))}
                />
                <span className="form-hint">{hint}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="listtab__footer">
        <Button
          variant="filled"
          disabled={saving}
          onClick={async () => {
            setSaving(true);
            try {
              const payload = {} as TenantSettings["default_accounts"];
              for (const { key } of MAPPINGS) {
                const value = mapping[key];
                payload[key] = value ? Number(value) : null;
              }
              const updated = await api.updateDefaultAccounts(payload);
              setSettings((s) => (s ? { ...s, default_accounts: updated } : s));
              showToast("Default accounts saved");
            } catch (err) {
              showToast(err instanceof Error ? err.message : "Failed to save default accounts", "error");
            } finally {
              setSaving(false);
            }
          }}
        >
          {saving ? "Saving..." : "Save Mapping"}
        </Button>
      </div>
    </div>
  );
}
