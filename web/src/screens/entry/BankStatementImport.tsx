import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { BackendAccount } from "../../types";

interface Props {
  tabId: string;
  initialTitle?: string;
}

interface ParsedLine {
  id: string;
  tx_date: string;
  description: string;
  reference: string;
  amount_cents: number;
}

/** Parse a pasted CSV blob into statement lines. Supports comma, tab, and
 * semicolon delimiters. Expected columns (in order):
 *   tx_date, description, reference, amount
 * Amount may use a minus sign for debits (money out); the parsed amount is
 * stored signed (positive = credit / deposit, negative = debit / withdrawal). */
function parseCsv(raw: string): ParsedLine[] {
  const lines = raw
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.length > 0);
  if (lines.length === 0) return [];
  // Detect delimiter from the first line.
  const sample = lines[0];
  const delimiter = sample.includes("\t") ? "\t" : sample.includes(";") ? ";" : ",";
  const out: ParsedLine[] = [];
  let startIdx = 0;
  // Skip a header row if the first cell isn't a date.
  const firstCells = sample.split(delimiter);
  if (firstCells.length > 0 && !/^\d{4}-\d{2}-\d{2}/.test(firstCells[0].trim())) {
    startIdx = 1;
  }
  for (let i = startIdx; i < lines.length; i++) {
    const cells = lines[i].split(delimiter).map((c) => c.trim().replace(/^"|"$/g, ""));
    if (cells.length < 2) continue;
    const txDate = cells[0] ?? "";
    const description = cells[1] ?? "";
    const reference = cells[2] ?? "";
    const amountText = (cells[3] ?? "0").replace(/[^\d.\-]/g, "");
    const amount = Math.round((parseFloat(amountText) || 0) * 100);
    out.push({
      id: crypto.randomUUID(),
      tx_date: txDate,
      description,
      reference,
      amount_cents: amount,
    });
  }
  return out;
}

export function BankStatementImport({ tabId }: Props) {
  const workbench = useWorkbench();

  const [bankAccounts, setBankAccounts] = useState<BackendAccount[]>([]);
  const [bankAccountId, setBankAccountId] = useState("");
  const [statementDate, setStatementDate] = useState(new Date().toISOString().slice(0, 10));
  const [opening, setOpening] = useState("0");
  const [closing, setClosing] = useState("0");
  const [csvText, setCsvText] = useState("");
  const [parsedLines, setParsedLines] = useState<ParsedLine[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listBankAccounts().then((banks) => {
      setBankAccounts(banks);
      setBankAccountId((prev) => prev || (banks[0]?.id != null ? String(banks[0].id) : ""));
    }).catch(() => setBankAccounts([]));
  }, []);

  const openingCents = Math.round((parseFloat(opening) || 0) * 100);
  const closingCents = Math.round((parseFloat(closing) || 0) * 100);

  const linesTotal = useMemo(
    () => parsedLines.reduce((sum, l) => sum + l.amount_cents, 0),
    [parsedLines],
  );

  function handleParse() {
    setError("");
    const parsed = parseCsv(csvText);
    if (parsed.length === 0) {
      setError("No lines could be parsed. Expected: tx_date, description, reference, amount.");
      setParsedLines([]);
      return;
    }
    setParsedLines(parsed);
    workbench.markUnsaved(tabId, true);
  }

  function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!bankAccountId) {
      setError("Select a bank account.");
      return;
    }
    if (parsedLines.length === 0) {
      setError("Parse the CSV before importing.");
      return;
    }
    setSaving(true);
    api
      .importBankStatement({
        bank_account_id: parseInt(bankAccountId, 10),
        statement_date: statementDate,
        opening_balance_cents: openingCents,
        closing_balance_cents: closingCents,
        lines: parsedLines.map((l) => ({
          tx_date: l.tx_date,
          description: l.description,
          reference: l.reference,
          amount_cents: l.amount_cents,
        })),
      })
      .then((stmt) => {
        workbench.markUnsaved(tabId, false);
        workbench.openEntryExisting("bank-reconciliation-entry", stmt.id, `Reconciliation #${stmt.id}`, stmt.status);
      })
      .catch((err: unknown) => {
        const msg = err && typeof err === "object" && "message" in err ? (err as { message: string }).message : "Import failed.";
        setError(msg);
      })
      .finally(() => setSaving(false));
  }

  return (
    <form className="entrytab" onSubmit={handleSave}>
      <div className="entrytab__body">
        <header className="entrytab__head">
          <h2 className="entrytab__title">Import Bank Statement</h2>
          <p className="entrytab__hint">
            Import a bank statement (CSV) to reconcile recorded cash transactions against the bank.
          </p>
        </header>

        <div className="entrytab__row">
          <label className="field">
            <span className="field__label">Bank Account</span>
            <select value={bankAccountId} onChange={(e) => setBankAccountId(e.target.value)}>
              <option value="">Select bank account…</option>
              {bankAccounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.code} — {a.name}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span className="field__label">Statement Date</span>
            <input type="date" value={statementDate} onChange={(e) => setStatementDate(e.target.value)} required />
          </label>
        </div>

        <div className="entrytab__row">
          <label className="field">
            <span className="field__label">Opening Balance (IDR)</span>
            <input
              type="number"
              step="0.01"
              value={opening}
              onChange={(e) => setOpening(e.target.value)}
            />
          </label>
          <label className="field">
            <span className="field__label">Closing Balance (IDR)</span>
            <input
              type="number"
              step="0.01"
              value={closing}
              onChange={(e) => setClosing(e.target.value)}
            />
          </label>
        </div>

        <div className="entrytab__row entrytab__row--full">
          <label className="field">
            <span className="field__label">Paste bank statement CSV</span>
            <textarea
              rows={8}
              placeholder={"tx_date,description,reference,amount\n2026-08-01,Deposit INV-12,REF-001,1500000\n2026-08-03,Supplier payment,REF-002,-750000"}
              value={csvText}
              onChange={(e) => setCsvText(e.target.value)}
            />
          </label>
          <button type="button" className="btn btn--ghost btn--sm" onClick={handleParse}>
            Parse lines
          </button>
        </div>

        {parsedLines.length > 0 && (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <span>#</span>
              <span>Date</span>
              <span>Description</span>
              <span>Reference</span>
              <span className="right">Amount</span>
            </div>
            {parsedLines.map((l, idx) => (
              <div className="ledger-table__row" key={l.id}>
                <span>{idx + 1}</span>
                <span>{l.tx_date}</span>
                <span>{l.description}</span>
                <span>{l.reference}</span>
                <span className="ledger-table__amount right">{formatIDR(l.amount_cents)}</span>
              </div>
            ))}
            <div className="ledger-table__row ledger-table__row--foot">
              <span>Lines: {parsedLines.length}</span>
              <span>Sum of lines: {formatIDR(linesTotal)}</span>
              <span>Opening: {formatIDR(openingCents)}</span>
              <span>Expected closing: {formatIDR(openingCents + linesTotal)}</span>
              <span className="right">Closing: {formatIDR(closingCents)}</span>
            </div>
          </div>
        )}

        <FormError message={error} />
      </div>

      <aside className="action-rail" aria-label="Form actions">
        <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
          <span>{saving ? "Importing..." : "Import & Reconcile"}</span>
        </button>
      </aside>
    </form>
  );
}
