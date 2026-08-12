/**
 * Types for the workbench: tab model, module tree, and entry kinds.
 *
 * The workbench uses a two-level tab model: a top-level tab is either the
 * dashboard or a module parent (one per sidebar group). A module parent
 * owns a nested list of child tabs (list views and entry forms) and a
 * single active child id, rendered as a sub-strip inside the work area.
 *
 * Tab identity is stable via id so sessionStorage can persist the open set.
 */

import type { EntrySubKind, ListSubKind } from "../types";

export type { EntrySubKind, ListSubKind };

export type ModuleId =
  | "cash-bank"
  | "sales"
  | "purchases"
  | "production"
  | "inventory"
  | "fixed-assets"
  | "accountant"
  | "reports"
  | "tax"
  | "email";

export type NestedTabKind = "list" | "entry";

/**
 * Reference to a parent source document used to pre-fill a new draft entry
 * (workflow chain: Quotation → SO, SO → DO, DO → Invoice, PO → GRN,
 * GRN → Supplier Invoice, Invoice → Credit Note). The target form loads the
 * parent document and copies its header fields + lines on mount.
 */
export type PrefillKind =
  | "quotation"
  | "sales-order"
  | "delivery-order"
  | "invoice"
  | "purchase-order"
  | "grn";

export interface PrefillRef {
  kind: PrefillKind;
  id: number;
}

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
  /** Parent document to auto-fill lines from (workflow chain drafts only). */
  prefill?: PrefillRef;
}

/** Module parent — owns a list of nested child tabs. */
export interface ModuleTab extends TabBase {
  kind: "module";
  /** The currently active sub-item label, shown in the top-level tab. */
  activeSubItem?: string;
}

/** Dashboard overview — opens by default when the user first lands. */
export interface DashboardTab extends TabBase {
  kind: "dashboard";
}

/** Top-level tab: dashboard or module parent. */
export type Tab = DashboardTab | ModuleTab;

/** Child tab that lives inside a module parent. */
export type NestedTab = ListTab | EntryTab;

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
  icon: "wallet" | "sale" | "purchase" | "box" | "building" | "report" | "ledger" | "factory" | "email";
  items: SubItem[];
}
