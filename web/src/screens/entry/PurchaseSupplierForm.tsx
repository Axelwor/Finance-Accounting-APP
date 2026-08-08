import { useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";

export function PurchaseSupplierForm() {
  const { replaceDraft, markUnsaved } = useWorkbench();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!code || !name) { setError("Code and name are required."); return; }
    setSaving(true);
    try {
      const supplier = await api.createSupplier({ code, name, contact_person: contact, phone, email, address, city });
      replaceDraft("sup-" + supplier.id, supplier.code, "saved");
      markUnsaved("sup-" + supplier.id, false);
    } catch (err: any) {
      setError(err?.message || "Failed to save supplier.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-title">New Supplier</div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Code *</span>
              <input className="input" value={code} onChange={(e) => setCode(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">Name *</span>
              <input className="input" value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">Contact Person</span>
              <input className="input" value={contact} onChange={(e) => setContact(e.target.value)} />
            </label>
            <label className="field">
              <span className="field__label">Phone</span>
              <input className="input" value={phone} onChange={(e) => setPhone(e.target.value)} />
            </label>
            <label className="field">
              <span className="field__label">Email</span>
              <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
            </label>
            <label className="field">
              <span className="field__label">Address</span>
              <input className="input" value={address} onChange={(e) => setAddress(e.target.value)} />
            </label>
            <label className="field">
              <span className="field__label">City</span>
              <input className="input" value={city} onChange={(e) => setCity(e.target.value)} />
            </label>
          </div>
        </div>
        <aside className="action-rail">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </button>
        </aside>
        <FormError message={error} />
      </div>
    </form>
  );
}
