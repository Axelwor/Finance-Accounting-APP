import { useId, useRef, useState } from "react";
import type { ChequeListItem } from "../../types";
import { Button } from "../../components/m3";
import { useDialogA11y } from "../../components/dialogA11y";

interface DepositModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: () => Promise<void>;
  cheque: ChequeListItem;
}

export function DepositModal({ open, onClose, onSubmit, cheque }: DepositModalProps) {
  const [depositing, setDepositing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const titleId = useId();

  // Focus trap + Escape + focus restore (no-ops while closed).
  useDialogA11y(open, onClose, dialogRef);

  if (!open) return null;

  const handleDeposit = async () => {
    setDepositing(true);
    setError(null);
    try {
      await onSubmit();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to deposit cheque.");
    } finally {
      setDepositing(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        ref={dialogRef}
        className="modal modal--centered"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal__head">
          <h3 className="modal__title" id={titleId}>Deposit Cheque</h3>
        </div>
        <div className="modal__body">
          <p><strong>Cheque #:</strong> {cheque.cheque_number}</p>
          <p><strong>Counterparty:</strong> {cheque.counterparty_name}</p>
          <p><strong>Amount:</strong> {formatIDR(cheque.amount_cents)}</p>
          {error && <p className="modal__error">{error}</p>}
        </div>
        <div className="modal__foot">
          <Button
            variant="text"
            onClick={onClose}
            disabled={depositing}
          >Cancel</Button>
          <Button
            variant="filled"
            onClick={handleDeposit}
            disabled={depositing}
          >
            {depositing ? "Processing..." : "Confirm Deposit"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function formatIDR(cents: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR" }).format(cents / 100);
}
