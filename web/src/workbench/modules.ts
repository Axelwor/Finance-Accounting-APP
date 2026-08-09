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
    { id: "cb-receipt", label: "Other Receipt", hint: "M+", openList: "cash-other-receipt", openEntry: "money-in" },
    { id: "cb-payment", label: "Other Payment", hint: "M-", openList: "cash-other-payment", openEntry: "money-out" },
    { id: "cb-transfer", label: "Bank Transfer", hint: "Xfer", openList: "cash-transfer", openEntry: "cash-transfer" },
    { id: "cb-recon", label: "Bank Reconciliation", hint: "REC", openList: "bank-reconciliation", openEntry: "bank-reconciliation-entry" },
  ],
};

const sales: Module = {
  id: "sales",
  label: "Sales",
  icon: "sale",
  items: [
    { id: "sl-quotation", label: "Quotations", hint: "SQ", openList: "sales-quotation", openEntry: "sales-quotation-entry" },
    { id: "sl-order", label: "Sales Orders", hint: "SO", openList: "sales-order", openEntry: "sales-order-entry" },
    { id: "sl-delivery", label: "Delivery Orders", hint: "DO", openList: "delivery-order", openEntry: "delivery-order-entry" },
    { id: "sl-invoice", label: "Sales Invoices", hint: "INV", openList: "sales-invoice", openEntry: "sales-invoice" },
    { id: "sl-credit-note", label: "Credit Notes", hint: "CN", openList: "credit-note", openEntry: "credit-note-entry" },
    { id: "sl-receipt", label: "Sales Receipts", hint: "RCP", openList: "sales-receipt", openEntry: "sales-receipt", mockData: true },
  ],
};

const purchases: Module = {
  id: "purchases",
  label: "Purchases",
  icon: "purchase",
  items: [
    { id: "pu-po", label: "Purchase Orders", hint: "PO", openList: "purchase-order", openEntry: "purchase-order-entry" },
    { id: "pu-grn", label: "Goods Received", hint: "GRN", openList: "grn", openEntry: "grn-entry" },
    { id: "pu-supplier", label: "Suppliers", hint: "SUP", openList: "purchase-supplier", openEntry: "purchase-supplier-entry" },
    { id: "pu-invoice", label: "Supplier Invoices", hint: "BIL", openList: "supplier-invoice", openEntry: "supplier-invoice-entry" },
    { id: "pu-payment", label: "Purchase Payments", hint: "PAY", openList: "purchase-payment", openEntry: "purchase-payment", mockData: true },
    { id: "pu-return", label: "Purchase Returns", hint: "PRET", openList: "purchase-return", openEntry: "purchase-return-entry" },
  ],
};

const production: Module = {
  id: "production",
  label: "Production",
  icon: "factory",
  items: [
    { id: "pr-bom", label: "Bill of Materials", hint: "BOM", openList: "bom", openEntry: "bom-entry" },
    { id: "pr-job", label: "Production Jobs", hint: "JOB", openList: "production-job", openEntry: "production-job-entry" },
  ],
};

const inventory: Module = {
  id: "inventory",
  label: "Inventory",
  icon: "box",
  items: [
    { id: "in-items", label: "Item List", hint: "ITM", openList: "inventory-items", openEntry: "inventory-item", mockData: true },
    { id: "in-movements", label: "Stock Movements", hint: "STK", openList: "stock-movements", mockData: true },
    { id: "in-opname", label: "Stock Opnames", hint: "OPN", openList: "stock-opname", openEntry: "stock-opname-entry" },
    { id: "in-transfer", label: "Stock Transfers", hint: "TRF", openList: "stock-transfer", openEntry: "stock-transfer-entry" },
  ],
};

const fixedAssets: Module = {
  id: "fixed-assets",
  label: "Fixed Assets",
  icon: "building",
  items: [
    { id: "fa-register", label: "Asset Register", hint: "AST", openList: "fixed-assets", openEntry: "fixed-assets-entry" },
  ],
};

const accountant: Module = {
  id: "accountant",
  label: "Accountant",
  icon: "ledger",
  items: [
    { id: "ac-journal", label: "Journal Entries", hint: "JE", openList: "journal-entry", openEntry: "journal-entry" },
    { id: "ac-ledger", label: "General Ledger", hint: "GL", openList: "general-ledger" },
    { id: "ac-register", label: "Journal Register", hint: "REG", openList: "journal-register" },
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
    { id: "rp-notes", label: "Financial Notes", hint: "CAL", openList: "financial-notes", openEntry: "financial-notes-entry" },
    { id: "rp-reminders", label: "Due Date Reminders", hint: "DUE", openList: "due-date-reminders" },
  ],
};

