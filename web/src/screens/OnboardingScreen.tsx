import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { AmountField, Button, FormError, SelectField, TextField } from "../components/ui";
import type { CurrencyCode } from "../types";
import { formatIDR } from "../lib/format";

type BalanceKey = "cash" | "bank" | "receivables" | "payables" | "equity";

const BALANCE_FIELDS: { key: BalanceKey; label: string; hint?: string }[] = [
  { key: "cash", label: "Cash on hand", hint: "Physical money in the drawer." },
  { key: "bank", label: "Bank balance" },
  { key: "receivables", label: "Receivables", hint: "Money owed to the business." },
  { key: "payables", label: "Payables", hint: "Money the business owes." },
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

const CURRENCIES: { value: CurrencyCode; label: string }[] = [{ value: "IDR", label: "Rupiah (IDR)" }];

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

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
  const [finishing, setFinishing] = useState(false);

  const navigate = useNavigate();
  const { setBusiness } = useAppState();

  const totalAssets =
    (balance.cash ? Number(balance.cash) : 0) +
    (balance.bank ? Number(balance.bank) : 0) +
    (balance.receivables ? Number(balance.receivables) : 0);
  const totalLiabilitiesEquity =
    (balance.payables ? Number(balance.payables) : 0) + (balance.equity ? Number(balance.equity) : 0);

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
            options={MONTHS.map((m, i) => ({ value: String(i + 1), label: m }))}
          />
          <p className="field-note">
            Book period: {MONTHS[Number(startMonth) - 1]} {year} through{" "}
            {MONTHS[(Number(startMonth) + 10) % 12]} {Number(startMonth) === 1 ? year : Number(year) + 1}.
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
          <p className="balance-summary__label">Day-one ruling</p>
          <div className="balance-summary__row">
            <span>Total assets</span>
            <strong>{formatIDR(totalAssets)}</strong>
          </div>
          <div className="balance-summary__row">
            <span>Payables + capital</span>
            <strong>{formatIDR(totalLiabilitiesEquity)}</strong>
          </div>
          <p className="balance-summary__note">
            The system will balance any difference against capital automatically.
          </p>
        </div>
      </div>
    );
  };

  const stepMeta = [
    { label: "Business details", desc: "Name, type, and currency." },
    { label: "Book period", desc: "When the fiscal year starts." },
    { label: "Opening balance", desc: "Your day-one financial position." },
  ];

  return (
    <div className="onboarding">
      <div className="onboarding__head">
        <p className="onboarding__meta">Setup / Three short steps</p>
        <h1 className="onboarding__title">
          Open the <em>ledger</em>
        </h1>
        <p className="onboarding__sub">Rule your books in three short steps, about two minutes.</p>
      </div>

      <ol className="stepper" aria-label="Onboarding steps">
        {stepMeta.map((s, i) => (
          <li
            key={s.label}
            className={`stepper__item${i === step ? " is-active" : i < step ? " is-done" : ""}`}
          >
            <span className="stepper__num" aria-hidden="true">
              {String(i + 1).padStart(2, "0")}
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
          <Button type="submit" variant="primary" disabled={finishing}>
            {step < 2 ? "Next" : finishing ? "Ruling..." : "Open the dashboard"}
          </Button>
        </div>
      </form>
    </div>
  );
}
