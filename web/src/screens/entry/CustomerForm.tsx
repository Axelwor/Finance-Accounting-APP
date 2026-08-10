import { useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { parseAmountInput } from "../../lib/format";
import type { CreateCustomerInput, Customer } from "../../types";

type PriceLevel = "RETAIL" | "WHOLESALE" | "DISTRIBUTOR" | "SPECIAL";

export function CustomerForm() {
  const { replaceDraft, markUnsaved } = useWorkbench();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [billingAddress, setBillingAddress] = useState("");
  const [shippingAddress, setShippingAddress] = useState("");
  const [customerGroup, setCustomerGroup] = useState("");
  const [priceLevel, setPriceLevel] = useState<PriceLevel>("RETAIL");
  const [currencyCode, setCurrencyCode] = useState("IDR");
  const [isPkp, setIsPkp] = useState(false);
  const [creditHold, setCreditHold] = useState(false);
  const [website, setWebsite] = useState("");
  const [fax, setFax] = useState("");
  const [contactPerson2, setContactPerson2] = useState("");
  const [phone2, setPhone2] = useState("");
  const [npwpName, setNpwpName] = useState("");
  const [openingBalanceDisplay, setOpeningBalanceDisplay] = useState("");
  const [openingBalanceCents, setOpeningBalanceCents] = useState<number>(0);
  const [openingBalanceDate, setOpeningBalanceDate] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [customer, setCustomer] = useState<Customer | null>(null);

  const priceLevels: PriceLevel[] = ["RETAIL", "WHOLESALE", "DISTRIBUTOR", "SPECIAL"];

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!code || !name) {
      setError("Code and name are required.");
      return;
    }
    setSaving(true);
    try {
      const input: CreateCustomerInput = {
        code,
        name,
        billing_address: billingAddress || undefined,
        shipping_address: shippingAddress || undefined,
        customer_group: customerGroup || undefined,
        price_level: priceLevel,
        currency_code: currencyCode || undefined,
        is_pkp: isPkp,
        credit_hold: creditHold,
        website: website || undefined,
        fax: fax || undefined,
        contact_person_2: contactPerson2 || undefined,
        phone_2: phone2 || undefined,
        npwp_name: npwpName || undefined,
        opening_balance_cents: openingBalanceCents,
        opening_balance_date: openingBalanceDate || undefined,
      };
      const created = await api.createCustomer(input);
      replaceDraft("cus-" + created.id, created.code, "saved");
      markUnsaved("cus-" + created.id, false);
    } catch (err: any) {
      setError(err?.message || "Failed to save customer.");
    } finally {
      setSaving(false);
    }
  }

  const handleOpeningBalanceChange = (value: string) => {
    setOpeningBalanceDisplay(value);
    const clean = value.replace(/[^\d]/g, "");
    const amount = parseInt(clean || "0", 10);
    const cents = Math.round(amount * 100);
    setOpeningBalanceCents(cents);
  };

  if (loading) {
    return <LoadingState label="Loading customer form..." />;
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-title">{customer ? `Edit Customer ${customer.code}` : "New Customer"}</div>
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
          </div>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Info Pajak</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">NPWP Name</span>
                <input className="input" value={npwpName} onChange={(e) => setNpwpName(e.target.value)} />
              </label>
              <label className="field field--checkbox">
                <input
                  type="checkbox"
                  checked={isPkp}
                  onChange={(e) => setIsPkp(e.target.checked)}
                />
                <span className="field__label">PKP (Pengusaha Kena Pajak)</span>
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Alamat Pengiriman/Billing</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Billing Address</span>
                <textarea className="input" rows={3} value={billingAddress} onChange={(e) => setBillingAddress(e.target.value)} />
              </label>
              <label className="field">
                <span className="field__label">Shipping Address</span>
                <textarea className="input" rows={3} value={shippingAddress} onChange={(e) => setShippingAddress(e.target.value)} />
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

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Data Keuangan</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Opening Balance (IDR)</span>
                <input
                  type="text"
                  className="input"
                  value={openingBalanceDisplay}
                  onChange={(e) => handleOpeningBalanceChange(e.target.value)}
                  placeholder="Enter Rupiah amount"
                />
              </label>
              <label className="field">
                <span className="field__label">Opening Balance Date</span>
                <input
                  type="date"
                  className="input"
                  value={openingBalanceDate}
                  onChange={(e) => setOpeningBalanceDate(e.target.value)}
                />
              </label>
            </div>
          </fieldset>

          <fieldset className="fieldset">
            <legend className="fieldset__legend">Harga &amp; Group</legend>
            <div className="entrytab__detail-grid">
              <label className="field">
                <span className="field__label">Price Level</span>
                <select className="input" value={priceLevel} onChange={(e) => setPriceLevel(e.target.value as PriceLevel)}>
                  {priceLevels.map((level) => (
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
                  <option value="IDR">IDR</option>
                  <option value="USD">USD</option>
                </select>
              </label>
              <label className="field field--checkbox">
                <input
                  type="checkbox"
                  checked={creditHold}
                  onChange={(e) => setCreditHold(e.target.checked)}
                />
                <span className="field__label">Credit Hold</span>
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