export const MODULES: Module[] = [cashBank, sales, purchases, production, inventory, fixedAssets, accountant, reports];

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
    case "sales-quotation":
      return "Quotations";
    case "sales-order":
      return "Sales Orders";
    case "delivery-order":
      return "Delivery Orders";
    case "credit-note":
      return "Credit Notes";
    case "purchase-order":
      return "Purchase Orders";
    case "grn":
      return "Goods Received Notes";
    case "purchase-supplier":
      return "Suppliers";
    case "supplier-invoice":
      return "Supplier Invoices";
    case "purchase-invoice":
      return "Purchase Invoices";
    case "purchase-payment":
      return "Purchase Payments";
    case "purchase-return":
      return "Purchase Returns";
    case "bank-reconciliation":
      return "Bank Reconciliation";
    case "inventory-items":
      return "Item List";
    case "stock-movements":
      return "Stock Movements";
    case "stock-opname":
      return "Stock Opnames";
    case "stock-transfer":
      return "Stock Transfers";
    case "asset-register":
      return "Asset Register";
    case "fixed-assets":
      return "Fixed Assets";
    case "journal-entry":
      return "Journal Entries";
    case "general-ledger":
      return "General Ledger";
    case "journal-register":
      return "Journal Register";
    case "report-trial-balance":
      return "Trial Balance";
    case "report-profit-loss":
      return "Profit & Loss";
    case "report-balance-sheet":
      return "Balance Sheet";
    case "report-cash-flow":
      return "Cash Flow";
    case "financial-notes":
      return "Financial Notes";
    case "due-date-reminders":
      return "Due Date Reminders";
    case "bom":
      return "Bill of Materials";
    case "production-job":
      return "Production Jobs";
  }
}

/** Default tab title for a draft entry. */
export function defaultEntryTitle(entryKind: import("./types").EntrySubKind): string {
  switch (entryKind) {
    case "money-in":
      return "Other Receipt";
    case "money-out":
      return "Other Payment";
    case "cash-transfer":
      return "Bank Transfer";
    case "sales-invoice":
      return "Sales Invoice";
    case "sales-receipt":
      return "Sales Receipt";
    case "sales-quotation-entry":
      return "Quotation";
    case "sales-order-entry":
      return "Sales Order";
    case "delivery-order-entry":
      return "Delivery Order";
    case "credit-note-entry":
      return "Credit Note";
    case "purchase-order-entry":
      return "Purchase Order";
    case "grn-entry":
      return "Goods Received Note";
    case "purchase-supplier-entry":
      return "Supplier";
    case "purchase-invoice":
      return "Purchase Invoice";
    case "supplier-invoice-entry":
      return "Supplier Invoice";
    case "purchase-payment":
      return "Purchase Payment";
    case "purchase-return-entry":
      return "Purchase Return";
    case "bank-reconciliation-entry":
      return "Bank Reconciliation";
    case "inventory-item":
      return "Item";
    case "stock-opname-entry":
      return "Stock Opname";
    case "stock-transfer-entry":
      return "Stock Transfer";
    case "asset-register":
      return "Asset";
    case "fixed-assets-entry":
      return "Fixed Asset";
    case "asset-depreciate":
      return "Depreciation";
    case "asset-dispose":
      return "Asset Disposal";
    case "journal-entry":
      return "Journal Entry";
    case "financial-notes-entry":
      return "Financial Note";
    case "bom-entry":
      return "Bill of Materials";
    case "production-job-entry":
      return "Production Job";
    default:
      return "Entry";
  }
}

/** Stable draft number per entry kind, e.g. "OP-DRAFT". */
export function draftNumber(entryKind: import("./types").EntrySubKind): string {
  switch (entryKind) {
    case "money-in": return "OR-DRAFT";
    case "money-out": return "OP-DRAFT";
    case "cash-transfer": return "BT-DRAFT";
    case "sales-invoice": return "SI-DRAFT";
    case "sales-receipt": return "SR-DRAFT";
    case "sales-quotation-entry": return "SQ-DRAFT";
    case "sales-order-entry": return "SO-DRAFT";
    case "delivery-order-entry": return "DO-DRAFT";
    case "credit-note-entry": return "CN-DRAFT";
    case "purchase-order-entry": return "PO-DRAFT";
    case "grn-entry": return "GRN-DRAFT";
    case "purchase-supplier-entry": return "SUP-DRAFT";
    case "purchase-invoice": return "PI-DRAFT";
    case "supplier-invoice-entry": return "BIL-DRAFT";
    case "purchase-payment": return "PP-DRAFT";
    case "purchase-return-entry": return "PRET-DRAFT";
    case "bank-reconciliation-entry": return "REC-DRAFT";
    case "inventory-item": return "IT-DRAFT";
    case "stock-opname-entry": return "OPN-DRAFT";
    case "stock-transfer-entry": return "TRF-DRAFT";
    case "asset-register": return "FA-DRAFT";
    case "fixed-assets-entry": return "FA-DRAFT";
    case "asset-depreciate": return "FA-DEP";
    case "asset-dispose": return "FA-DISP";
    case "journal-entry": return "JE-DRAFT";
    case "financial-notes-entry": return "FN-DRAFT";
    case "bom-entry": return "BOM-DRAFT";
    case "production-job-entry": return "PRD-DRAFT";
    default: return "DRAFT";
  }
}
