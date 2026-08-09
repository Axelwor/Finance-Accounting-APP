import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { LeaseContract, CreateLeaseContractInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const FREQUENCIES = [
  { value: "MONTHLY", label: "Monthly" },
  { value: "QUARTERLY", label: "Quarterly" },
  { value: "ANNUALLY", label: "Annually" },
];

export function LeaseContractForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = !!entryId;

  const [lesseeName, setLesseeName] = useState("");
  const [lessorName, setLessorName] = useState("");
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10));
  const [endDate, setEndDate] = useState("");
  const [paymentAmount, setPaymentAmount] = useState("");
  const [frequency, setFrequency] = useState("MONTHLY");
  const [totalPayments, setTotalPayments] = useState("");
  const [discountRate, setDiscountRate] = useState("0.01");
  const [paymentAccountCode, setPaymentAccountCode] = useState("1101");
  const [description, setDescription] = useState("");

  const [existing, setExisting] = useState<LeaseContract | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    api.getLeaseContract(id).then((lc) => {
      setExisting(lc);
      setLesseeName(lc.lessee_name);
      setLessorName(lc.lessor_name ?? "");
      setStartDate(lc.start_date);
      setEndDate(lc.end_date);
      setPaymentAmount(String(lc.payment_amount_cents));
      setFrequency(lc.payment_frequency);
      setTotalPayments(String(lc.total_payments));
      setDiscountRate(lc.discount_rate);
    }).catch(() => {});
  }, [entryId]);

  function markDirty() {
    workbench.markUnsaved(tabId, true);
  }

  function computePVPreview(): number {
    const payment = parseInt(paymentAmount, 10);
    const n = parseInt(totalPayments, 10);
    const r = parseFloat(discountRate);
    if (!Number.isFinite(payment) || !Number.isFinite(n) || !Number.isFinite(r) || r <= 0 || n <= 0) return 0;
    return Math.round(payment * (1 - Math.pow(1 + r, -n)) / r);
  }

  async function handleSave() {
    setError("");
    const paymentCents = parseInt(paymentAmount, 10);
    const n = parseInt(totalPayments, 10);
    if (!lesseeName.trim()) { setError("Lessee name is required."); return; }
    if (!startDate) { setError("Start date is required."); return; }
    if (!endDate) { setError("End date is required."); return; }
    if (!Number.isFinite(paymentCents) || paymentCents <= 0) { setError("Payment amount must be > 0."); return; }
    if (!Number.isFinite(n) || n <= 0) { setError("Total payments must be > 0."); return; }
    const r = parseFloat(discountRate);
    if (!Number.isFinite(r) || r <= 0 || r >= 1) { setError("Discount rate must be between 0 and 1 (e.g. 0.01 for 1%)."); return; }

    setSaving(true);
    try {
      const input: CreateLeaseContractInput = {
        lessee_name: lesseeName.trim(),
        lessor_name: lessorName.trim() || undefined,
        start_date: startDate,
        end_date: endDate,
        payment_amount_cents: paymentCents,
        payment_frequency: frequency,
        total_payments: n,
        discount_rate: discountRate,
        payment_account_code: paymentAccountCode,
        description: description.trim() || undefined,
      };
      const result = await api.createLeaseContract(input);
      workbench.replaceDraft(tabId, result.number, result.status);
      workbench.openEntryExisting("lease-contract-entry", result.id, result.number, result.status);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register lease.");
    } finally {
      setSaving(false);
    }
  }

  if (isExisting && existing) {
    return <LeaseContractDetail tabId={tabId} lease={existing} />;
  }

  const pvPreview = computePVPreview();

  return (
    <div className="entrytab">
      <div className="entrytab__head">
        <div className="entrytab__title-bar">
          <span className="entrytab__title">New Lease Contract</span>
          <span className="entrytab__draft-no">{draftNumber("lease-contract-entry")}</span>
        </div>
        <small>PSAK 73 — register a lease to recognise the right-of-use asset and lease liability at present value.</small>
      </div>

      <div className="entrytab__body">
        <div className="entryform">
          <div className="entryform__row">
            <label className="entryform__field">
              <span>Lessee Name</span>
              <input type="text" value={lesseeName} onChange={(e) => { setLesseeName(e.target.value); markDirty(); }} placeholder="e.g. PT Example" />
            </label>
            <label className="entryform__field">
              <span>Lessor Name</span>
              <input type="text" value={lessorName} onChange={(e) => { setLessorName(e.target.value); markDirty(); }} placeholder="e.g. landlord / leasing company" />
            </label>
          </div>

          <div className="entryform__row">
            <label className="entryform__field">
              <span>Start Date</span>
              <input type="date" value={startDate} onChange={(e) => { setStartDate(e.target.value); markDirty(); }} />
            </label>
            <label className="entryform__field">
              <span>End Date</span>
              <input type="date" value={endDate} onChange={(e) => { setEndDate(e.target.value); markDirty(); }} />
            </label>
          </div>

          <div className="entryform__row">
            <label className="entryform__field">
              <span>Payment Amount (Rp)</span>
              <input type="number" value={paymentAmount} onChange={(e) => { setPaymentAmount(e.target.value); markDirty(); }} placeholder="e.g. 5000000" />
            </label>
            <label className="entryform__field">
              <span>Frequency</span>
              <select value={frequency} onChange={(e) => { setFrequency(e.target.value); markDirty(); }}>
                {FREQUENCIES.map((f) => <option key={f.value} value={f.value}>{f.label}</option>)}
              </select>
            </label>
          </div>

          <div className="entryform__row">
            <label className="entryform__field">
              <span>Total Payments</span>
              <input type="number" value={totalPayments} onChange={(e) => { setTotalPayments(e.target.value); markDirty(); }} placeholder="e.g. 12" />
            </label>
            <label className="entryform__field">
              <span>Discount Rate (decimal)</span>
              <input type="text" value={discountRate} onChange={(e) => { setDiscountRate(e.target.value); markDirty(); }} placeholder="e.g. 0.01 for 1%" />
            </label>
          </div>

          <div className="entryform__row">
            <label className="entryform__field">
              <span>Payment Account (Cash/Bank)</span>
              <input type="text" value={paymentAccountCode} onChange={(e) => { setPaymentAccountCode(e.target.value); markDirty(); }} placeholder="1101" />
            </label>
            <label className="entryform__field">
              <span>Description (optional)</span>
              <input type="text" value={description} onChange={(e) => { setDescription(e.target.value); markDirty(); }} placeholder="e.g. Office space lease" />
            </label>
          </div>

          {pvPreview > 0 && (
            <div className="entryform__summary">
              <span>Present Value (initial ROU / Liability):</span>
              <strong>{formatIDR(pvPreview)}</strong>
            </div>
          )}

          {error && <FormError message={error} />}

          <div className="entryform__actions">
            <button type="button" className="btn btn--primary" disabled={saving} onClick={() => void handleSave()}>
              {saving ? "Saving..." : "Register Lease"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Lease contract detail view — shows the contract + payment schedule.
// ---------------------------------------------------------------------------

function LeaseContractDetail({ tabId, lease }: { tabId: string; lease: LeaseContract }) {
  const workbench = useWorkbench();
  return (
    <div className="entrytab">
      <div className="entrytab__head">
        <div className="entrytab__title-bar">
          <span className="entrytab__title">{lease.number}</span>
          <span className="kind-mark is-positive">{lease.status}</span>
        </div>
        <small>PSAK 73 lease — {lease.lessee_name}</small>
      </div>
      <div className="entrytab__body">
        <div className="entryform">
          <div className="entryform__summary-grid">
            <div><span>Period</span><strong>{lease.start_date} → {lease.end_date}</strong></div>
            <div><span>Payment</span><strong>{formatIDR(lease.payment_amount_cents)} / {lease.payment_frequency}</strong></div>
            <div><span>Total Payments</span><strong>{lease.total_payments}</strong></div>
            <div><span>Discount Rate</span><strong>{lease.discount_rate}</strong></div>
            <div><span>Initial ROU Asset</span><strong>{formatIDR(lease.initial_rou_cents)}</strong></div>
            <div><span>Initial Liability</span><strong>{formatIDR(lease.initial_liability_cents)}</strong></div>
          </div>

          <div className="entryform__actions">
            <button type="button" className="btn btn--secondary" onClick={() => workbench.openEntryExisting("lease-payment-schedule", lease.id, `Schedule ${lease.number}`, lease.status)}>
              View Payment Schedule
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
