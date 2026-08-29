import { afterEach, describe, expect, it } from "vitest";
import {
  configureFormatters,
  resetFormatters,
  formatIDR,
  formatIDRFromCents,
  formatDate,
  todayISO,
  formatRelativeDate,
  parseAmountInput,
  parseRupiahToCents,
  fmtCurrencyIDR,
  fmtDateIDR,
  parseDateInput,
} from "./format";

// Default (unconfigured) behaviour matches the historical IDR formatting:
// "Rp" prefix, dot thousand separator, no decimals, DD/MM/YYYY dates.
afterEach(() => {
  resetFormatters();
});

describe("formatIDR (default)", () => {
  it("formats whole rupiah with the Rp prefix and dot grouping", () => {
    expect(formatIDR(1500000)).toBe("Rp 1.500.000");
  });

  it("drops decimal fraction digits", () => {
    expect(formatIDR(1234.56)).toBe("Rp 1.235");
  });

  it("formats zero", () => {
    expect(formatIDR(0)).toBe("Rp 0");
  });

  it("formats negative amounts", () => {
    expect(formatIDR(-25000)).toBe("-Rp 25.000");
  });
});

describe("configureFormatters", () => {
  it("applies tenant currency symbol and separators", () => {
    configureFormatters({
      currencyCode: "USD",
      symbol: "$",
      amountDecimalPlaces: 2,
      thousandSeparator: ",",
      decimalSeparator: ".",
    });
    expect(formatIDR(1234.5)).toBe("$ 1,234.50");
  });

  it("applies the tenant date format", () => {
    configureFormatters({ dateFormat: "MM/DD/YYYY" });
    expect(formatDate("2026-06-15")).toBe("06/15/2026");
    configureFormatters({ dateFormat: "YYYY-MM-DD" });
    expect(formatDate("2026-06-15")).toBe("2026-06-15");
  });

  it("resets back to the IDR default", () => {
    configureFormatters({ symbol: "$", amountDecimalPlaces: 2 });
    resetFormatters();
    expect(formatIDR(1500)).toBe("Rp 1.500");
  });
});

describe("fmtCurrencyIDR", () => {
  it("is an alias of formatIDR", () => {
    expect(fmtCurrencyIDR(999000)).toBe(formatIDR(999000));
  });
});

describe("formatDate (default DD/MM/YYYY)", () => {
  it("formats an ISO date as DD/MM/YYYY", () => {
    expect(formatDate("2026-06-15")).toBe("15/06/2026");
  });

  it("returns the input unchanged when it is not a valid ISO date", () => {
    expect(formatDate("not-a-date")).toBe("not-a-date");
  });

  it("returns the input unchanged for empty string", () => {
    expect(formatDate("")).toBe("");
  });
});

describe("fmtDateIDR", () => {
  it("is an alias of formatDate", () => {
    expect(fmtDateIDR("2026-01-02")).toBe("02/01/2026");
  });
});

describe("todayISO", () => {
  it("returns a yyyy-mm-dd string for the local date", () => {
    const iso = todayISO();
    expect(iso).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("matches the local calendar date components", () => {
    const now = new Date();
    const expected = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(
      now.getDate(),
    ).padStart(2, "0")}`;
    expect(todayISO()).toBe(expected);
  });
});

describe("formatRelativeDate", () => {
  it("returns 'Today' for the current local date", () => {
    expect(formatRelativeDate(todayISO())).toBe("Today");
  });

  it("returns 'Yesterday' for the previous local date", () => {
    const d = new Date();
    d.setDate(d.getDate() - 1);
    const iso = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    expect(formatRelativeDate(iso)).toBe("Yesterday");
  });

  it("formats older dates as DD/MM/YYYY", () => {
    expect(formatRelativeDate("2026-06-15")).toBe("15/06/2026");
  });
});

describe("parseAmountInput", () => {
  it("parses digits ignoring separators", () => {
    expect(parseAmountInput("1.500.000")).toBe(1500000);
  });

  it("returns 0 for empty input", () => {
    expect(parseAmountInput("")).toBe(0);
  });
});

describe("parseRupiahToCents", () => {
  it("parses a typed rupiah amount into cents", () => {
    expect(parseRupiahToCents("12.500")).toBe(1250000);
  });

  it("returns 0 for empty input", () => {
    expect(parseRupiahToCents("")).toBe(0);
  });
});

describe("formatIDRFromCents", () => {
  it("divides cents by 100 and formats as IDR", () => {
    expect(formatIDRFromCents(25000000)).toBe("Rp 250.000");
  });

  it("rounds sub-rupiah cents to whole rupiah", () => {
    expect(formatIDRFromCents(150)).toBe("Rp 2");
  });

  it("formats zero cents as zero rupiah", () => {
    expect(formatIDRFromCents(0)).toBe("Rp 0");
  });
});

describe("parseDateInput", () => {
  it("parses a valid ISO date", () => {
    const d = parseDateInput("2026-06-15");
    expect(d).not.toBeNull();
    expect(d?.getFullYear()).toBe(2026);
    expect(d?.getMonth()).toBe(5);
    expect(d?.getDate()).toBe(15);
  });

  it("returns null for an invalid date", () => {
    expect(parseDateInput("not-a-date")).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(parseDateInput("")).toBeNull();
  });
});
