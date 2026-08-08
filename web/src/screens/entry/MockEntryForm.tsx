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
  amount: string;
}

const PARTY_LABEL: Record<EntrySubKind, string> = {
  "money-in": "Received From",
  "money-out": "Paid To",
  "cash-transfer": "Memo",
  "sales-invoice": "Customer",
  "sales-receipt": "Payer",
  "sales-quotation-entry": "Customer",
  "sales-order-entry": "Customer",
  "delivery-order-entry": "Customer",
    "credit-note-entry": "Customer",
    "purchase-order-entry": "Supplier",
    "grn-entry": "Supplier",
    "purchase-supplier-entry": "Supplier",
    "purchase-invoice": "Supplier",
    "supplier-invoice-entry": "Supplier",
    "purchase-return-entry": "Supplier",
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
  "sales-quotation-entry": "Sales Quotation",
  "sales-order-entry": "Sales Order",
  "delivery-order-entry": "Delivery Order",
    "credit-note-entry": "Credit Note",
    "purchase-order-entry": "Purchase Order",
    "grn-entry": "Goods Received Note",
    "purchase-supplier-entry": "Supplier",
    "purchase-return-entry": "Purchase Return",
    "purchase-invoice": "Purchase Invoice",
    "supplier-invoice-entry": "Supplier Invoice",
  "purchase-payment": "Purchase Payment",
  "inventory-item": "Inventory Item",
  "asset-register": "Asset",
};

const PRIMARY_ACCOUNT_HINT: Record<EntrySubKind, { label: string }> = {
  "money-in": { label: "Cash / Bank" },
  "money-out": { label: "Cash / Bank" },
  "cash-transfer": { label: "From account" },
  "sales-invoice": { label: "Receivable" },
  "sales-receipt": { label: "Cash / Bank" },
  "sales-quotation-entry": { label: "Quote lines" },
  "sales-order-entry": { label: "Order lines" },
  "delivery-order-entry": { label: "Delivery lines" },
    "credit-note-entry": { label: "Return lines" },
    "purchase-order-entry": { label: "Order lines" },
    "grn-entry": { label: "Received items" },
    "purchase-supplier-entry": { label: "Supplier info" },
    "purchase-return-entry": { label: "Return lines" },
    "purchase-invoice": { label: "Inventory / Expense" },
    "supplier-invoice-entry": { label: "Uninvoiced Payables" },
  "purchase-payment": { label: "Cash / Bank" },
  "inventory-item": { label: "Inventory Asset" },
  "asset-register": { label: "Fixed Asset" },
};

/**
 * Demo entry form stub for modules that don't yet have a real backend
 * (Sales, Purchases, Inventory, Fixed Assets). Uses the same Accurate
 * layout chrome as the cash entry form: 2-column header, search bar,
 * detail grid, right action rail, bottom total.
 */
