import { useEffect, useState } from "react";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR, todayISO } from "../../lib/format";
import type { SupplierPayment } from "../../types";

/**
 * SupplierPaymentPanel — the "Bayar" (pay supplier) section for a supplier
 * invoice. Designed as a standalone component so it can be dropped into
 * SupplierInvoiceForm without editing that file (avoids merge conflicts with
 * the agent building US-033).
 *
 * Props:
 *   invoiceId      — the supplier invoice ID (must be > 0).
 *   payableCents   — current payable balance on the invoice (drives the
 *                    "hide form once fully paid" affordance).
 *   invoiceStatus  — when "PAID" or "VOID" the receive form is hidden.
 */
interface Props {
  invoiceId: number;
  payableCents: number;
  invoiceStatus?: string;
}

interface CashAccountOption {
  id: number;
  name: string;
}

export function SupplierPaymentPanel({ invoiceId, payableCents, invoiceStatus }: Props) {
  const [payments, setPayments] = useState<SupplierPayment[]>([]);
  const [cashAccounts, setCashAccounts] = useState<CashAccountOption[]>([]);
  const [amount, setAmount] = useState(0);
  const [paymentDate, setPaymentDate] = useState(todayISO());
  const [cashAccountId, setCashAccountId] = useState(0);
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [posting, setPosting] = useState(false);

  // Load cash/bank accounts once.
  useEffect(() => {
    let cancelled = false;
    void api.listAccounts().then((accs) => {
      if (cancelled) return;
      const options = accs.map((a) => ({ id: Number(a.id), name: a.name }));
      setCashAccounts(options);
      if (options.length > 0) setCashAccountId(options[0].id);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  // Load payments whenever the invoice changes (or after a successful post).
  const loadPayments = async () => {
    if (!invoiceId) {
      setPayments([]);
      return;
    }
    try {
      const pmts = await api.listSupplierPayments(invoiceId);
      setPayments(pmts);
    } catch {
      setPayments([]);
    }
  };

  useEffect(() => {
    void loadPayments();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invoiceId]);

  const totalApplied = payments.reduce((sum, p) => sum + p.ap_applied_cents, 0);
  const fullyPaid = payableCents <= 0 || invoiceStatus === "PAID";
  const isVoid = invoiceStatus === "VOID";

  const handlePost = async () => {
    if (!invoiceId) return;
    setError(null);
    if (amount <= 0) {
      setError("Payment amount must be greater than zero.");
      return;
    }
    if (!cashAccountId) {
      setError("Pick a cash/bank account.");
      return;
    }
    setPosting(true);
    try {
      await api.createSupplierPayment(invoiceId, {
        cash_account_id: cashAccountId,
        amount_cents: amount,
        payment_date: paymentDate,
        description: description.trim() || undefined,
      });
      await loadPayments();
      setAmount(0);
      setDescription("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not post the payment.");
    } finally {
      setPosting(false);
    }
  };

  return (
    <div style={{ marginTop: 16, borderTop: "2px solid var(--accent)", paddingTop: 12 }}>
      <div className="entrytab__detail-title" style={{ marginBottom: 8 }}>
        Payments — Paid: <strong>{formatIDR(totalApplied)}</strong> / Payable:{" "}
        <strong>{formatIDR(payableCents)}</strong>
      </div>

      {payments.length > 0 && (
        <div className="ledger-table" style={{ marginBottom: 12 }}>
          <div className="ledger-table__head">
            <span>Number</span>
            <span>Date</span>
            <span className="right">Amount</span>
            <span className="right">AP Applied</span>
            <span>Status</span>
          </div>
          {payments.map((pmt) => (
            <div className="ledger-table__row" key={pmt.id}>
              <span className="ledger-table__no">{pmt.number}</span>
              <span className="ledger-table__date">{pmt.payment_date}</span>
              <span className="ledger-table__amount right">{formatIDR(pmt.amount_cents)}</span>
              <span className="ledger-table__amount right">{formatIDR(pmt.ap_applied_cents)}</span>
              <span>
                <span
                  className={`kind-mark ${pmt.status === "PAID" ? "is-positive" : "is-negative"}`}
                >
                  {pmt.status}
                </span>
              </span>
            </div>
          ))}
        </div>
      )}

      {!fullyPaid && !isVoid && (
        <div className="detail-grid detail-grid--quote" style={{ gridTemplateColumns: "1fr 1fr 2fr" }}>
          <div className="field">
            <span className="field__label">Amount</span>
            <input
              className="amount input"
              type="text"
              inputMode="numeric"
              value={centsInput(amount)}
              onChange={(e) => setAmount(parseCents(e.target.value))}
              placeholder="0"
            />
          </div>
          <div className="field">
            <span className="field__label">Cash/Bank</span>
            <select
              className="input"
              value={cashAccountId}
              onChange={(e) => setCashAccountId(Number(e.target.value))}
            >
              {cashAccounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <span className="field__label">Description</span>
            <input
              className="input"
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Payment reference..."
            />
          </div>
          <div className="field">
            <span className="field__label">Payment Date</span>
            <input
              className="input"
              type="date"
              value={paymentDate}
              onChange={(e) => setPaymentDate(e.target.value)}
            />
          </div>
          <div />
          <div style={{ display: "flex", alignItems: "flex-end" }}>
            <button
              type="button"
              className="btn btn--primary btn--sm"
              onClick={() => void handlePost()}
              disabled={posting}
            >
              {posting ? "Posting..." : "Pay Supplier"}
            </button>
          </div>
        </div>
      )}
      <FormError message={error} />
    </div>
  );
}

function parseCents(raw: string): number {
  const digits = (raw || "").replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) : 0;
}

function centsInput(cents: number): string {
  if (!cents) return "";
  return new Intl.NumberFormat("en-US").format(cents);
}
