import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { EntrySubKind } from "../../types";

interface Props {
  tabId: string;
  subKind: EntrySubKind;
  /** Entry form title, e.g. "Sales Invoice", "Asset". */
  title: string;
  /** Existing number (when opening a persisted row). */
  initialTitle?: string;
}

interface CounterLine {
  id: string;
  accountId: string;
  description: string;
  amount: string;
}

/** Field labels per sub-kind — the "party" field has different names. */
const PARTY_LABEL: Record<EntrySubKind, string> = {
  "money-in": "Received From",
  "money-out": "Paid To",
  "cash-transfer": "Memo",
  "sales-invoice": "Customer",
  "sales-receipt": "Payer",
  "purchase-invoice": "Supplier",
  "purchase-payment": "Supplier",
  "inventory-item": "Item Name",
  "asset-register": "Asset Name",
};

const HEADER_TITLE: Record<EntrySubKind, string> = {
  "money-in": "Other Receipt",
  "money-out": "Other Payment",
  "cash-transfer": "Bank Transfer",
  "sales-invoice": "Sales Invoice",
  "sales-receipt": "Sales Receipt",
  "purchase-invoice": "Purchase Invoice",
  "purchase-payment": "Purchase Payment",
  "inventory-item": "Inventory Item",
  "asset-register": "Asset",
};

/** Default account shown in the locked-line placeholder for each demo module. */
const PRIMARY_ACCOUNT_HINT: Record<EntrySubKind, { side: "debit" | "credit"; label: string }> = {
  "money-in": { side: "debit", label: "Cash / Bank" },
  "money-out": { side: "credit", label: "Cash / Bank" },
  "cash-transfer": { side: "debit", label: "From account" },
  "sales-invoice": { side: "credit", label: "Receivable" },
  "sales-receipt": { side: "debit", label: "Cash / Bank" },
  "purchase-invoice": { side: "debit", label: "Inventory / Expense" },
  "purchase-payment": { side: "credit", label: "Cash / Bank" },
  "inventory-item": { side: "debit", label: "Inventory Asset" },
  "asset-register": { side: "debit", label: "Fixed Asset" },
};

/**
 * Entry form stub for modules that don't yet have a real backend
 * (Sales, Purchases, Inventory, Fixed Assets). Mirrors the
 * CashEntryForm chrome: header with status pill + action bar, header
 * section with date + number + party, a multi-line account grid where
 * the first line is locked to the primary account for the module and
 * the counter rows can be added/removed. Posting is disabled in demo
 * mode.
 */
