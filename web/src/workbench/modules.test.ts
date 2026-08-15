import { describe, expect, it } from "vitest";
import {
  defaultListTitle,
  defaultEntryTitle,
  draftNumber,
  findModule,
  findSubItemByList,
} from "./modules";

describe("defaultListTitle", () => {
  it("returns human titles for every known list kind", () => {
    expect(defaultListTitle("cash-other-receipt")).toBe("Other Receipt");
    expect(defaultListTitle("sales-invoice")).toBe("Sales Invoices");
    expect(defaultListTitle("purchase-order")).toBe("Purchase Orders");
    expect(defaultListTitle("journal-entry")).toBe("Journal Entries");
    expect(defaultListTitle("warehouse-list")).toBe("Warehouses");
    expect(defaultListTitle("email-queue")).toBe("Email Queue");
    expect(defaultListTitle("cost-center-list")).toBe("Cost Centers");
    expect(defaultListTitle("petty-cash-funds")).toBe("Petty Cash Funds");
    expect(defaultListTitle("recurring-transactions")).toBe("Recurring Transactions");
  });

  it("falls back to the raw kind for unknown values", () => {
    // Cast because the type is a closed union — this tests the default branch.
    expect(defaultListTitle("something-new" as never)).toBe("something-new");
  });
});

describe("defaultEntryTitle", () => {
  it("returns human titles for entry kinds", () => {
    expect(defaultEntryTitle("money-in")).toBe("Other Receipt");
    expect(defaultEntryTitle("sales-invoice")).toBe("Sales Invoice");
    expect(defaultEntryTitle("grn-entry")).toBe("Goods Received Note");
    expect(defaultEntryTitle("pc-fund-entry")).toBe("Entry"); // falls to default
    expect(defaultEntryTitle("warehouse-entry")).toBe("Warehouse");
  });
});

describe("draftNumber", () => {
  it("returns stable draft numbers per entry kind", () => {
    expect(draftNumber("money-in")).toBe("OR-DRAFT");
    expect(draftNumber("money-out")).toBe("OP-DRAFT");
    expect(draftNumber("cash-transfer")).toBe("BT-DRAFT");
    expect(draftNumber("sales-invoice")).toBe("SI-DRAFT");
    expect(draftNumber("journal-entry")).toBe("JE-DRAFT");
  });

  it("returns DRAFT for unmapped kinds", () => {
    expect(draftNumber("pc-replenish-entry")).toBe("DRAFT");
  });
});

describe("findModule", () => {
  it("locates a module by its id", () => {
    const found = findModule("sales");
    expect(found).toBeDefined();
    expect(found!.label).toBe("Sales");
  });

  it("returns undefined for unknown module ids", () => {
    expect(findModule("nope" as never)).toBeUndefined();
  });
});

describe("findSubItemByList", () => {
  it("resolves the module + sub-item that opens a given list", () => {
    const found = findSubItemByList("sales-invoice");
    expect(found).toBeDefined();
    expect(found!.module.label).toBe("Sales");
    expect(found!.item.openList).toBe("sales-invoice");
  });

  it("returns undefined for list kinds owned by no sub-item", () => {
    expect(findSubItemByList("nonexistent-list" as never)).toBeUndefined();
  });
});
