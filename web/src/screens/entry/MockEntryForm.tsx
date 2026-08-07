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

interface AccountLine {
  id: string;
  accountCode: string;
  accountName: string;
  description: string;
  amount: string;
  side: "debit" | "credit";
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

/**
 * Generic entry form stub for the modules that don't yet have a real
 * backend (Sales, Purchases, Inventory, Fixed Assets). It mirrors the
 * CashEntryForm chrome — header with status pill, action bar, header
 * section with date + number + party, a 2-row account grid, and a
 * memo footer — but Save only displays a FormError noting that the
 * module is in demo mode.
 */
export function MockEntryForm({ tabId, subKind, title, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber(subKind));
  const [party, setParty] = useState("");
  const [description, setDescription] = useState("");
  const [memo, setMemo] = useState("");
  const [lines, setLines] = useState<AccountLine[]>(() => seedLines(subKind));
  const [error, setError] = useState<string | null>(null);

  // Mark the tab as unsaved whenever the user touches the form.
  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, party, description, memo, lines, workbench]);

  const totals = useMemo(() => {
    let debit = 0;
    let credit = 0;
    for (const line of lines) {
      const cents = parseCents(line.amount);
      if (line.side === "debit") debit += cents;
      else credit += cents;
    }
    return { debit, credit, diff: debit - credit };
  }, [lines]);

  const updateLine = (id: string, patch: Partial<AccountLine>) => {
    setLines((current) => current.map((line) => (line.id === id ? { ...line, ...patch } : line)));
  };

  const removeLine = (id: string) => {
    setLines((current) => (current.length > 2 ? current.filter((line) => line.id !== id) : current));
  };

  const addLine = (side: "debit" | "credit") => {
    setLines((current) => [
      ...current,
      {
        id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        accountCode: "",
        accountName: "",
        description: "",
        amount: "",
        side,
      },
    ]);
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
          <span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", color: "var(--ink-muted)" }}>
            {number}
          </span>
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

        <div>
          <div className="entrytab__section-title" style={{ padding: "var(--space-5) var(--space-5) var(--space-2)" }}>
            Account lines
          </div>
          <div className="entry-grid">
            <div className="entry-grid__head">
              <div>#</div>
              <div>Account code</div>
              <div>Account name</div>
              <div>Description</div>
              <div className="right">Debit</div>
              <div className="right">Credit</div>
              <div aria-hidden="true" />
            </div>
            {lines.map((line, idx) => (
              <div className="entry-grid__row" key={line.id}>
                <div className="entry-grid__num">{idx + 1}</div>
                <div>
                  <input
                    type="text"
                    value={line.accountCode}
                    onChange={(e) => updateLine(line.id, { accountCode: e.target.value })}
                    placeholder="0000"
                    style={{ fontFamily: "var(--font-mono)" }}
                  />
                </div>
                <div>
                  <input
                    type="text"
                    value={line.accountName}
                    onChange={(e) => updateLine(line.id, { accountName: e.target.value })}
                    placeholder="Account name"
                  />
                </div>
                <div>
                  <input
                    type="text"
                    value={line.description}
                    onChange={(e) => updateLine(line.id, { description: e.target.value })}
                    placeholder="Line memo"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={line.side === "credit" ? "" : formatAmountInput(line.amount)}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateLine(line.id, { amount: digits, side: "debit" });
                    }}
                    placeholder="0"
                  />
                </div>
                <div>
                  <input
                    className="amount"
                    type="text"
                    inputMode="numeric"
                    value={line.side === "debit" ? "" : formatAmountInput(line.amount)}
                    onChange={(e) => {
                      const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                      updateLine(line.id, { amount: digits, side: "credit" });
                    }}
                    placeholder="0"
                  />
                </div>
                <div>
                  <button
                    type="button"
                    className="entry-grid__remove"
                    onClick={() => removeLine(line.id)}
                    aria-label="Remove line"
                  >
                    ×
                  </button>
                </div>
              </div>
            ))}
            <div className="entry-grid__row">
              <div className="entry-grid__num">+</div>
              <div style={{ display: "flex", gap: "var(--space-2)" }}>
                <button type="button" className="entry-grid__add" onClick={() => addLine("debit")}>
                  + Debit line
                </button>
                <button type="button" className="entry-grid__add" onClick={() => addLine("credit")}>
                  + Credit line
                </button>
              </div>
              <div />
              <div />
              <div />
              <div />
              <div />
            </div>
          </div>
          <div className="entry-grid__totals">
            <span>Totals</span>
            <span>
              D <strong>{formatIDR(totals.debit)}</strong>
            </span>
            <span>
              C <strong>{formatIDR(totals.credit)}</strong>
            </span>
            <span className={totals.diff === 0 ? "" : "is-off"}>
              Diff <strong>{formatIDR(Math.abs(totals.diff))}</strong>
            </span>
          </div>
        </div>

        <div className="entrytab__section" style={{ gridTemplateColumns: "1fr" }}>
          <div className="entrytab__section-title">Memo</div>
          <label className="field">
            <span className="field__label">Internal note</span>
            <textarea
              className="input"
              rows={3}
              value={memo}
              onChange={(e) => setMemo(e.target.value)}
              placeholder="Anything to remember about this entry..."
            />
          </label>
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

function seedLines(subKind: EntrySubKind): AccountLine[] {
  const seedText =
    subKind === "sales-invoice"
      ? { d: "Revenue / Pendapatan", c: "Receivable / Piutang" }
      : subKind === "sales-receipt"
        ? { d: "Cash / Bank", c: "Receivable / Piutang" }
        : subKind === "purchase-invoice"
          ? { d: "Inventory / Persediaan", c: "Payable / Hutang" }
          : subKind === "purchase-payment"
            ? { d: "Payable / Hutang", c: "Cash / Bank" }
            : subKind === "inventory-item"
              ? { d: "Inventory Asset", c: "Equity / Modal" }
              : subKind === "asset-register"
                ? { d: "Fixed Asset", c: "Cash / Bank" }
                : { d: "Debit", c: "Credit" };
  return [
    {
      id: "ln-debit",
      accountCode: "",
      accountName: "",
      description: seedText.d,
      amount: "",
      side: "debit",
    },
    {
      id: "ln-credit",
      accountCode: "",
      accountName: "",
      description: seedText.c,
      amount: "",
      side: "credit",
    },
  ];
}