export function MockEntryForm({ tabId, subKind, title, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber(subKind));
  const [party, setParty] = useState("");
  const [description, setDescription] = useState("");
  const [memo, setMemo] = useState("");
  const [counterLines, setCounterLines] = useState<CounterLine[]>([seedCounterLine()]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, party, description, memo, counterLines, workbench]);

  const primary = PRIMARY_ACCOUNT_HINT[subKind];
  const cashAmountCents = useMemo(
    () => counterLines.reduce((sum, line) => sum + parseCents(line.amount), 0),
    [counterLines],
  );
  const countersTotals = cashAmountCents;
  const diff = (primary.side === "debit" ? cashAmountCents : 0) - (primary.side === "credit" ? cashAmountCents : 0);

  const updateCounter = (id: string, patch: Partial<CounterLine>) => {
    setCounterLines((current) => current.map((line) => (line.id === id ? { ...line, ...patch } : line)));
  };
  const removeCounter = (id: string) => {
    setCounterLines((current) => (current.length > 1 ? current.filter((line) => line.id !== id) : current));
  };
  const addCounter = () => {
    setCounterLines((current) => [...current, seedCounterLine()]);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError("This module is in demo mode. Connect a backend endpoint to post entries.");
  };

  const headerLabel = HEADER_TITLE[subKind] ?? title;
  const partyLabel = PARTY_LABEL[subKind] ?? "Name";
  const status = initialTitle && !initialTitle.endsWith("DRAFT") ? "POSTED" : "DRAFT";

  return (
    <form className="entrytab" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{headerLabel}</span>
          <span
            className={`entrytab__status ${
              status === "POSTED" ? "entrytab__status--posted" : "entrytab__status--draft"
            }`}
          >
            {status}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="listtab__demo" title="Demo mode — no backend endpoint yet">
            Demo
          </span>
        </div>
        <div className="entrytab__actions">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => workbench.close(tabId)}>
            Close
          </button>
          <button type="button" className="btn btn--ghost btn--sm" disabled>
            Print
          </button>
          <button type="button" className="btn btn--ghost btn--sm" disabled>
            More
          </button>
          <button type="submit" className="btn btn--ink btn--sm">
            Save
          </button>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__section">
          <div className="entrytab__section-title">Header</div>

          <label className="field">
            <span className="field__label">Date</span>
            <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </label>

          <label className="field">
            <span className="field__label">Number</span>
            <input className="input" value={number} onChange={(e) => setNumber(e.target.value)} />
          </label>

          <label className="field">
            <span className="field__label">{partyLabel}</span>
            <input
              className="input"
              value={party}
              onChange={(e) => setParty(e.target.value)}
              placeholder={`${partyLabel} name`}
            />
          </label>

          <label className="field">
            <span className="field__label">Description</span>
            <input
              className="input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Short description"
            />
          </label>
        </div>

        <div className="entrytab__section">
          <div className="entrytab__section-title">Account lines</div>

          <div className="entry-grid">
            <div className="entry-grid__head">
              <div>#</div>
              <div>Account</div>
              <div>Description</div>
              <div className="right">Debit</div>
              <div className="right">Credit</div>
              <div aria-hidden="true" />
            </div>

            <div className="entry-grid__row entry-grid__row--locked">
              <div className="entry-grid__num">1</div>
              <div>
                <input
                  type="text"
                  value={primary.label}
                  readOnly
                  aria-readonly="true"
                  className="entry-grid__readonly"
                  title="Locked primary account"
                />
              </div>
              <div>
                <input
                  type="text"
                  value={description || primary.label}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Line memo"
                />
              </div>
              <div>
                <input
                  className="amount"
                  type="text"
                  inputMode="numeric"
                  value={primary.side === "credit" ? "" : formatAmountInput(String(cashAmountCents))}
                  readOnly
                  aria-readonly="true"
                />
              </div>
              <div>
                <input
                  className="amount"
                  type="text"
                  inputMode="numeric"
                  value={primary.side === "debit" ? "" : formatAmountInput(String(cashAmountCents))}
                  readOnly
                  aria-readonly="true"
                />
              </div>
              <div aria-hidden="true" />
            </div>

            {counterLines.map((line, idx) => (
              <div className="entry-grid__row" key={line.id}>
                <div className="entry-grid__num">{idx + 2}</div>
                <div>
                  <input
                    type="text"
                    value={line.accountId}
                    onChange={(e) => updateCounter(line.id, { accountId: e.target.value })}
                    placeholder="Account code"
                  />
                </div>
                <div>
                  <input
                    type="text"
                    value={line.description}
                    onChange={(e) => updateCounter(line.id, { description: e.target.value })}
                    placeholder="Line memo"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={primary.side === "credit" ? formatAmountInput(line.amount) : ""}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateCounter(line.id, { amount: digits });
                    }}
                    placeholder="0"
                    readOnly={primary.side === "debit"}
                    aria-readonly={primary.side === "debit" ? "true" : undefined}
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={primary.side === "debit" ? formatAmountInput(line.amount) : ""}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateCounter(line.id, { amount: digits });
                    }}
                    placeholder="0"
                    readOnly={primary.side === "credit"}
                    aria-readonly={primary.side === "credit" ? "true" : undefined}
                  />
                </div>
                <div>
                  <button
                    type="button"
                    className="entry-grid__remove"
                    onClick={() => removeCounter(line.id)}
                    aria-label="Remove counter line"
                    disabled={counterLines.length === 1}
                  >
                    ×
                  </button>
                </div>
              </div>
            ))}

            <div className="entry-grid__row entry-grid__row--add">
              <div className="entry-grid__num">+</div>
              <div>
                <button type="button" className="entry-grid__add" onClick={addCounter}>
                  + Add counter line
                </button>
              </div>
              <div />
              <div />
              <div />
              <div />
            </div>
          </div>

          <div className="entry-grid__totals">
            <span>Totals</span>
            <span>
              D <strong>{formatIDR(primary.side === "debit" ? cashAmountCents : 0)}</strong>
            </span>
            <span>
              C <strong>{formatIDR(primary.side === "credit" ? cashAmountCents : 0)}</strong>
            </span>
            <span className={diff === 0 ? "" : "is-off"}>
              Diff <strong>{formatIDR(Math.abs(diff))}</strong>
            </span>
          </div>
        </div>

        <FormError message={error} />
      </div>
    </form>
  );
}

function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function formatAmountInput(raw: string): string {
  if (!raw) return "";
  const digits = raw.replace(/[^\d]/g, "");
  if (!digits) return "";
  return new Intl.NumberFormat("en-US").format(parseInt(digits, 10));
}

function seedCounterLine(): CounterLine {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: "",
    description: "",
    amount: "",
  };
}
