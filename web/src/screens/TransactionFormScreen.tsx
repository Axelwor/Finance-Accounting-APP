import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import {
  AmountField,
  Button,
  DateField,
  ErrorState,
  FormError,
  LoadingState,
  SelectField,
  TextareaField,
} from "../components/ui";
import { todayISO } from "../lib/format";
import type { AccountItem, Category, TransactionKind } from "../types";

const KIND_META: Record<
  TransactionKind,
  { title: string; sub: string; icon: string; submit: string }
> = {
  "money-in": {
    title: "Record money in",
    sub: "Money received by the business, e.g. sales.",
    icon: "+",
    submit: "Save money in",
  },
  "money-out": {
    title: "Record money out",
    sub: "Money spent by the business, e.g. purchases or rent.",
    icon: "-",
    submit: "Save money out",
  },
  transfer: {
    title: "Transfer money",
    sub: "Move money between accounts or cash, without profit or loss.",
    icon: "⇄",
    submit: "Save transfer",
  },
};

/** Transaction input form: money in / money out / transfer. */
export function TransactionFormScreen() {
  const { kindParam } = useParams<{ kindParam: string }>();
  const kind: TransactionKind =
    kindParam === "money-out"
      ? "money-out"
      : kindParam === "transfer"
        ? "transfer"
        : "money-in";
  const meta = KIND_META[kind];

  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(todayISO());
  const [description, setDescription] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  const [categories, setCategories] = useState<Category[]>([]);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const navigate = useNavigate();
  const { setTransactions } = useAppState();

  useEffect(() => {
    document.title = `${meta.title} - Ledgerly`;
  }, [meta.title]);

  const loadMaster = useCallback(async () => {
    setLoadError(null);
    try {
      const [cats, accts] = await Promise.all([
        api.listCategories(),
        api.listAccounts(),
      ]);
      setCategories(cats);
      setAccounts(accts);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load data. Try again.");
    }
  }, []);

  useEffect(() => {
    void loadMaster();
  }, [loadMaster, kind]);

  const categoryOptions =
    kind === "transfer"
      ? []
      : categories.filter((c) => c.kind === kind).map((c) => ({ value: c.id, label: c.name }));

  const accountOptions = accounts.map((a) => ({ value: a.id, label: a.name }));

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    setSaved(false);
    try {
      const { recentTransactions } = await api.createTransaction({
        kind,
        amount: Number(amount || 0),
        date,
        description,
        categoryId: kind === "transfer" ? undefined : categoryId || undefined,
        from: kind === "transfer" ? from || undefined : undefined,
        to: kind === "transfer" ? to || undefined : undefined,
      });
      setTransactions(recentTransactions);
      setSaved(true);
      setAmount("");
      setDescription("");
      setCategoryId("");
      setFrom("");
      setTo("");
      setTimeout(() => navigate("/dashboard"), 1100);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
      setSaving(false);
    }
  };

  return (
    <div className="form-page">
      <header className="page-head">
        <div>
          <h1 className="page-title">{meta.title}</h1>
          <p className="page-sub">{meta.sub}</p>
        </div>
      </header>

      {loadError ? (
        <ErrorState message={loadError} onRetry={() => void loadMaster()} />
      ) : categories.length === 0 ? (
        <LoadingState label="Preparing the form..." />
      ) : (
        <form className="form-card" onSubmit={handleSubmit} noValidate>
          <AmountField label="Amount (IDR)" value={amount} onChange={setAmount} placeholder="0" />

          <DateField label="Date" value={date} onChange={setDate} max={todayISO()} />

          {kind === "transfer" ? (
            <div className="field-row">
              <SelectField label="From account" value={from} onChange={setFrom} options={accountOptions} placeholder="Select account" />
              <SelectField label="To account" value={to} onChange={setTo} options={accountOptions} placeholder="Select account" />
            </div>
          ) : (
            <SelectField
              label="Category"
              value={categoryId}
              onChange={setCategoryId}
              options={categoryOptions}
              placeholder="Select category"
            />
          )}

          <TextareaField
            label="Description"
            value={description}
            onChange={setDescription}
            placeholder="e.g. Today's cash sales"
            hint="A short note so you can recognize it later."
          />

          <FormError message={error} />

          <div className="form-card__actions">
            <Button variant="secondary" to="/dashboard">
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={saving || saved}>
              {saving ? "Saving..." : saved ? "Saved ✓" : meta.submit}
            </Button>
          </div>

          {saved ? (
            <p className="form-card__success" role="status">
              Saved. Taking you to the summary...
            </p>
          ) : null}
        </form>
      )}
    </div>
  );
}
