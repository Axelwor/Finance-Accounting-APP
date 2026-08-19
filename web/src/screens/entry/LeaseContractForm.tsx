import { useCallback, useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { NextStepsBar } from "../../components/NextSteps";
import { useToast } from "../../components/Toast";
import { api } from "../../api";
import { formatIDR, todayISO } from "../../lib/format";
import { draftNumber } from "../../workbench/modules";
import type { LeaseContract, CreateLeaseContractInput } from "../../types";
import { Button } from "../../components/m3";

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
  const toast = useToast();
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
      toast.success(`✓ Registered lease ${result.number}`);
      workbench.openEntryExisting("lease-contract-entry", result.id, result.number, result.status);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register lease.");
    } finally {
      setSaving(false);
    }
  }

  /** Re-fetch the contract after a payment / modification / termination. */
  const reloadLease = useCallback(async (id: number) => {
    try {
      const lc = await api.getLeaseContract(id);
      setExisting(lc);
    } catch {
      // keep the last known copy if the refresh fails
    }
  }, []);

  if (isExisting && existing) {
    return <LeaseContractDetail tabId={tabId} lease={existing} onReload={() => void reloadLease(existing.id)} />;
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
            <Button
              variant="filled"
              disabled={saving}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving..." : "Register Lease"}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Lease contract detail view — workflow chain for a saved contract:
// View Schedule / Post Payment / Modify / Terminate / Close.
// ---------------------------------------------------------------------------

function LeaseContractDetail({
  tabId,
  lease,
  onReload,
}: {
  tabId: string;
  lease: LeaseContract;
  onReload: () => void;
}) {
  const workbench = useWorkbench();
  const toast = useToast();
  const [postingPayment, setPostingPayment] = useState(false);
  const [showModify, setShowModify] = useState(false);
  const [modPayment, setModPayment] = useState(String(lease.payment_amount_cents));
  const [modTotalPayments, setModTotalPayments] = useState(String(lease.total_payments));
  const [modEffectiveDate, setModEffectiveDate] = useState(todayISO());
  const [modError, setModError] = useState("");
  const [postingModify, setPostingModify] = useState(false);
  const [terminating, setTerminating] = useState(false);

  const isActive = lease.status === "ACTIVE";
  const nextPayment = lease.schedule?.find((p) => !p.posted);

  const handlePostPayment = async () => {
    if (!nextPayment) return;
    setPostingPayment(true);
    try {
      const result = await api.postLeasePayment(lease.id, nextPayment.payment_no);
      toast.success(`✓ Payment #${result.payment_no} posted — principal ${formatIDR(result.principal_cents)}, interest ${formatIDR(result.interest_cents)}`);
      onReload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not post the lease payment.");
    } finally {
      setPostingPayment(false);
    }
  };

  const handleModify = async () => {
    setModError("");
    const paymentCents = parseInt(modPayment, 10);
    const n = parseInt(modTotalPayments, 10);
    if (!Number.isFinite(paymentCents) || paymentCents <= 0) {
      setModError("New payment amount must be > 0.");
      return;
    }
    if (!Number.isFinite(n) || n <= 0) {
      setModError("New total payments must be > 0.");
      return;
    }
    if (!modEffectiveDate) {
      setModError("Effective date is required.");
      return;
    }
    setPostingModify(true);
    try {
      const result = await api.modifyLeaseContract(lease.id, {
        new_payment_amount_cents: paymentCents,
        new_total_payments: n,
        effective_date: modEffectiveDate,
      });
      toast.success(`Lease modified — new PV ${formatIDR(result.new_pv_cents)} (adjustment ${formatIDR(Math.abs(result.delta_cents))})`);
      setShowModify(false);
      onReload();
    } catch (err) {
      setModError(err instanceof Error ? err.message : "Could not modify the lease.");
    } finally {
      setPostingModify(false);
    }
  };

  const handleTerminate = async () => {
    if (!window.confirm(`Terminate lease ${lease.number}? This derecognises the RoU asset and liability.`)) return;
    setTerminating(true);
    try {
      await api.terminateLeaseContract(lease.id, { termination_date: todayISO() });
      toast.success(`Lease ${lease.number} terminated`);
      onReload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not terminate the lease.");
    } finally {
      setTerminating(false);
    }
  };

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

          <NextStepsBar number={lease.number} hint={lease.status}>
            <button
              type="button"
              className="next-steps__btn"
              onClick={() => workbench.openEntryExisting("lease-payment-schedule", lease.id, `Schedule ${lease.number}`, lease.status)}
            >
              View Schedule
            </button>
            {isActive && (
              <>
                <button
                  type="button"
                  className="next-steps__btn next-steps__btn--primary"
                  onClick={() => void handlePostPayment()}
                  disabled={postingPayment || !nextPayment}
                  title={nextPayment ? `Post payment #${nextPayment.payment_no} (${nextPayment.payment_date})` : "All payments posted"}
                >
                  {postingPayment ? "Posting..." : nextPayment ? `Post Payment #${nextPayment.payment_no}` : "Post Payment"}
                </button>
                <button type="button" className="next-steps__btn" onClick={() => setShowModify((v) => !v)}>
                  Modify
                </button>
                <button type="button" className="next-steps__btn next-steps__btn--danger" onClick={() => void handleTerminate()} disabled={terminating}>
                  {terminating ? "Terminating..." : "Terminate"}
                </button>
              </>
            )}
            <button type="button" className="next-steps__btn" onClick={() => workbench.close(tabId)}>
              Close
            </button>
          </NextStepsBar>

          {isActive && showModify && (
            <div style={{ marginTop: 12 }}>
              <div className="entrytab__detail-title">Lease modification (re-measurement)</div>
              <div className="entryform__row">
                <label className="entryform__field">
                  <span>New Payment Amount (Rp)</span>
                  <input type="number" value={modPayment} onChange={(e) => { setModPayment(e.target.value); setModError(""); }} />
                </label>
                <label className="entryform__field">
                  <span>New Total Payments</span>
                  <input type="number" value={modTotalPayments} onChange={(e) => { setModTotalPayments(e.target.value); setModError(""); }} />
                </label>
                <label className="entryform__field">
                  <span>Effective Date</span>
                  <input type="date" value={modEffectiveDate} onChange={(e) => { setModEffectiveDate(e.target.value); setModError(""); }} />
                </label>
              </div>
              <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
                <Button
                  variant="filled"
                  onClick={() => void handleModify()}
                  disabled={postingModify}
                >
                  {postingModify ? "Posting..." : "Post Modification"}
                </Button>
                <Button variant="outlined" onClick={() => setShowModify(false)}>
                  Cancel
                </Button>
              </div>
              <FormError message={modError} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
