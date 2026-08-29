/**
 * Formatting utilities for the UI.
 *
 * SET-001: the formatters are tenant-configurable via configureFormatters()
 * (currency symbol, decimal places, separators, date format) loaded from
 * GET /settings right after login/tenant switch. Until configured they keep
 * the historical IDR behaviour so every existing call site is unchanged.
 */

interface FormatterConfig {
  currencyCode: string;
  symbol: string;
  amountDecimalPlaces: number;
  thousandSeparator: string;
  decimalSeparator: string;
  dateFormat: "DD/MM/YYYY" | "MM/DD/YYYY" | "YYYY-MM-DD";
}

const defaultConfig: FormatterConfig = {
  currencyCode: "IDR",
  symbol: "Rp",
  amountDecimalPlaces: 0,
  thousandSeparator: ".",
  decimalSeparator: ",",
  dateFormat: "DD/MM/YYYY",
};

let config: FormatterConfig = defaultConfig;

/** Apply tenant formatting settings (idempotent; defaults when partial). */
export function configureFormatters(partial: Partial<FormatterConfig>): void {
  config = { ...defaultConfig, ...partial };
}

/** Reset to the default IDR formatting (used by tests and logout). */
export function resetFormatters(): void {
  config = defaultConfig;
}

/** Current base currency code (tenant base currency). */
export function currentCurrencyCode(): string {
  return config.currencyCode;
}

export function formatIDR(value: number): string {
  return formatCurrencyAmount(value);
}

/** Format a whole-amount value with the configured currency symbol. */
export function formatCurrencyAmount(value: number): string {
  const negative = value < 0;
  const abs = Math.abs(value);
  const fixed = abs.toFixed(config.amountDecimalPlaces);
  const [intPart, decPart] = fixed.split(".");
  const grouped = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, config.thousandSeparator);
  let out = config.symbol + " " + grouped;
  if (decPart) out += config.decimalSeparator + decPart;
  return negative ? "-" + out : out;
}

/** Format a YYYY-MM-DD date per the tenant date format. */
export function formatDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  if (!year || !month || !day) return date;
  const mm = String(month).padStart(2, "0");
  const dd = String(day).padStart(2, "0");
  switch (config.dateFormat) {
    case "MM/DD/YYYY":
      return `${mm}/${dd}/${year}`;
    case "YYYY-MM-DD":
      return `${year}-${mm}-${dd}`;
    case "DD/MM/YYYY":
    default:
      return `${dd}/${mm}/${year}`;
  }
}

/** Today's date in yyyy-mm-dd format (local). */
export function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/** "Today" / "Yesterday" / short date for the transaction list. */
export function formatRelativeDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  if (!year || !month || !day) return date;
  const d = new Date(year, month - 1, day);
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const diff = Math.round((startOfToday.getTime() - d.getTime()) / 86400000);
  if (diff === 0) return "Today";
  if (diff === 1) return "Yesterday";
  return formatDate(date);
}

/** Parse numeric form input into a whole amount (0 when empty). */
export function parseAmountInput(text: string): number {
  const clean = text.replace(/[^\d]/g, "");
  if (!clean) return 0;
  return parseInt(clean, 10);
}

/**
 * Parse an amount typed by the user into integer cents (backend stores every
 * money value as *_cents). Empty or zero input yields 0.
 */
export function parseRupiahToCents(input: string): number {
  const clean = (input || "").replace(/[^\d]/g, "");
  if (!clean) return 0;
  return parseInt(clean, 10) * 100;
}

/**
 * Format an integer-cents value with the configured currency, consistent
 * with formatIDR styling (divided by 100 first).
 */
export function formatIDRFromCents(cents: number): string {
  return formatIDR(Math.round((cents || 0) / 100));
}

/** Alias for formatIDR — used by recurring/cost-center modules. */
export function fmtCurrencyIDR(value: number): string {
  return formatIDR(value);
}

/** Alias for formatDate — used by recurring module. */
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
