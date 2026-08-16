import { formatIDR } from "../../lib/format";
import type { PPNSummary } from "../../types";

/** PPN tax summary widget — keluaran, masukan, net payable. */
export function TaxSummaryWidget({ data }: { data: PPNSummary | null }) {
  const keluaran = data?.ppn_keluaran_cents ?? 0;
  const masukan = data?.ppn_masukan_cents ?? 0;
  const net = data?.net_ppn_cents ?? keluaran - masukan;
  return (
    <div className="dashboard-widget">
      <div className="dashboard-widget__head">
        <h2 className="dashboard-widget__title">PPN summary</h2>
        <span className="dashboard-widget__meta">current period</span>
      </div>
      <ul className="tax-list">
        <li className="tax-list__row">
          <span className="tax-list__label">PPN Keluaran (output)</span>
          <span className="tax-list__value">{formatIDR(keluaran)}</span>
        </li>
        <li className="tax-list__row">
          <span className="tax-list__label">PPN Masukan (input)</span>
          <span className="tax-list__value">{formatIDR(masukan)}</span>
        </li>
        <li className="tax-list__row tax-list__row--net">
          <span className="tax-list__label">Net PPN payable</span>
          <span className={`tax-list__value${net > 0 ? " is-positive" : net < 0 ? " is-negative" : ""}`}>
            {formatIDR(net)}
          </span>
        </li>
      </ul>
    </div>
  );
}
