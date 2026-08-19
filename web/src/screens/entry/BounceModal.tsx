import { useState } from "react";
import type { ChequeListItem } from "../../types";
import { Button } from "../../components/m3";

interface BounceModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => Promise<void>;
  cheque: ChequeListItem;
}

export function BounceModal({ open, onClose, onSubmit, cheque }: BounceModalProps) {
  const [bouncing, setBouncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  if (!open) return null;

  const handleBounce = async () => {
    if (!reason.trim()) {
      setError("Please provide a reason for bouncing the cheque.");
      return;
    }
    setBouncing(true);
    setError(null);
    try {
      await onSubmit(reason.trim());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to bounce cheque.");
    } finally {
      setBouncing(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal modal--centered" onClick={(e) => e.stopPropagation()}>
        <div className="modal__head">
          <h3 className="modal__title">Cheque Bounced</h3>
        </div>
        <div className="modal__body">
          <p><strong>Cheque #:</strong> {cheque.cheque_number}</p>
          <p><strong>Counterparty:</strong> {cheque.counterparty_name}</p>
          <p><strong>Amount:</strong> {formatIDR(cheque.amount_cents)}</p>
          <label className="form__field" style={{ marginTop: "var(--md-sys-spacing-4)" }}>
            <span className="form__label">Reason *</span>
            <textarea
              className="form__input form__input--large"
              rows={3}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Enter reason for bounce..."
            />
          </label>
          {error && <p className="modal__error">{error}</p>}
        </div>
        <div className="modal__foot">
          <Button
            variant="text"
            onClick={onClose}
            disabled={bouncing}
          >Cancel</Button>
          <Button
            variant="outlined"
            danger
            onClick={handleBounce}
            disabled={bouncing || !reason.trim()}
          >
            {bouncing ? "Processing..." : "Confirm Bounce"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function formatIDR(cents: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR" }).format(cents / 100);
}
