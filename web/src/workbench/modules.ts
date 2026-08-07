/**
 * Module registry for the sidebar. Each top-level module lists its
 * sub-items, and each sub-item maps to a default list tab and (where
 * applicable) an entry form kind. Items marked `mockData: true` render
 * from structured mocks until a real backend endpoint is wired.
 */

import type { Module, ModuleId, SubItem } from "./types";

const cashBank: Module = {
  id: "cash-bank",
  label: "Cash & Bank",
  icon: "wallet",
  items: [
    { id: "cb-receipt", label: "Other Receipt", hint: "M+", openList: "cash-other-receipt", openEntry: "cash-receipt" },
    { id: "cb-payment", label: "Other Payment", hint: "M-", openList: "cash-other-payment", openEntry: "cash-payment" },
    { id: "cb-transfer", label: "Bank Transfer", hint: "Xfer", openList: "cash-transfer", openEntry: "cash-transfer" },
  ],
};

const sales: Module = {
  id: "sales",
  label: "Sales",
  icon: "sale",
  items: [
    { id: "sl-invoice", label: "Sales Invoices", hint: "INV", openList: "sales-invoice", openEntry: "sales-invoice", mockData: true },
    { id: "sl-receipt", label: "Sales Receipts", hint: "RCP", openList: "sales-receipt", openEntry: "sales-receipt", mockData: true },
  ],
};

const purchases: Module = {
  id: "purchases",
  label: "Purchases",
  icon: "purchase",
  items: [
    { id: "pu-invoice", label: "Purchase Invoices", hint: "BIL", openList: "purchase-invoice", openEntry: "purchase-invoice", mockData: true },
    { id: "pu-payment", label: "Purchase Payments", hint: "PAY", openList: "purchase-payment", openEntry: "purchase-payment", mockData: true },
  ],
};

const inventory: Module = {
  id: "inventory",
  label: "Inventory",
  icon: "box",
  items: [
    { id: "in-items", label: "Item List", hint: "ITM", openList: "inventory-items", openEntry: "inventory-item", mockData: true },
    { id: "in-movements", label: "Stock Movements", hint: "STK", openList: "stock-movements", mockData: true },
  ],
};

const fixedAssets: Module = {
  id: "fixed-assets",
  label: "Fixed Assets",
  icon: "building",
  items: [
    { id: "fa-register", label: "Asset Register", hint: "AST", openList: "asset-register", openEntry: "asset-register", mockData: true },
  ],
};

const reports: Module = {
  id: "reports",
  label: "Reports",
  icon: "report",
  items: [
    { id: "rp-trial", label: "Trial Balance", hint: "TB", openList: "report-trial-balance" },
    { id: "rp-pl", label: "Profit & Loss", hint: "P&L", openList: "report-profit-loss" },
    { id: "rp-bs", label: "Balance Sheet", hint: "BS", openList: "report-balance-sheet" },
    { id: "rp-cf", label: "Cash Flow", hint: "CF", openList: "report-cash-flow" },
  ],
};

export const MODULES: Module[] = [cashBank, sales, purchases, inventory, fixedAssets, reports];

export function findModule(id: ModuleId): Module | undefined {
  return MODULES.find((m) => m.id === id);
}

export function findSubItem(moduleId: ModuleId, subId: string): SubItem | undefined {
  return findModule(moduleId)?.items.find((s) => s.id === subId);
}

export function findSubItemByList(listKind: import("./types").ListSubKind): { module: Module; item: SubItem } | undefined {
  for (const module of MODULES) {
    const item = module.items.find((s) => s.openList === listKind);
    if (item) return { module, item };
  }
  return undefined;
}

/** Default tab title for a list view. */
export function defaultListTitle(listKind: import("./types").ListSubKind): string {
  switch (listKind) {
    case "cash-other-receipt":
      return "Other Receipt";
    case "cash-other-payment":
      return "Other Payment";
    case "cash-transfer":
      return "Bank Transfer";
    case "sales-invoice":
      return "Sales Invoices";
    case "sales-receipt":
      return "Sales Receipts";
    case "purchase-invoice":
      return "Purchase Invoices";
    case "purchase-payment":
      return "Purchase Payments";
    case "inventory-items":
      return "Item List";
    case "stock-movements":
      return "Stock Movements";
    case "asset-register":
      return "Asset Register";
    case "report-trial-balance":
      return "Trial Balance";
    case "report-profit-loss":
      return "Profit & Loss";
    case "report-balance-sheet":
      return "Balance Sheet";
    case "report-cash-flow":
      return "Cash Flow";
  }
}

/** Default tab title for a draft entry. */
export function defaultEntryTitle(entryKind: import("./types").EntrySubKind): string {
  switch (entryKind) {
    case "cash-receipt":
      return "Other Receipt";
    case "cash-payment":
      return "Other Payment";
    case "cash-transfer":
      return "Bank Transfer";
    case "sales-invoice":
      return "Sales Invoice";
    case "sales-receipt":
      return "Sales Receipt";
    case "purchase-invoice":
      return "Purchase Invoice";
    case "purchase-payment":
      return "Purchase Payment";
    case "inventory-item":
      return "Item";
    case "asset-register":
      return "Asset";
  }
}

/** Stable draft number per entry kind, e.g. "OP-DRAFT". */
export function draftNumber(entryKind: import("./types").EntrySubKind): string {
  switch (entryKind) {
    case "cash-receipt": return "OR-DRAFT";
    case "cash-payment": return "OP-DRAFT";
    case "cash-transfer": return "BT-DRAFT";
    case "sales-invoice": return "SI-DRAFT";
    case "sales-receipt": return "SR-DRAFT";
    case "purchase-invoice": return "PI-DRAFT";
    case "purchase-payment": return "PP-DRAFT";
    case "inventory-item": return "IT-DRAFT";
    case "asset-register": return "FA-DRAFT";
  }
}
