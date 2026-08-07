/**
 * Types for the workbench: tab model, module tree, and entry kinds.
 *
 * A tab is either a list-view of an entity (e.g. "Other Payment" history)
 * or an entry form (draft or persisted). Tab identity is stable via id so
 * sessionStorage can persist the open set.
 */

export type ModuleId =
  | "cash-bank"
  | "sales"
  | "purchases"
  | "inventory"
  | "fixed-assets"
  | "reports";

export type TabKind = "list" | "entry";

export type EntrySubKind =
  | "cash-receipt"
  | "cash-payment"
  | "cash-transfer"
  | "sales-invoice"
  | "sales-receipt"
  | "purchase-invoice"
  | "purchase-payment"
  | "inventory-item"
  | "asset-register";

export type ListSubKind =
  | "cash-other-receipt"
  | "cash-other-payment"
  | "cash-transfer"
  | "sales-invoice"
  | "sales-receipt"
  | "purchase-invoice"
  | "purchase-payment"
  | "inventory-items"
  | "stock-movements"
  | "asset-register"
  | "report-trial-balance"
  | "report-profit-loss"
  | "report-balance-sheet"
  | "report-cash-flow";

export interface TabBase {
  id: string;
  moduleId: ModuleId;
  title: string;
  status?: string;
  unsaved?: boolean;
  createdAt: number;
}

export interface ListTab extends TabBase {
  kind: "list";
  subKind: ListSubKind;
}

export interface EntryTab extends TabBase {
  kind: "entry";
  subKind: EntrySubKind;
  /** When false, this is a draft. Persisted entries carry backend ids. */
  draft: boolean;
  entryId?: string | number;
}

export type Tab = ListTab | EntryTab;

export interface SubItem {
  id: string;
  label: string;
  hint?: string;
  openList: ListSubKind;
  openEntry?: EntrySubKind;
  /** When true, this sub-item has a structured mock backend (no real API yet). */
  mockData?: boolean;
}

export interface Module {
  id: ModuleId;
  label: string;
  icon: "wallet" | "sale" | "purchase" | "box" | "building" | "report";
  items: SubItem[];
}
