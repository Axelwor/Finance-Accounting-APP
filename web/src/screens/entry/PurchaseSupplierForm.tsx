import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { parseRupiahToCents } from "../../lib/format";
import type { CreateSupplierInput, PaymentTerm, Supplier } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const SUPPLIER_TYPES = ["GOODS", "SERVICE", "MIXED"] as const;
const CURRENCIES = ["IDR", "USD", "EUR", "SGD", "JPY", "CNY", "AUD", "GBP"] as const;

/**
 * Supplier master-data form. Supports create (draft) and edit: when opened
 * with an `entryId` the existing supplier is loaded via GET /suppliers/{id}
 * and saving PUTs the changes back.
 */
export function PurchaseSupplierForm({ tabId, entryId, initialTitle }: Props) {
  const { markUnsaved, replaceDraft } = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null && entryId !== "";

  // Data Utama
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [supplierType, setSupplierType] = useState<string>("GOODS");
  const [currencyCode, setCurrencyCode] = useState<string>("IDR");
  const [isActive, setIsActive] = useState(true);
  // Tax
  const [npwp, setNpwp] = useState("");
  const [isPkp, setIsPkp] = useState(false);
  // Terms & limit
  const [paymentTermId, setPaymentTermId] = useState("");
  const [creditLimitDisplay, setCreditLimitDisplay] = useState("");
  // Bank
  const [bankName, setBankName] = useState("");
  const [bankAccountNumber, setBankAccountNumber] = useState("");
  const [bankAccountName, setBankAccountName] = useState("");
  // Contact
  const [contact, setContact] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [contact2, setContact2] = useState("");
  const [phone2, setPhone2] = useState("");
  const [fax, setFax] = useState("");
  const [website, setWebsite] = useState("");
  // Address
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [province, setProvince] = useState("");
  const [postalCode, setPostalCode] = useState("");
  // Opening balance (create only)
  const [openingBalance, setOpeningBalance] = useState("");
  const [openingBalanceDate, setOpeningBalanceDate] = useState("");

  const [paymentTerms, setPaymentTerms] = useState<PaymentTerm[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(isEdit);
  const [error, setError] = useState("");
  const [savedSupplier, setSavedSupplier] = useState<Supplier | null>(null);

  useEffect(() => {
    void api.listPaymentTerms().then(setPaymentTerms);
  }, []);

  // Load the supplier being edited.
  useEffect(() => {
    if (!isEdit) return;
    let cancelled = false;
    setLoading(true);
    api
      .getSupplier(Number(entryId))
      .then((supplier) => {
        if (cancelled) return;
        applySupplier(supplier);
        setSavedSupplier(supplier);
        setLoading(false);
        markUnsaved(tabId, false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load supplier.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entryId, isEdit]);

  const applySupplier = (supplier: Supplier) => {
    setCode(supplier.code);
    setName(supplier.name);
    setSupplierType(supplier.supplier_type ?? "GOODS");
    setCurrencyCode(supplier.currency_code ?? "IDR");
    setIsActive(supplier.is_active);
    setNpwp(supplier.npwp ?? "");
    setIsPkp(Boolean(supplier.is_pkp));
    setPaymentTermId(supplier.payment_term_id ? String(supplier.payment_term_id) : "");
    setCreditLimitDisplay(supplier.credit_limit_cents ? centsInput(supplier.credit_limit_cents) : "");
    setBankName(supplier.bank_name ?? "");
    setBankAccountNumber(supplier.bank_account_number ?? "");
    setBankAccountName(supplier.bank_account_name ?? "");
    setContact(supplier.contact_person ?? "");
    setPhone(supplier.phone ?? "");
    setEmail(supplier.email ?? "");
    setContact2(supplier.contact_person_2 ?? "");
    setPhone2(supplier.phone_2 ?? "");
    setFax(supplier.fax ?? "");
    setWebsite(supplier.website ?? "");
    setAddress(supplier.address ?? "");
    setCity(supplier.city ?? "");
    setProvince(supplier.province ?? "");
    setPostalCode(supplier.postal_code ?? "");
    setOpeningBalance(supplier.opening_balance_cents ? centsInput(supplier.opening_balance_cents) : "");
    setOpeningBalanceDate(supplier.opening_balance_date ?? "");
  };

  useEffect(() => {
    markUnsaved(tabId, true);
  }, [
    tabId, code, name, supplierType, currencyCode, isActive, npwp, isPkp,
    paymentTermId, creditLimitDisplay, bankName, bankAccountNumber, bankAccountName,
    contact, phone, email, contact2, phone2, fax, website, address, city,
    province, postalCode, openingBalance, openingBalanceDate, markUnsaved,
  ]);

  const buildInput = (): CreateSupplierInput => ({
    code: code.trim(),
    name: name.trim(),
    contact_person: contact.trim() || undefined,
    phone: phone.trim() || undefined,
    email: email.trim() || undefined,
    address: address.trim() || undefined,
    city: city.trim() || undefined,
    province: province.trim() || undefined,
    postal_code: postalCode.trim() || undefined,
    payment_term_id: paymentTermId ? Number(paymentTermId) : undefined,
    credit_limit_cents: parseRupiahToCents(creditLimitDisplay) || undefined,
    npwp: npwp.trim() || undefined,
    supplier_type: supplierType as CreateSupplierInput["supplier_type"],
    is_pkp: isPkp,
    currency_code: currencyCode,
    bank_name: bankName.trim() || undefined,
    bank_account_number: bankAccountNumber.trim() || undefined,
    bank_account_name: bankAccountName.trim() || undefined,
    website: website.trim() || undefined,
    fax: fax.trim() || undefined,
    contact_person_2: contact2.trim() || undefined,
    phone_2: phone2.trim() || undefined,
  });

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!code.trim() || !name.trim()) {
      setError("Code and name are required.");
      return;
    }
    setSaving(true);
    const payload = buildInput();
    try {
      if (isEdit) {
        const updated = await api.updateSupplier(Number(entryId), payload);
        setSavedSupplier(updated);
        replaceDraft(tabId, `${updated.code} · ${updated.name}`, updated.is_active ? "ACTIVE" : "INACTIVE", updated.id);
      } else {
        const ob = parseRupiahToCents(openingBalance);
        if (ob > 0) {
          payload.opening_balance_cents = ob;
          payload.opening_balance_date = openingBalanceDate || undefined;
        }
        const supplier = await api.createSupplier(payload);
        setSavedSupplier(supplier);
        replaceDraft(tabId, `${supplier.code} · ${supplier.name}`, supplier.is_active ? "ACTIVE" : "INACTIVE", supplier.id);
      }
        markUnsaved(tabId, false);
    } catch (err: unknown) {
      // Surface the real backend message (ApiError carries .message) instead
      // of a generic fallback, so a failed save is never silent.
      const message =
        typeof (err as { message?: unknown } | null | undefined)?.message === "string"
          ? (err as { message: string }).message
          : "Failed to save supplier.";
      setError(message);
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <LoadingState label="Loading supplier..." />;
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__header">
        <div className="entrytab__header-title">
          {isEdit ? `Edit Supplier · ${initialTitle ?? savedSupplier?.code ?? ""}` : "New Supplier"}
        </div>
        <span className={`entrytab__status ${isActive ? "" : "entrytab__status--draft"}`}>
          {isActive ? "ACTIVE" : "INACTIVE"}
        </span>
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
              <label className="field field--checkbox">
                <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} disabled />
                <span className="field__label">Active</span>
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
            <legend className="entrytab__legend">Termin &amp; Limit</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Payment Term</span>
                <select className="input" value={paymentTermId} onChange={(e) => setPaymentTermId(e.target.value)}>
                  <option value="">(none)</option>
                  {paymentTerms.map((term) => (
                    <option key={term.id} value={term.id}>
                      {term.code} · {term.name} ({term.due_days}d)
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Credit Limit (IDR)</span>
                <input
                  className="input"
                  type="text"
                  inputMode="numeric"
                  value={creditLimitDisplay}
                  onChange={(e) => setCreditLimitDisplay(digitOnly(e.target.value))}
                  placeholder="0"
                />
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
              <label className="field">
                <span className="field__label">Province</span>
                <input className="input" value={province} onChange={(e) => setProvince(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Postal Code</span>
                <input className="input" value={postalCode} onChange={(e) => setPostalCode(e.target.value)} />
              </label>
            </div>
          </fieldset>

          {!isEdit && (
            <fieldset className="entrytab__fieldset">
              <legend className="entrytab__legend">Saldo Awal Hutang</legend>
              <div className="entrytab__detail-grid">
                <label className="field">
                  <span className="field__label">Saldo Awal (Rp)</span>
                  <input
                    className="input"
                    type="text"
                    inputMode="numeric"
                    value={openingBalance}
                    onChange={(e) => setOpeningBalance(digitOnly(e.target.value))}
                    placeholder="0"
                  />
                </label>
                <label className="field">
                  <span className="field__label">Tanggal Saldo Awal</span>
                  <input className="input" type="date" value={openingBalanceDate} onChange={(e) => setOpeningBalanceDate(e.target.value)} />
                </label>
              </div>
            </fieldset>
          )}
        </div>
        <aside className="action-rail">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            {saving ? "Saving..." : isEdit ? "Save Changes" : "Save"}
          </button>
        </aside>
        <FormError message={error} />
      </div>
    </form>
  );
}

function centsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(Math.round(cents / 100));
}

function digitOnly(raw: string): string {
  return (raw || "").replace(/[^\d]/g, "").slice(0, 15);
}
