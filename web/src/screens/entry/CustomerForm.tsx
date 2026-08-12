import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { BackendAccount, CreateCustomerInput, Customer, PaymentTerm } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const PRICE_LEVELS = ["RETAIL", "WHOLESALE", "DISTRIBUTOR", "SPECIAL"] as const;
const CURRENCIES = ["IDR", "USD", "EUR", "SGD", "JPY", "CNY", "AUD", "GBP"] as const;

/**
 * Customer master-data form. Supports create (draft) and edit: when opened
 * with an `entryId` the existing customer is loaded via GET /customers/{id}
 * and saving PUTs the changes back.
 */
export function CustomerForm({ tabId, entryId, initialTitle }: Props) {
  const { markUnsaved, replaceDraft } = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null && entryId !== "";

  // Identity
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  // Contact
  const [contactPerson, setContactPerson] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  // Address
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [province, setProvince] = useState("");
  const [postalCode, setPostalCode] = useState("");
  const [billingAddress, setBillingAddress] = useState("");
  const [shippingAddress, setShippingAddress] = useState("");
  // Terms & defaults
  const [paymentTermId, setPaymentTermId] = useState("");
  const [creditLimitDisplay, setCreditLimitDisplay] = useState("");
  const [defaultRevenueAccountId, setDefaultRevenueAccountId] = useState("");
  const [defaultReceivableAccountId, setDefaultReceivableAccountId] = useState("");
  const [isActive, setIsActive] = useState(true);
  // Group & pricing
  const [customerGroup, setCustomerGroup] = useState("");
  const [priceLevel, setPriceLevel] = useState<(typeof PRICE_LEVELS)[number]>("RETAIL");
  const [currencyCode, setCurrencyCode] = useState("IDR");
  // Tax
  const [npwp, setNpwp] = useState("");
  const [npwpName, setNpwpName] = useState("");
  const [isPkp, setIsPkp] = useState(false);
  const [creditHold, setCreditHold] = useState(false);
  // Extra contact
  const [website, setWebsite] = useState("");
  const [fax, setFax] = useState("");
  const [contactPerson2, setContactPerson2] = useState("");
  const [phone2, setPhone2] = useState("");
  // Opening balance (create only)
  const [openingBalanceDisplay, setOpeningBalanceDisplay] = useState("");
  const [openingBalanceDate, setOpeningBalanceDate] = useState("");

  // Masters
  const [paymentTerms, setPaymentTerms] = useState<PaymentTerm[]>([]);
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);

  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(isEdit);
  const [error, setError] = useState("");
  const [savedCustomer, setSavedCustomer] = useState<Customer | null>(null);

  // Load master data (payment terms + accounts for the default-account pickers).
  useEffect(() => {
    void api.listPaymentTerms().then(setPaymentTerms);
    void api
      .listBackendAccounts()
      .then((list) => setAccounts(list.filter((a) => !a.is_group && a.is_active)));
  }, []);

  // Load the customer being edited.
  useEffect(() => {
    if (!isEdit) return;
    let cancelled = false;
    setLoading(true);
    api
      .getCustomer(Number(entryId))
      .then((customer) => {
        if (cancelled) return;
        applyCustomer(customer);
        setSavedCustomer(customer);
        setLoading(false);
        markUnsaved(tabId, false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load customer.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entryId, isEdit]);

  const applyCustomer = (customer: Customer) => {
    setCode(customer.code);
    setName(customer.name);
    setContactPerson(customer.contact_person ?? "");
    setPhone(customer.phone ?? "");
    setEmail(customer.email ?? "");
    setAddress(customer.address ?? "");
    setCity(customer.city ?? "");
    setProvince(customer.province ?? "");
    setPostalCode(customer.postal_code ?? "");
    setBillingAddress(customer.billing_address ?? "");
    setShippingAddress(customer.shipping_address ?? "");
    setPaymentTermId(customer.payment_term_id ? String(customer.payment_term_id) : "");
    setCreditLimitDisplay(customer.credit_limit_cents ? centsInput(customer.credit_limit_cents) : "");
    setDefaultRevenueAccountId(customer.default_revenue_account_id ? String(customer.default_revenue_account_id) : "");
    setDefaultReceivableAccountId(customer.default_receivable_account_id ? String(customer.default_receivable_account_id) : "");
    setIsActive(customer.is_active);
    setCustomerGroup(customer.customer_group ?? "");
    setPriceLevel((customer.price_level as (typeof PRICE_LEVELS)[number]) ?? "RETAIL");
    setCurrencyCode(customer.currency_code ?? "IDR");
    setNpwp(customer.npwp ?? "");
    setNpwpName(customer.npwp_name ?? "");
    setIsPkp(customer.is_pkp);
    setCreditHold(customer.credit_hold);
    setWebsite(customer.website ?? "");
    setFax(customer.fax ?? "");
    setContactPerson2(customer.contact_person_2 ?? "");
    setPhone2(customer.phone_2 ?? "");
    setOpeningBalanceDisplay(customer.opening_balance_cents ? centsInput(customer.opening_balance_cents) : "");
    setOpeningBalanceDate(customer.opening_balance_date ?? "");
  };

  // Flag the tab unsaved whenever a field changes.
  useEffect(() => {
    markUnsaved(tabId, true);
  }, [
    tabId, code, name, contactPerson, phone, email, address, city, province,
    postalCode, billingAddress, shippingAddress, paymentTermId, creditLimitDisplay,
    defaultRevenueAccountId, defaultReceivableAccountId, isActive, customerGroup,
    priceLevel, currencyCode, npwp, npwpName, isPkp, creditHold, website, fax,
    contactPerson2, phone2, openingBalanceDisplay, openingBalanceDate, markUnsaved,
  ]);

  const buildInput = (): CreateCustomerInput => ({
    code: code.trim(),
    name: name.trim(),
    contact_person: contactPerson.trim() || undefined,
    phone: phone.trim() || undefined,
    email: email.trim() || undefined,
    address: address.trim() || undefined,
    city: city.trim() || undefined,
    province: province.trim() || undefined,
    postal_code: postalCode.trim() || undefined,
    payment_term_id: paymentTermId ? Number(paymentTermId) : undefined,
    credit_limit_cents: parseCents(creditLimitDisplay) || undefined,
    default_revenue_account_id: defaultRevenueAccountId ? Number(defaultRevenueAccountId) : undefined,
    default_receivable_account_id: defaultReceivableAccountId ? Number(defaultReceivableAccountId) : undefined,
    is_active: isActive,
    billing_address: billingAddress.trim() || undefined,
    shipping_address: shippingAddress.trim() || undefined,
    customer_group: customerGroup.trim() || undefined,
    price_level: priceLevel,
    currency_code: currencyCode,
    npwp: npwp.trim() || undefined,
    npwp_name: npwpName.trim() || undefined,
    is_pkp: isPkp,
    credit_hold: creditHold,
    website: website.trim() || undefined,
    fax: fax.trim() || undefined,
    contact_person_2: contactPerson2.trim() || undefined,
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
    const input = buildInput();
    try {
      if (isEdit) {
        const updated = await api.updateCustomer(Number(entryId), input);
        setSavedCustomer(updated);
        replaceDraft(tabId, `${updated.code} · ${updated.name}`, updated.is_active ? "ACTIVE" : "INACTIVE", updated.id);
      } else {
        // Opening balance is only meaningful on create.
        const obCents = parseCents(openingBalanceDisplay);
        if (obCents > 0) {
          input.opening_balance_cents = obCents;
          input.opening_balance_date = openingBalanceDate || undefined;
        }
        const created = await api.createCustomer(input);
        setSavedCustomer(created);
        replaceDraft(tabId, created.code, created.is_active ? "ACTIVE" : "INACTIVE", created.id);
      }
        markUnsaved(tabId, false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save customer.");
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <LoadingState label="Loading customer..." />;
  }

  const revenueAccounts = accounts.filter((a) => a.report_group === "revenue");
  const receivableAccounts = accounts.filter(
    (a) => a.report_group === "asset" || a.account_type === "RECEIVABLE",
  );

  return (
    <form className="entrytab" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__header">
        <div className="entrytab__header-title">
          {isEdit ? `Edit Customer · ${initialTitle ?? savedCustomer?.code ?? ""}` : "New Customer"}
        </div>
        <span className={`entrytab__status ${isActive ? "" : "entrytab__status--draft"}`}>
          {isActive ? "ACTIVE" : "INACTIVE"}
        </span>
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
            <label className="field field--checkbox">
              <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
              <span className="field__label">Active</span>
            </label>
          </div>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Kontak</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Contact Person</span>
                <input className="input" value={contactPerson} onChange={(e) => setContactPerson(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Phone</span>
                <input className="input" value={phone} onChange={(e) => setPhone(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Email</span>
                <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Alamat</legend>
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
              <label className="field">
                <span className="field__label">Billing Address</span>
                <textarea className="input" rows={2} value={billingAddress} onChange={(e) => setBillingAddress(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Shipping Address</span>
                <textarea className="input" rows={2} value={shippingAddress} onChange={(e) => setShippingAddress(e.target.value)} />
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Termin &amp; Limit</legend>
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
              <label className="field">
                <span className="field__label">Default Revenue Account</span>
                <select className="input" value={defaultRevenueAccountId} onChange={(e) => setDefaultRevenueAccountId(e.target.value)}>
                  <option value="">(none)</option>
                  {revenueAccounts.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.code} · {a.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Default Receivable Account</span>
                <select className="input" value={defaultReceivableAccountId} onChange={(e) => setDefaultReceivableAccountId(e.target.value)}>
                  <option value="">(none)</option>
                  {receivableAccounts.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.code} · {a.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Info Pajak</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">NPWP</span>
                <input className="input" value={npwp} onChange={(e) => setNpwp(e.target.value)} placeholder="00.000.000.0-000.000" />
              </label>
              <label className="field">
                <span className="field__label">NPWP Name</span>
                <input className="input" value={npwpName} onChange={(e) => setNpwpName(e.target.value)} />
              </label>
              <label className="field field--checkbox">
                <input type="checkbox" checked={isPkp} onChange={(e) => setIsPkp(e.target.checked)} />
                <span className="field__label">PKP (Pengusaha Kena Pajak)</span>
              </label>
              <label className="field field--checkbox">
                <input type="checkbox" checked={creditHold} onChange={(e) => setCreditHold(e.target.checked)} />
                <span className="field__label">Credit Hold</span>
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Harga &amp; Group</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Price Level</span>
                <select className="input" value={priceLevel} onChange={(e) => setPriceLevel(e.target.value as (typeof PRICE_LEVELS)[number])}>
                  {PRICE_LEVELS.map((level) => (
                    <option key={level} value={level}>{level}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field__label">Customer Group</span>
                <input className="input" value={customerGroup} onChange={(e) => setCustomerGroup(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Currency Code</span>
                <select className="input" value={currencyCode} onChange={(e) => setCurrencyCode(e.target.value)}>
                  {CURRENCIES.map((c) => (
                    <option key={c} value={c}>{c}</option>
                  ))}
                </select>
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Kontak Tambahan</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Website</span>
                <input className="input" value={website} onChange={(e) => setWebsite(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Fax</span>
                <input className="input" value={fax} onChange={(e) => setFax(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Contact Person 2</span>
                <input className="input" value={contactPerson2} onChange={(e) => setContactPerson2(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Phone 2</span>
                <input className="input" value={phone2} onChange={(e) => setPhone2(e.target.value)} />
              </label>
            </div>
          </fieldset>

          {!isEdit && (
            <fieldset className="fieldset">
              <legend className="fieldset__legend">Saldo Awal Piutang</legend>
              <div className="entrytab__detail-grid">
                <label className="field">
                  <span className="field__label">Opening Balance (IDR)</span>
                  <input
                    className="input"
                    type="text"
                    inputMode="numeric"
                    value={openingBalanceDisplay}
                    onChange={(e) => setOpeningBalanceDisplay(digitOnly(e.target.value))}
                    placeholder="0"
                  />
                </label>
                <label className="field">
                  <span className="field__label">Opening Balance Date</span>
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

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) * 100 : 0;
}

function centsInput(cents: number): string {
  if (!cents) return "";
  // Stored as cents; display whole rupiah.
  return new Intl.NumberFormat("en-US").format(Math.round(cents / 100));
}

function digitOnly(raw: string): string {
  return (raw || "").replace(/[^\d]/g, "").slice(0, 15);
}
