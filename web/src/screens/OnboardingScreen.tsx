import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { AmountField, Button, FormError, SelectField, TextField } from "../components/ui";
import type { CurrencyCode } from "../types";
import { formatIDR } from "../lib/format";

type BalanceKey = "cash" | "bank" | "receivables" | "payables" | "equity";

const BALANCE_FIELDS: { key: BalanceKey; label: string; hint?: string }[] = [
  { key: "cash", label: "Cash on hand", hint: "Physical money you hold." },
  { key: "bank", label: "Bank account balance" },
  { key: "receivables", label: "Receivables (money to be received)" },
  { key: "payables", label: "Payables (money to be paid)" },
  { key: "equity", label: "Starting capital" },
];

const BUSINESS_TYPES = [
  "Grocery / convenience store",
  "Cafe / restaurant",
  "Online shop",
  "Services",
  "Workshop",
  "Other",
];

const CURRENCIES: { value: CurrencyCode; label: string }[] = [{ value: "IDR", label: "Rupiah (Rp)" }];

/** 3-step onboarding flow: business details, book period, opening balance. */
export function OnboardingScreen() {
  const [step, setStep] = useState(0);
  const [name, setName] = useState("");
  const [businessType, setBusinessType] = useState("");
  const [currency, setCurrency] = useState<CurrencyCode>("IDR");
  const [year, setYear] = useState(String(new Date().getFullYear()));
  const [startMonth, setStartMonth] = useState("1");
  const [balance, setBalance] = useState<Record<BalanceKey, string>>({
    cash: "",
    bank: "",
    receivables: "",
    payables: "",
    equity: "",
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [finishing, setFinishing] = useState(false);

  const navigate = useNavigate();
  const { setBusiness } = useAppState();

  const totalAssets = (balance.cash ? Number(balance.cash) : 0) + (balance.bank ? Number(balance.bank) : 0) + (balance.receivables ? Number(balance.receivables) : 0);
  const totalLiabilitiesEquity = (balance.payables ? Number(balance.payables) : 0) + (balance.equity ? Number(balance.equity) : 0);

  const monthLabels = [
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
  ];

  const stepValid = (): boolean => {
    if (step === 0) return name.trim().length > 0 && businessType.trim().length > 0;
    if (step === 1) {
      const y = Number(year);
      return y >= 2000 && y <= 2100 && startMonth !== "";
    }
    return true;
  };

  const next = () => {
    setError(null);
    if (step === 0 && !stepValid()) {
      setError("Please fill in business name and business type to continue.");
      return;
    }
    if (step < 2) setStep((s) => s + 1);
  };

  const handleFinish = async () => {
    setError(null);
    setFinishing(true);
    try {
      await api.completeOnboarding({
        business: { name: name.trim(), businessType: businessType.trim(), currency },
        period: { year: Number(year), startMonth: Number(startMonth) },
        openingBalance: {
          cash: Number(balance.cash || 0),
          bank: Number(balance.bank || 0),
          receivables: Number(balance.receivables || 0),
          payables: Number(balance.payables || 0),
          equity: Number(balance.equity || 0),
        },
      });
      const state = api.getLocalState();
      setBusiness(state.business);
      setLoading(true);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
      setFinishing(false);
    }
  };

  const setBalanceField = (key: BalanceKey, raw: string) => {
    setBalance((s) => ({ ...s, [key]: raw.replace(/[^\d]/g, "").slice(0, 15) }));
  };

  const renderStep = () => {
    if (step === 0) {
      return (
        <div className="form-stack">
          <TextField label="Business name" value={name} onChange={setName} placeholder="e.g. Sari Corner Store" />
          <SelectField
            label="Business type"
            value={businessType}
            onChange={setBusinessType}
            options={BUSINESS_TYPES.map((t) => ({ value: t, label: t }))}
            placeholder="Select business type"
          />
          <SelectField
            label="Currency"
            value={currency}
            onChange={(v) => setCurrency(v as CurrencyCode)}
            options={CURRENCIES}
          />
        </div>
      );
    }
    if (step === 1) {
      return (
        <div className="form-stack">
          <TextField
            label="Fiscal year"
            value={year}
            onChange={(v) => setYear(v.replace(/[^\d]/g, "").slice(0, 4))}
            inputMode="numeric"
            hint="The year your bookkeeping starts."
          />
          <SelectField
            label="Book period starts in month"
            value={startMonth}
            onChange={setStartMonth}
            options={monthLabels.map((m, i) => ({ value: String(i + 1), label: m }))}
          />
          <p className="field-note">
            Book period: {monthLabels[Number(startMonth) - 1]} {year} through{" "}
            {monthLabels[(Number(startMonth) + 10) % 12]} {Number(startMonth) === 1 ? year : Number(year) + 1}.
          </p>
        </div>
      );
    }
    return (
      <div className="form-stack">
        {BALANCE_FIELDS.map((f) => (
          <AmountField
            key={f.key}
            label={f.label}
            value={balance[f.key]}
            onChange={(raw) => setBalanceField(f.key, raw)}
            hint={f.hint}
          />
        ))}
        <div className="balance-summary">
          <p className="balance-summary__label">Summary</p>
          <div className="balance-summary__row">
            <span>Total assets (cash + bank + receivables)</span>
            <strong>{formatIDR(totalAssets)}</strong>
          </div>
          <div className="balance-summary__row">
            <span>Payables + capital</span>
            <strong>{formatIDR(totalLiabilitiesEquity)}</strong>
          </div>
          <p className="balance-summary__note">
            Small differences between the two figures are balanced automatically by the system.
          </p>
        </div>
      </div>
    );
  };

  const stepMeta = [
    { label: "Business details", desc: "Name, type, and currency." },
    { label: "Book period", desc: "When your fiscal year starts." },
    { label: "Opening balance", desc: "A summary of your day-one financial position." },
  ];

  return (
    <div className="onboarding">
      <div className="onboarding__head">
        <p className="onboarding__brand">
          <span className="brand__mark" aria-hidden="true" />
          <span className="brand__name">Ledgerly</span>
        </p>
        <h1 className="onboarding__title">Set up your business books</h1>
        <p className="onboarding__sub">Three short steps, about 2 minutes.</p>
      </div>

      <ol className="stepper" aria-label="Onboarding steps">
        {stepMeta.map((s, i) => (
          <li key={s.label} className={`stepper__item${i === step ? " is-active" : i < step ? " is-done" : ""}`}>
            <span className="stepper__num" aria-hidden="true">
              {i < step ? "✓" : i + 1}
            </span>
            <span className="stepper__label">{s.label}</span>
          </li>
        ))}
      </ol>

      <form
        className="onboarding-card"
        onSubmit={(e) => {
          e.preventDefault();
          if (step === 2) void handleFinish();
          else next();
        }}
        noValidate
      >
        <div className="onboarding-card__head">
          <h2 className="onboarding-card__title">{stepMeta[step].label}</h2>
          <p className="onboarding-card__desc">{stepMeta[step].desc}</p>
        </div>

        {renderStep()}
        <FormError message={error} />

        <div className="onboarding-card__actions">
          {step > 0 ? (
            <Button variant="secondary" onClick={() => setStep((s) => s - 1)}>
              Back
            </Button>
          ) : (
            <span />
          )}
          <Button type="submit" variant="primary" disabled={loading || finishing}>
            {step < 2 ? "Next" : finishing ? "Saving..." : "Finish, open dashboard"}
          </Button>
        </div>
      </form>
    </div>
  );
}
