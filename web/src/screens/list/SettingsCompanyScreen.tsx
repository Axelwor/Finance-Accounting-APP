import { useEffect, useState } from "react";
import { api } from "../../api";
import type { TenantSettings } from "../../types";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { Button } from "../../components/m3";
import { showToast } from "../../lib/toast";

/** Company profile editor (SET-001): legal name, address, tax id, contact. */
export function SettingsCompanyScreen() {
  const [settings, setSettings] = useState<TenantSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    legal_name: "",
    address: "",
    city: "",
    phone: "",
    email: "",
    tax_id: "",
  });

  useEffect(() => {
    api
      .getSettings()
      .then((s) => {
        setSettings(s);
        setForm({
          legal_name: s.company.legal_name || "",
          address: s.company.address || "",
          city: s.company.city || "",
          phone: s.company.phone || "",
          email: s.company.email || "",
          tax_id: s.company.tax_id || "",
        });
      })
      .catch(() => setError("Failed to load settings"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading company info..." />;
  if (error) return <FormError message={error} />;
  if (!settings) return <EmptyState title="No settings" message="Settings could not be loaded." />;

  const field = (label: string, key: keyof typeof form, placeholder: string) => (
    <label className="form-field">
      <span className="form-field__label">{label}</span>
      <input
        className="input input--compact"
        type="text"
        value={form[key]}
        placeholder={placeholder}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
      />
    </label>
  );

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Company Info</span>
          <small>Business identity used across documents and reports.</small>
        </div>
      </div>
      <div className="listtab__body">
        <div className="form-card">
          <div className="form-card__title">Identity</div>
          <div className="form-grid form-grid-2col">
            <label className="form-field">
              <span className="form-field__label">Business Name</span>
              <input className="input input--compact" type="text" value={settings.company.name} disabled />
            </label>
            {field("Legal Name", "legal_name", "PT Contoh Sejahtera")}
            {field("Tax ID (NPWP)", "tax_id", "01.234.567.8-901.000")}
          </div>
        </div>
        <div className="form-card">
          <div className="form-card__title">Address & Contact</div>
          <div className="form-grid form-grid-2col">
            {field("Address", "address", "Jl. Sudirman No. 1")}
            {field("City", "city", "Jakarta")}
            {field("Phone", "phone", "+62 21 555 0102")}
            {field("Email", "email", "finance@contoh.co.id")}
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
              const company = await api.updateCompanyProfile(form);
              setSettings((s) => (s ? { ...s, company } : s));
              showToast("Company info saved");
            } catch (err) {
              showToast(err instanceof Error ? err.message : "Failed to save company info", "error");
            } finally {
              setSaving(false);
            }
          }}
        >
          {saving ? "Saving..." : "Save Changes"}
        </Button>
      </div>
    </div>
  );
}
