import { describe, expect, it } from "vitest";
import {
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

// Intl inserts a U+00A0 (non-breaking space) between the currency code and
// the amount in the en-US IDR format. Build expected strings with the same
// character so the assertions are exact.
const NBSP = "\u00A0";

describe("formatIDR", () => {
  it("formats whole rupiah with IDR currency code", () => {
    expect(formatIDR(1500000)).toBe(`IDR${NBSP}1,500,000`);
  });

  it("drops decimal fraction digits", () => {
    expect(formatIDR(1234.56)).toBe(`IDR${NBSP}1,235`);
  });

  it("formats zero", () => {
    expect(formatIDR(0)).toBe(`IDR${NBSP}0`);
  });

  it("formats negative amounts", () => {
    expect(formatIDR(-25000)).toBe(`-IDR${NBSP}25,000`);
  });
});

describe("fmtCurrencyIDR", () => {
  it("is an alias of formatIDR", () => {
    expect(fmtCurrencyIDR(999000)).toBe(formatIDR(999000));
  });
});

describe("formatDate", () => {
  it("formats an ISO date as 'Month D, YYYY'", () => {
    expect(formatDate("2026-06-15")).toBe("June 15, 2026");
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
    expect(fmtDateIDR("2026-01-02")).toBe("January 2, 2026");
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
    const [y, m, d] = [
      new Date().getFullYear(),
      String(new Date().getMonth() + 1).padStart(2, "0"),
      String(new Date().getDate()).padStart(2, "0"),
    ];
    expect(formatRelativeDate(`${y}-${m}-${d}`)).toBe("Today");
  });

  it("returns 'Yesterday' for one day ago", () => {
    const yest = new Date();
    yest.setDate(yest.getDate() - 1);
    const iso = `${yest.getFullYear()}-${String(yest.getMonth() + 1).padStart(2, "0")}-${String(
      yest.getDate(),
    ).padStart(2, "0")}`;
    expect(formatRelativeDate(iso)).toBe("Yesterday");
  });

  it("formats older dates as 'Mon D, YYYY'", () => {
    expect(formatRelativeDate("2026-01-02")).toBe("Jan 2, 2026");
  });
});

describe("parseAmountInput", () => {
  it("parses plain digits", () => {
    expect(parseAmountInput("15000")).toBe(15000);
  });

  it("strips non-digit characters (separators, currency symbols)", () => {
    expect(parseAmountInput("1.500.000")).toBe(1500000);
    expect(parseAmountInput("Rp 25,000")).toBe(25000);
  });

  it("returns 0 for empty or non-numeric input", () => {
    expect(parseAmountInput("")).toBe(0);
    expect(parseAmountInput("abc")).toBe(0);
  });
});

describe("parseRupiahToCents", () => {
  it("multiplies integer rupiah by 100 (backend stores cents)", () => {
    expect(parseRupiahToCents("150000")).toBe(15000000);
  });

  it("strips separators and currency symbols before converting", () => {
    expect(parseRupiahToCents("1.250.000")).toBe(125000000);
    expect(parseRupiahToCents("Rp 12,500")).toBe(1250000);
  });

  it("returns 0 for empty or non-numeric input", () => {
    expect(parseRupiahToCents("")).toBe(0);
    expect(parseRupiahToCents("abc")).toBe(0);
  });

  it("returns 0 for explicit zero", () => {
    expect(parseRupiahToCents("0")).toBe(0);
  });
});

describe("formatIDRFromCents", () => {
  it("divides cents by 100 and formats as IDR", () => {
    expect(formatIDRFromCents(15000000)).toBe(`IDR${NBSP}150,000`);
  });

  it("rounds sub-rupiah cents to whole rupiah (consistent with formatIDR)", () => {
    expect(formatIDRFromCents(1234567)).toBe(`IDR${NBSP}12,346`);
  });

  it("formats zero cents as zero rupiah", () => {
    expect(formatIDRFromCents(0)).toBe(`IDR${NBSP}0`);
  });

  it("matches formatIDR styling for the same amount", () => {
    expect(formatIDRFromCents(250000)).toBe(formatIDR(2500));
  });
});

describe("parseDateInput", () => {
  it("parses a valid yyyy-mm-dd into a local Date", () => {
    const d = parseDateInput("2026-08-15");
    expect(d).not.toBeNull();
    expect(d!.getFullYear()).toBe(2026);
    expect(d!.getMonth()).toBe(7); // August (0-indexed)
    expect(d!.getDate()).toBe(15);
  });

  it("returns null for empty input", () => {
    expect(parseDateInput("")).toBeNull();
  });

  it("returns null for malformed dates", () => {
    expect(parseDateInput("15/08/2026")).toBeNull();
    expect(parseDateInput("--")).toBeNull();
  });
});
