import { useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import type { CreateSupplierInput } from "../../types";

const SUPPLIER_TYPES = ["GOODS", "SERVICE", "MIXED"] as const;
const CURRENCIES = ["IDR", "USD", "EUR", "SGD", "JPY", "CNY", "AUD", "GBP"] as const;

export function PurchaseSupplierForm() {
  const { replaceDraft, markUnsaved } = useWorkbench();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  // ERP fields
  const [npwp, setNpwp] = useState("");
  const [supplierType, setSupplierType] = useState<string>("GOODS");
  const [isPkp, setIsPkp] = useState(false);
  const [currencyCode, setCurrencyCode] = useState<string>("IDR");
  const [bankName, setBankName] = useState("");
  const [bankAccountNumber, setBankAccountNumber] = useState("");
  const [bankAccountName, setBankAccountName] = useState("");
  const [website, setWebsite] = useState("");
  const [fax, setFax] = useState("");
  const [contact2, setContact2] = useState("");
  const [phone2, setPhone2] = useState("");
  const [openingBalance, setOpeningBalance] = useState("");
  const [openingBalanceDate, setOpeningBalanceDate] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!code || !name) { setError("Code and name are required."); return; }
    setSaving(true);
    try {
      const payload: CreateSupplierInput = {
        code,
        name,
        contact_person: contact || undefined,
        phone: phone || undefined,
        email: email || undefined,
        address: address || undefined,
        city: city || undefined,
        npwp: npwp || undefined,
        supplier_type: supplierType as CreateSupplierInput["supplier_type"],
        is_pkp: isPkp,
        currency_code: currencyCode,
        bank_name: bankName || undefined,
        bank_account_number: bankAccountNumber || undefined,
        bank_account_name: bankAccountName || undefined,
        website: website || undefined,
        fax: fax || undefined,
        contact_person_2: contact2 || undefined,
        phone_2: phone2 || undefined,
      };
      const ob = parseFloat(openingBalance.replace(/[^0-9.-]/g, ""));
      if (!isNaN(ob) && ob !== 0) {
        payload.opening_balance_cents = Math.round(ob * 100);
        payload.opening_balance_date = openingBalanceDate || undefined;
      }
      const supplier = await api.createSupplier(payload);
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
          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Data Utama</legend>
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
                <span className="field__label">Tipe Supplier</span>
                <select className="input" value={supplierType} onChange={(e) => setSupplierType(e.target.value)}>
                  {SUPPLIER_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Mata Uang</span>
                <select className="input" value={currencyCode} onChange={(e) => setCurrencyCode(e.target.value)}>
                  {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
                </select>
              </label>
            </div>
          </fieldset>

          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Info Pajak</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">NPWP</span>
                <input className="input" value={npwp} onChange={(e) => setNpwp(e.target.value)} placeholder="00.000.000.0-000.000" />
              </label>
              <label className="field field--checkbox">
                <input type="checkbox" checked={isPkp} onChange={(e) => setIsPkp(e.target.checked)} />
                <span className="field__label">PKP (Pengusaha Kena Pajak)</span>
              </label>
            </div>
          </fieldset>

          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Bank</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Nama Bank</span>
                <input className="input" value={bankName} onChange={(e) => setBankName(e.target.value)} placeholder="BCA / Mandiri / ..." />
              </label>
              <label className="field">
                <span className="field__label">No. Rekening</span>
                <input className="input" value={bankAccountNumber} onChange={(e) => setBankAccountNumber(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Nama Pemilik Rekening</span>
                <input className="input" value={bankAccountName} onChange={(e) => setBankAccountName(e.target.value)} />
              </label>
            </div>
          </fieldset>

          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Kontak</legend>
            <div className="entrytab__detail-grid">
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
                <span className="field__label">Contact Person 2</span>
                <input className="input" value={contact2} onChange={(e) => setContact2(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Phone 2</span>
                <input className="input" value={phone2} onChange={(e) => setPhone2(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Fax</span>
                <input className="input" value={fax} onChange={(e) => setFax(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Website</span>
                <input className="input" value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://..." />
              </label>
            </div>
          </fieldset>

          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Alamat</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Address</span>
                <input className="input" value={address} onChange={(e) => setAddress(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">City</span>
                <input className="input" value={city} onChange={(e) => setCity(e.target.value)} />
              </label>
            </div>
          </fieldset>

          <fieldset className="entrytab__fieldset">
            <legend className="entrytab__legend">Saldo Awal Hutang</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Saldo Awal (Rp)</span>
                <input className="input" value={openingBalance} onChange={(e) => setOpeningBalance(e.target.value)} placeholder="0" />
              </label>
              <label className="field">
                <span className="field__label">Tanggal Saldo Awal</span>
                <input className="input" type="date" value={openingBalanceDate} onChange={(e) => setOpeningBalanceDate(e.target.value)} />
              </label>
            </div>
          </fieldset>
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
