import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { LeaseContract } from "../../types";

interface Props {
  tabId: string;
  leaseId?: string | number;
  initialTitle?: string;
}

export function LeasePaymentSchedule({ tabId, leaseId }: Props) {
  const id = Number(leaseId);
  const [lease, setLease] = useState<LeaseContract | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [postingNo, setPostingNo] = useState<number | null>(null);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.getLeaseContract(id);
      setLease(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load lease schedule.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, [id]);

  async function handlePost(paymentNo: number) {
    setPostingNo(paymentNo);
    try {
      await api.postLeasePayment(id, paymentNo);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to post payment.");
    } finally {
      setPostingNo(null);
    }
  }

  if (loading) return <LoadingState label="Loading payment schedule..." />;
  if (error && !lease) return <ErrorState message={error} onRetry={() => void load()} />;
  if (!lease || !lease.schedule || lease.schedule.length === 0) {
    return <EmptyState title="No schedule" message="This lease has no payment schedule." />;
  }

  const postedCount = lease.schedule.filter((p) => p.posted).length;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Payment Schedule — {lease.number}</span>
          <small>PSAK 73 effective interest method — {postedCount}/{lease.schedule.length} posted</small>
        </div>
      </div>
      {error && <div className="listtab__error">{error}</div>}
      <div className="listtab__body">
        <div className="ledger-table">
          <div className="ledger-table__row ledger-table__row--header">
            <span>#</span>
            <span>Date</span>
            <span className="right">Payment</span>
            <span className="right">Interest</span>
            <span className="right">Principal</span>
            <span className="right">Remaining</span>
            <span></span>
          </div>
          {lease.schedule.map((p) => (
            <div key={p.payment_no} className={`ledger-table__row ${p.posted ? "is-muted" : ""}`}>
              <span className="ledger-table__num">{p.payment_no}</span>
              <span className="ledger-table__date">{p.payment_date}</span>
              <span className="ledger-table__amount right">{formatIDR(p.payment_amount_cents)}</span>
              <span className="ledger-table__amount right">{formatIDR(p.interest_cents)}</span>
              <span className="ledger-table__amount right">{formatIDR(p.principal_cents)}</span>
              <span className="ledger-table__amount right">{formatIDR(p.remaining_liability_cents)}</span>
              <span>
                {p.posted ? (
                  <span className="kind-mark is-positive">Posted</span>
                ) : (
                  <button
                    type="button"
                    className="btn btn--primary btn--sm"
                    disabled={postingNo === p.payment_no}
                    onClick={() => void handlePost(p.payment_no)}
                  >
                    {postingNo === p.payment_no ? "Posting..." : "Post"}
                  </button>
                )}
              </span>
            </div>
          ))}
        </div>
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">Initial liability: {formatIDR(lease.initial_liability_cents)}</span>
      </div>
    </div>
  );
}
