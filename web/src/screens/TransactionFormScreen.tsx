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
  { title: string; sub: string; submit: string; meta: string }
> = {
  "money-in": {
    title: "Money in",
    sub: "Cash receipt or receivable collected into a cash or bank account.",
    submit: "Rule entry",
    meta: "RECEIPT / +",
  },
  "money-out": {
    title: "Money out",
    sub: "Cash payment or expense against a cash or bank account.",
    submit: "Rule entry",
    meta: "PAYMENT / -",
  },
  transfer: {
    title: "Transfer",
    sub: "Move money between accounts — no profit or loss.",
    submit: "Rule entry",
    meta: "TRANSFER / =",
  },
};

/** Form for a single transaction entry: money in / money out / transfer. */
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
        <div className="page-head__left">
          <p className="page-head__meta"><strong>{meta.meta}</strong></p>
          <h1 className="page-title">
            <span>{meta.title}</span>
            <span className="page-title__sep">/</span>
            <span className="page-title__sub">new entry</span>
          </h1>
          <p className="page-head__meta" style={{ marginTop: "var(--md-sys-spacing-2)", textTransform: "none", letterSpacing: 0, color: "var(--md-sys-color-outline)" }}>
            {meta.sub}
          </p>
        </div>
      </header>

      {loadError ? (
        <ErrorState message={loadError} onRetry={() => void loadMaster()} />
      ) : categories.length === 0 ? (
        <LoadingState label="Loading masters..." />
      ) : (
        <form className="card-panel" onSubmit={handleSubmit} noValidate>
          <div className="card-panel__head">
            <h2 className="card-panel__title">
              <span className={`kind-mark kind-mark--${kind}`}>{kind === "money-in" ? "IN" : kind === "money-out" ? "OUT" : "XFER"}</span>
              {meta.title} entry
            </h2>
            <p className="card-panel__sub">{meta.sub}</p>
          </div>

          <div className="form-stack">
            <AmountField label="Amount" value={amount} onChange={setAmount} placeholder="0" />

            <DateField label="Entry date" value={date} onChange={setDate} max={todayISO()} />

            {kind === "transfer" ? (
              <div className="field-row">
                <SelectField label="From account" value={from} onChange={setFrom} options={accountOptions} placeholder="Select source" />
                <SelectField label="To account" value={to} onChange={setTo} options={accountOptions} placeholder="Select destination" />
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
              label="Note"
              value={description}
              onChange={setDescription}
              placeholder="e.g. Today's cash sales"
              hint="Short ruling so you can trace this later."
            />

            <FormError message={error} />

            <div className="form-card__actions">
              <Button variant="secondary" to="/dashboard">
                Cancel
              </Button>
              <Button type="submit" variant="primary" disabled={saving || saved}>
                {saving ? "Posting..." : saved ? "Posted ✓" : meta.submit}
              </Button>
            </div>

            {saved ? (
              <p className="form-card__success" role="status">
                Entry ruled. Returning to console...
              </p>
            ) : null}
          </div>
        </form>
      )}
    </div>
  );
}