export function MockEntryForm({ tabId, subKind, title, initialTitle }: Props) {
  const workbench = useWorkbench();
  const [date, setDate] = useState(todayISO());
  const [number, setNumber] = useState(initialTitle ?? draftNumber(subKind));
  const [autoNumber, setAutoNumber] = useState(true);
  const [party, setParty] = useState("");
  const [description, setDescription] = useState("");
  const [counterLines, setCounterLines] = useState<CounterLine[]>([seedCounterLine()]);
  const [accountSearch, setAccountSearch] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    workbench.markUnsaved(tabId, true);
  }, [tabId, date, number, party, description, counterLines, workbench]);

  const primary = PRIMARY_ACCOUNT_HINT[subKind];
  const totalCents = useMemo(
    () => counterLines.reduce((sum, line) => sum + parseCents(line.amount), 0),
    [counterLines],
  );

  const updateCounter = (id: string, patch: Partial<CounterLine>) => {
    setCounterLines((current) => current.map((line) => (line.id === id ? { ...line, ...patch } : line)));
  };
  const removeCounter = (id: string) => {
    setCounterLines((current) => (current.length > 1 ? current.filter((line) => line.id !== id) : current));
  };
  const addCounter = () => setCounterLines((current) => [...current, seedCounterLine()]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError("This module is in demo mode. Connect a backend endpoint to post entries.");
  };

  const headerLabel = HEADER_TITLE[subKind] ?? title;
  const partyLabel = PARTY_LABEL[subKind] ?? "Name";
  const status = initialTitle && !initialTitle.endsWith("DRAFT") ? "POSTED" : "DRAFT";

  return (
    <form className="entrytab entrytab--accurate" onSubmit={handleSubmit} noValidate>
      <div className="entrytab__head">
        <div className="entrytab__title">
          <span>{headerLabel}</span>
          <span className={`entrytab__status ${status === "POSTED" ? "entrytab__status--posted" : "entrytab__status--draft"}`}>
            {status}
          </span>
          <span className="entrytab__number">{number}</span>
          <span className="entrytab__date">{formatDateID(date)}</span>
          <span className="listtab__demo" title="Demo mode — no backend endpoint yet">Demo</span>
        </div>
      </div>

      <div className="entrytab__body">
        <div className="entrytab__main">
          <div className="entrytab__header-grid">
            <div className="entrytab__header-col">
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
                <span className="field__label">Date</span>
                <input className="input" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
              </label>
            </div>
            <div className="entrytab__header-col">
              <label className="field field--inline">
                <span className="field__label">No Bukti</span>
                <input
                  type="checkbox"
                  checked={autoNumber}
                  onChange={(e) => setAutoNumber(e.target.checked)}
                  aria-label="Auto-generate document number"
                />
              </label>
              <input
                className="input"
                value={number}
                onChange={(e) => setNumber(e.target.value)}
                placeholder="Document number"
              />
              <button type="button" className="btn btn--secondary btn--sm entrytab__ambil" disabled>
                <span aria-hidden="true">↗</span> Ambil
              </button>
            </div>
          </div>

          <label className="field">
            <span className="field__label">Keterangan / Description</span>
            <input
              className="input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Short description"
            />
          </label>

          <div className="entrytab__search">
            <input
              type="search"
              className="input"
              placeholder="Cari/Pilih Akun Perkiraan..."
              value={accountSearch}
              onChange={(e) => setAccountSearch(e.target.value)}
            />
            <span className="entrytab__search-icon" aria-hidden="true">🔍</span>
          </div>

          <div className="entrytab__detail">
            <div className="entrytab__detail-title">{headerLabel} detail *</div>
            <div className="detail-grid">
              <div className="detail-grid__head">
                <div>Akun</div>
                <div>Nama Akun</div>
                <div className="right">Nilai</div>
                <div aria-hidden="true" />
              </div>
              {counterLines.map((line) => (
                <div className="detail-grid__row" key={line.id}>
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
                      value={line.accountId}
                      readOnly
                      aria-readonly="true"
                      className="detail-grid__readonly"
                      placeholder="—"
                    />
                  </div>
                  <div>
                    <input
                      className="amount"
                      type="text"
                      inputMode="numeric"
                      value={formatAmountInput(line.amount)}
                      onChange={(e) => {
                        const digits = e.target.value.replace(/[^\d]/g, "").slice(0, 15);
                        updateCounter(line.id, { amount: digits });
                      }}
                      placeholder="0"
                    />
                  </div>
                  <div>
                    <button
                      type="button"
                      className="detail-grid__remove"
                      onClick={() => removeCounter(line.id)}
                      aria-label="Remove line"
                      disabled={counterLines.length === 1}
                    >
                      ×
                    </button>
                  </div>
                </div>
              ))}
              <div className="detail-grid__row detail-grid__row--add">
                <div>
                  <button type="button" className="btn btn--secondary btn--sm" onClick={addCounter}>
                    + Add line
                  </button>
                </div>
                <div />
                <div />
                <div />
              </div>
            </div>
          </div>

          <div className="entrytab__total">
            <span className="entrytab__total-label">Nilai</span>
            <span className="entrytab__total-value">{formatIDR(totalCents)}</span>
          </div>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled title="Save (demo mode)">
            <DiskIcon />
            <span>Save</span>
          </button>
          <button type="button" className="action-rail__btn action-rail__btn--secondary" disabled title="Save & New (demo mode)">
            <SavePlusIcon />
            <span>Save &amp; New</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="Duplicate this entry">
            <DocIcon />
            <span>Document</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="Attach a file">
            <AttachIcon />
            <span>Attach</span>
          </button>
          <button type="button" className="action-rail__btn" disabled title="More actions">
            <MoreIcon />
            <span>More</span>
          </button>
          <div className="action-rail__hint">
            <strong>Primary:</strong> {primary.label}
          </div>
        </aside>

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

function formatDateID(iso: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function seedCounterLine(): CounterLine {
  return {
    id: `ln-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    accountId: "",
    amount: "",
  };
}

function DiskIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="currentColor" />
      <path d="M12 7v5l3 2" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}
function SavePlusIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <rect x="3" y="4" width="18" height="16" rx="2" fill="currentColor" />
      <path d="M12 9v6m-3-3h6" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
function DocIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M6 3h9l4 4v14a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z" fill="currentColor" />
      <path d="M14 3v5h5" fill="rgba(255,255,255,0.5)" />
    </svg>
  );
}
function AttachIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M21 11l-9 9a5 5 0 0 1-7-7l9-9a3 3 0 0 1 4 4l-9 9a1 1 0 0 1-1.4-1.4l8-8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}
function MoreIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="5" cy="12" r="1.5" fill="currentColor" />
      <circle cx="12" cy="12" r="1.5" fill="currentColor" />
      <circle cx="19" cy="12" r="1.5" fill="currentColor" />
    </svg>
  );
}
