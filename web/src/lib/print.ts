/**
 * Print helper — opens a minimal, print-friendly document in a new window and
 * triggers the browser print dialog. Used by the workflow-chain "Print"
 * buttons (Quotation, DO, Invoice, PO, Supplier Invoice) since the backend
 * has no per-document PDF endpoint yet.
 */

export interface PrintColumn {
  label: string;
  right?: boolean;
}

export interface PrintOptions {
  /** Document title, e.g. "Sales Invoice INV-2026-0001". */
  title: string;
  /** Small line under the title, e.g. business name or doc status. */
  subtitle?: string;
  /** Header meta pairs, e.g. ["Customer", "PT Acme"]. */
  meta?: Array<[string, string]>;
  columns: PrintColumn[];
  /** Row cells; must match columns length. */
  rows: Array<Array<string | number>>;
  /** Footer totals, e.g. ["Total", "Rp 1.500.000"]. */
  totals?: Array<[string, string]>;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function openPrintWindow(options: PrintOptions): boolean {
  const win = window.open("", "_blank", "width=860,height=900");
  if (!win) return false;

  const metaHtml = (options.meta ?? [])
    .map(
      ([label, value]) =>
        `<div class="meta"><span class="meta-label">${escapeHtml(label)}</span><span>${escapeHtml(value)}</span></div>`,
    )
    .join("");

  const headHtml = options.columns
    .map((c) => `<th${c.right ? ' class="right"' : ""}>${escapeHtml(c.label)}</th>`)
    .join("");

  const rowsHtml = options.rows
    .map(
      (row) =>
        `<tr>${row
          .map((cell, i) => `<td${options.columns[i]?.right ? ' class="right"' : ""}>${escapeHtml(String(cell))}</td>`)
          .join("")}</tr>`,
    )
    .join("");

  const totalsHtml = (options.totals ?? [])
    .map(
      ([label, value]) =>
        `<div class="total"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`,
    )
    .join("");

  win.document.write(`<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<title>${escapeHtml(options.title)}</title>
<style>
  body { font-family: "Segoe UI", system-ui, sans-serif; color: #1c2430; margin: 32px; }
  h1 { font-size: 18px; margin: 0 0 2px; }
  .subtitle { color: #5b6572; font-size: 12px; margin-bottom: 16px; }
  .meta { display: flex; gap: 8px; font-size: 12px; margin: 2px 0; }
  .meta-label { color: #5b6572; width: 140px; }
  table { width: 100%; border-collapse: collapse; margin-top: 20px; font-size: 12px; }
  th, td { border-bottom: 1px solid #dde3ea; padding: 6px 8px; text-align: left; }
  th { background: #f4f6f9; }
  .right { text-align: right; }
  .totals { margin-top: 12px; margin-left: auto; width: 320px; font-size: 12px; }
  .total { display: flex; justify-content: space-between; padding: 3px 0; }
  @media print { body { margin: 8mm; } }
</style>
</head>
<body>
  <h1>${escapeHtml(options.title)}</h1>
  ${options.subtitle ? `<div class="subtitle">${escapeHtml(options.subtitle)}</div>` : ""}
  ${metaHtml}
  <table>
    <thead><tr>${headHtml}</tr></thead>
    <tbody>${rowsHtml}</tbody>
  </table>
  ${totalsHtml ? `<div class="totals">${totalsHtml}</div>` : ""}
  <script>window.onload = function () { window.focus(); window.print(); };</script>
</body>
</html>`);
  win.document.close();
  return true;
}
