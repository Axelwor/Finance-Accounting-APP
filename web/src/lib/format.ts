/** Formatting utilities for the UI (English, IDR currency). */

export function formatIDR(value: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value);
}

/** Format "June 15, 2026". */
export function formatDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  if (!year || !month || !day) return date;
  const d = new Date(year, month - 1, day);
  return new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long", year: "numeric" }).format(d);
}

/** Today's date in yyyy-mm-dd format (local). */
export function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/** "Today" / "Yesterday" / "Jun 12, 2026" for the transaction list. */
export function formatRelativeDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  if (!year || !month || !day) return date;
  const d = new Date(year, month - 1, day);
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const diff = Math.round((startOfToday.getTime() - d.getTime()) / 86400000);
  if (diff === 0) return "Today";
  if (diff === 1) return "Yesterday";
  return new Intl.DateTimeFormat("en-US", { day: "numeric", month: "short", year: "numeric" }).format(d);
}

/** Parse numeric form input into a whole amount (0 when empty). */
export function parseAmountInput(text: string): number {
  const clean = text.replace(/[^\d]/g, "");
  if (!clean) return 0;
  return parseInt(clean, 10);
}

/**
 * Parse a rupiah amount typed by the user ("12.500", "Rp 1,250,000") into
 * integer cents (backend stores every money value as *_cents, so rupiah is
 * multiplied by 100). Empty or zero input yields 0.
 */
export function parseRupiahToCents(input: string): number {
  const clean = (input || "").replace(/[^\d]/g, "");
  if (!clean) return 0;
  return parseInt(clean, 10) * 100;
}

/**
 * Format an integer-cents value as whole-rupiah IDR, consistent with
 * formatIDR styling (same Intl config, divided by 100 first).
 */
export function formatIDRFromCents(cents: number): string {
  return formatIDR(Math.round((cents || 0) / 100));
}

/** Alias for formatIDR — used by recurring/cost-center modules. */
export function fmtCurrencyIDR(value: number): string {
  return formatIDR(value);
}

/** Alias for formatDate — used by recurring module (IDR locale label). */
export function fmtDateIDR(date: string): string {
  return formatDate(date);
}

/** Parse a yyyy-mm-dd date input into a Date, or null when empty/invalid. */
export function parseDateInput(text: string): Date | null {
  if (!text) return null;
  const [year, month, day] = text.split("-").map(Number);
  if (!year || !month || !day) return null;
  return new Date(year, month - 1, day);
}
