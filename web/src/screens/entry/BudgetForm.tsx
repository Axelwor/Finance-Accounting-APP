import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { Button, EmptyState, ErrorState, FieldShell, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { BackendAccount, Budget, CreateBudgetInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

interface BudgetLineDraft {
  id: string;
  accountId: number;
  month: number;
  amount: string;
}

const MONTH_LABELS = [
  "Jan", "Feb", "Mar", "Apr", "May", "Jun",
  "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

function uid(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

/**
 * Budget form (US-093). Creates a budget with monthly lines per account.
 *
 * A budget is a plan for a fiscal year, with one line per (account, month).
 * The form loads revenue/expense accounts so the user can fill in monthly
 * planned amounts. Lines are optional; empty rows are skipped on submit.
 */
export function BudgetForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null && entryId !== "";
  const [name, setName] = useState("");
  const [fiscalYear, setFiscalYear] = useState(String(new Date().getFullYear()));
  const [lines, setLines] = useState<BudgetLineDraft[]>([]);
  const [accounts, setAccounts] = useState<BackendAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [budget, setBudget] = useState<Budget | null>(null);

  const draftNo = useMemo(() => draftNumber("budget-entry"), []);
  const title = isEdit
    ? `Edit Budget · ${initialTitle ?? name ?? ""}`
    : initialTitle ?? `BUD-${draftNo}`;

  // Budgets are editable only while DRAFT; approved/closed budgets are locked.
  const isLocked = isEdit && budget !== null && budget.status !== "DRAFT";

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entryId]);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const accs = await api.listBudgetAccounts();
      accs.sort((a: BackendAccount, b: BackendAccount) => a.code.localeCompare(b.code));
      setAccounts(accs);
      if (isEdit) {
        // Load the existing budget into editable state.
        const existing = await api.getBudget(Number(entryId));
        setBudget(existing);
        setName(existing.name);
        setFiscalYear(String(existing.fiscal_year));
        setLines(
          (existing.lines ?? []).map((line) => ({
            id: uid(),
            accountId: line.account_id,
            month: line.month,
            amount: String(Math.round(line.amount_cents / 100)),
          })),
        );
        workbench.markUnsaved(tabId, false);
      } else if (accs.length > 0) {
        // Seed one empty line for a fresh budget.
        setLines((current) =>
          current.length === 0
            ? [{ id: uid(), accountId: accs[0].id, month: 1, amount: "" }]
            : current,
        );
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load budget.");
    } finally {
      setLoading(false);
    }
  };

  const totalCents = useMemo(() => {
    let total = 0;
    for (const line of lines) {
      const clean = line.amount.replace(/[^\d]/g, "");
      if (clean) total += parseInt(clean, 10) * 100;
    }
    return total;
  }, [lines]);

  const accountName = (accountId: number): string => {
    const acc = accounts.find((a) => a.id === accountId);
    return acc ? `${acc.code} · ${acc.name}` : `Account ${accountId}`;
  };

  const addLine = () => {
    setLines((prev) => [
      ...prev,
      { id: uid(), accountId: accounts[0]?.id ?? 0, month: 1, amount: "" },
    ]);
  };

  const removeLine = (id: string) => {
    setLines((prev) => prev.filter((l) => l.id !== id));
  };

  const updateLine = (id: string, patch: Partial<BudgetLineDraft>) => {
    setLines((prev) => prev.map((l) => (l.id === id ? { ...l, ...patch } : l)));
  };

  const handleSubmit = async () => {
    setError(null);
    setSuccess(null);
    if (!name.trim()) {
      setError("Budget name is required.");
      return;
    }
    const year = Number(fiscalYear);
    if (!year || year < 2000) {
      setError("A valid fiscal year is required.");
      return;
    }
    // Filter to non-empty amounts and map to cents.
    const payloadLines = lines
      .filter((l) => l.accountId > 0 && l.month >= 1 && l.month <= 12 && l.amount.replace(/[^\d]/g, ""))
      .map((l) => {
        const clean = l.amount.replace(/[^\d]/g, "");
        return {
          account_id: l.accountId,
          month: l.month,
          amount_cents: parseInt(clean, 10) * 100,
        };
      });
    if (payloadLines.length === 0) {
      setError("Add at least one budget line with an amount.");
      return;
    }
    const payload: CreateBudgetInput = {
      name: name.trim(),
      fiscal_year: year,
      lines: payloadLines,
    };
    setSubmitting(true);
    try {
      if (isEdit) {
        const updated = await api.updateBudget(Number(entryId), payload);
        setBudget(updated);
        setSuccess(`Budget "${updated.name}" updated (${updated.lines?.length ?? 0} lines).`);
        workbench.replaceDraft(tabId, updated.name, updated.status, updated.id);
      } else {
        const created = await api.createBudget(payload);
        setBudget(created);
        setSuccess(`Budget "${created.name}" created (${created.lines?.length ?? 0} lines).`);
        workbench.replaceDraft(tabId, created.name, created.status, created.id);
      }
      workbench.markUnsaved(tabId, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save budget.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{title}</span>
          <small>Budget · monthly plan per account</small>
        </div>
        <div className="listtab__toolbar">
          {budget ? (
            <span className={`kind-mark ${budget.status === "DRAFT" ? "is-positive" : "is-negative"}`}>
              {budget.status}
            </span>
          ) : null}
          {isLocked ? (
            <span style={{ color: "var(--md-sys-color-error)" }}>Locked — only DRAFT budgets can be edited.</span>
          ) : null}
          {success ? <span style={{ color: "var(--md-sys-color-success)" }}>{success}</span> : null}
        </div>
      </div>

      <div className="listtab__body" style={{ paddingBottom: 64 }}>
        {loading ? (
          <LoadingState label="Loading accounts..." />
        ) : error && accounts.length === 0 ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : accounts.length === 0 ? (
          <EmptyState
            title="No revenue/expense accounts"
            message="Create accounts in the Chart of Accounts before planning a budget."
          />
        ) : (
          <>
            <div className="detail-grid" style={{ gridTemplateColumns: "1fr 1fr", gap: 16 }}>
              <FieldShell label="Budget Name" htmlFor="budget-name">
                <input
                  id="budget-name"
                  className="field__input"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Annual Operating Budget 2026"
                  disabled={isLocked}
                />
              </FieldShell>
              <FieldShell label="Fiscal Year" htmlFor="budget-year">
                <input
                  id="budget-year"
                  className="field__input"
                  inputMode="numeric"
                  value={fiscalYear}
                  onChange={(e) => setFiscalYear(e.target.value.replace(/[^\d]/g, ""))}
                  disabled={isLocked}
                />
              </FieldShell>
            </div>

            <div style={{ marginTop: 24 }}>
              <div className="detail-grid" style={{ gridTemplateColumns: "1fr", gap: 12 }}>
                <div className="listtab__title">
                  <span>Budget Lines</span>
                  <small>One row per (account, month)</small>
                </div>
              </div>

              <div style={{ overflowX: "auto", marginTop: 12 }}>
                <table className="data-table" style={{ width: "100%", borderCollapse: "collapse" }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: "left", padding: "8px 12px" }}>Account</th>
                      <th style={{ textAlign: "left", padding: "8px 12px" }}>Month</th>
                      <th style={{ textAlign: "right", padding: "8px 12px" }}>Amount (IDR)</th>
                      <th style={{ padding: "8px 12px" }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {lines.map((line) => (
                      <tr key={line.id} style={{ borderBottom: "1px solid var(--md-sys-color-outline-variant)" }}>
                        <td style={{ padding: "8px 12px" }}>
                          <select
                            className="field__input"
                            value={line.accountId}
                            onChange={(e) => updateLine(line.id, { accountId: Number(e.target.value) })}
                            style={{ minWidth: 220 }}
                            disabled={isLocked}
                          >
                            {accounts.map((acc) => (
                              <option key={acc.id} value={acc.id}>
                                {acc.code} · {acc.name}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td style={{ padding: "8px 12px" }}>
                          <select
                            className="field__input"
                            value={line.month}
                            onChange={(e) => updateLine(line.id, { month: Number(e.target.value) })}
                            disabled={isLocked}
                          >
                            {MONTH_LABELS.map((label, idx) => (
                              <option key={idx} value={idx + 1}>
                                {label}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td style={{ padding: "8px 12px", textAlign: "right" }}>
                          <input
                            className="field__input"
                            inputMode="numeric"
                            value={line.amount}
                            onChange={(e) => updateLine(line.id, { amount: e.target.value.replace(/[^\d]/g, "") })}
                            placeholder="0"
                            style={{ textAlign: "right", maxWidth: 160 }}
                            disabled={isLocked}
                          />
                        </td>
                        <td style={{ padding: "8px 12px" }}>
                          <Button variant="ghost" onClick={() => removeLine(line.id)} disabled={isLocked}>
                            Remove
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot>
                    <tr style={{ borderTop: "2px solid var(--md-sys-color-outline-variant)" }}>
                      <td colSpan={2} style={{ padding: "8px 12px", fontWeight: 600 }}>
                        Total
                      </td>
                      <td style={{ padding: "8px 12px", textAlign: "right", fontWeight: 600 }}>
                        {formatIDR(totalCents)}
                      </td>
                      <td></td>
                    </tr>
                  </tfoot>
                </table>
              </div>

              <div style={{ marginTop: 12, display: "flex", gap: 8 }}>
                <Button variant="secondary" onClick={addLine} disabled={isLocked}>
                  + Add Line
                </Button>
              </div>
            </div>

            {error ? (
              <p style={{ color: "var(--md-sys-color-error)", marginTop: 16 }}>{error}</p>
            ) : null}
          </>
        )}
      </div>

      <div className="listtab__footer">
        <span className="listtab__footer-count">{lines.length} Line(s)</span>
        <div style={{ marginLeft: "auto", display: "flex", gap: 8 }}>
          <Button variant="secondary" onClick={() => workbench.close(tabId)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={() => void handleSubmit()} disabled={submitting || loading || isLocked}>
            {submitting ? "Saving..." : isEdit ? "Save Changes" : "Save Budget"}
          </Button>
        </div>
      </div>
    </div>
  );
}
