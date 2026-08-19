import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { ChequeListItem } from "../../types";
import { DepositModal } from "../entry/DepositModal";
import { BounceModal } from "../entry/BounceModal";
import { Button } from "../../components/m3";

type DirectionFilter = "RECEIVED" | "ISSUED" | "";

export function ChequeList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<ChequeListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [directionFilter, setDirectionFilter] = useState<DirectionFilter>("");
  const [depositModalOpen, setDepositModalOpen] = useState(false);
  const [bounceModalOpen, setBounceModalOpen] = useState(false);
  const [selectedCheque, setSelectedCheque] = useState<ChequeListItem | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listCheques({
        direction: directionFilter === "" ? undefined : directionFilter,
      });
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cheques.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [directionFilter]);

  const handleDeposit = (cheque: ChequeListItem) => {
    setSelectedCheque(cheque);
    setDepositModalOpen(true);
  };

  const handleBounce = (cheque: ChequeListItem) => {
    setSelectedCheque(cheque);
    setBounceModalOpen(true);
  };

  const handlePostDeposit = async () => {
    if (!selectedCheque) return;
    try {
      await api.depositCheque(selectedCheque.id.toString());
      await load();
      setDepositModalOpen(false);
      setSelectedCheque(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to deposit cheque.");
    }
  };

  const handlePostBounce = async (reason: string) => {
    if (!selectedCheque) return;
    try {
      await api.bounceCheque(selectedCheque.id.toString(), reason);
      await load();
      setBounceModalOpen(false);
      setSelectedCheque(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to bounce cheque.");
    }
  };

  const filteredItems = directionFilter
    ? items.filter((it) => it.type === directionFilter)
    : items;

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "CLEARED":
        return <span className="status-badge status-badge--success">CLEARED</span>;
      case "DEPOSITED":
        return <span className="status-badge status-badge--warning">DEPOSITED</span>;
      case "BOUNCED":
        return <span className="status-badge status-badge--danger">BOUNCED</span>;
      default:
        return <span className="status-badge status-badge--info">REGISTERED</span>;
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Cheques & GIRO</span>
          <small>Track received and issued cheques through their lifecycle.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Direction</span>
            <select
              className="filter-pill__input"
              value={directionFilter}
              onChange={(e) => setDirectionFilter(e.target.value as DirectionFilter)}
            >
              <option value="">All</option>
              <option value="RECEIVED">Received</option>
              <option value="ISSUED">Issued</option>
            </select>
          </label>
        </div>
        <Button variant="filled" onClick={() => workbench.openEntryDraft("cheque-entry")}>
          + New Cheque
        </Button>
      </div>

      <div className="listtab__body">
        {loading && <LoadingState />}
        {error && <ErrorState message={error} onRetry={load} />}
        {!loading && !error && filteredItems.length === 0 && (
          <EmptyState title="No cheques found" message="Use the +New Cheque button to register a new cheque." />
        )}
        {!loading && !error && filteredItems.length > 0 && (
          <div className="ledger-table">
            <div className="ledger-table__row ledger-table__row--head">
              <div className="ledger-table__cell">Date</div>
              <div className="ledger-table__cell">Cheque #</div>
              <div className="ledger-table__cell">Type</div>
              <div className="ledger-table__cell">Counterparty</div>
              <div className="ledger-table__cell amount">Amount</div>
              <div className="ledger-table__cell">Status</div>
              <div className="ledger-table__cell">Actions</div>
            </div>
            {filteredItems.map((it) => (
              <div key={it.id} className="ledger-table__row">
                <div className="ledger-table__cell">{new Date(it.date).toLocaleDateString()}</div>
                <div className="ledger-table__cell">{it.cheque_number}</div>
                <div className="ledger-table__cell">{it.type}</div>
                <div className="ledger-table__cell">{it.counterparty_name}</div>
                <div className="ledger-table__cell amount right">{formatIDR(it.amount_cents)}</div>
                <div className="ledger-table__cell">{getStatusBadge(it.status)}</div>
                <div className="ledger-table__cell">
                  {it.status === "REGISTERED" && (
                    <Button
                      variant="text"
                      size="sm"
                      onClick={() => handleDeposit(it)}
                    >Deposit</Button>
                  )}
                  {it.status === "DEPOSITED" && (
                    <>
                      <Button
                        variant="text"
                        size="sm"
                        onClick={() => handleBounce(it)}
                      >Bounce</Button>
                    </>
                  )}
                  {it.status === "BOUNCED" && (
                    <Button
                      variant="text"
                      size="sm"
                      onClick={() => workbench.openEntryExisting("cheque-entry", it.id.toString(), it.cheque_number)}
                    >View</Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {selectedCheque && (
        <DepositModal
          open={depositModalOpen}
          onClose={() => setDepositModalOpen(false)}
          onSubmit={handlePostDeposit}
          cheque={selectedCheque}
        />
      )}
      {selectedCheque && (
        <BounceModal
          open={bounceModalOpen}
          onClose={() => setBounceModalOpen(false)}
          onSubmit={handlePostBounce}
          cheque={selectedCheque}
        />
      )}
    </div>
  );
}
