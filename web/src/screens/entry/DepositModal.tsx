import { useState } from "react";
import type { ChequeListItem } from "../../types";

interface DepositModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: () => Promise<void>;
  cheque: ChequeListItem;
}

export function DepositModal({ open, onClose, onSubmit, cheque }: DepositModalProps) {
  const [depositing, setDepositing] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
      <div className="modal modal--centered" onClick={(e) => e.stopPropagation()}>
        <div className="modal__head">
          <h3 className="modal__title">Deposit Cheque</h3>
        </div>
        <div className="modal__body">
          <p><strong>Cheque #:</strong> {cheque.cheque_number}</p>
          <p><strong>Counterparty:</strong> {cheque.counterparty_name}</p>
          <p><strong>Amount:</strong> {formatIDR(cheque.amount_cents)}</p>
          {error && <p className="modal__error">{error}</p>}
        </div>
        <div className="modal__foot">
          <button className="btn btn--ghost" onClick={onClose} disabled={depositing}>Cancel</button>
          <button className="btn btn--primary" onClick={handleDeposit} disabled={depositing}>
            {depositing ? "Processing..." : "Confirm Deposit"}
          </button>
        </div>
      </div>
    </div>
  );
}

function formatIDR(cents: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR" }).format(cents / 100);
}
