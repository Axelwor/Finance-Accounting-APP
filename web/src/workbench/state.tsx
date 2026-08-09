/**
 * Workbench state: top-level tab list (dashboard + module parents) plus
 * per-module nested child tabs (list views + entry forms).
 *
 * Persisted to sessionStorage so a refresh keeps the open tabs and
 * unsaved-entry warning. The Dashboard tab is pinned as the first
 * top-level tab; it cannot be closed.
 *
 * Single source of truth: the WorkbenchProvider wraps the app shell, and
 * screens call useWorkbench() to read/open/close/activate tabs.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from "react";
import type {
  EntrySubKind,
  EntryTab,
  ListSubKind,
  ModuleId,
  ModuleTab,
  NestedTab,
  Tab,
} from "./types";
import { defaultEntryTitle, defaultListTitle, draftNumber, findSubItemByList, findModule } from "./modules";

const STORAGE_KEY = "ledgerly.workbench.v2";
const MAX_NESTED_PER_MODULE = 12;

interface State {
  /** Top-level tabs: dashboard (pinned) + module parents. */
  tabs: Tab[];
  /** Children of each module parent, keyed by parent id. */
  nested: Record<string, NestedTab[]>;
  /** Currently active top-level tab id. */
  activeId: string | null;
  /** Currently active child per module parent. */
  activeChild: Record<string, string | null>;
}

const EMPTY_STATE: State = { tabs: [], nested: {}, activeId: null, activeChild: {} };

type Action =
  | { type: "open-list"; subKind: ListSubKind }
  | { type: "open-entry-draft"; subKind: EntrySubKind }
  | { type: "open-entry-existing"; subKind: EntrySubKind; entryId: string | number; title: string; status?: string }
  | { type: "open-dashboard" }
  | { type: "close"; id: string }
  | { type: "activate"; id: string }
  | { type: "replace-draft"; id: string; title: string; status: string }
  | { type: "mark-unsaved"; id: string; unsaved: boolean }
  | { type: "hydrate"; state: State }
  | { type: "ensure-dashboard" };

function newId(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
}

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "hydrate": {
      return action.state;
    }
    case "ensure-dashboard": {
      // Always keep the Dashboard tab as the first tab.
      const existing = state.tabs.find((t) => t.kind === "dashboard");
      if (existing) {
        if (!state.activeId) return { ...state, activeId: existing.id };
        return state;
      }
      const tab: Tab = {
        id: "tab-dashboard",
        kind: "dashboard",
        moduleId: "cash-bank",
        title: "Dashboard",
        createdAt: Date.now(),
      };
      return { ...state, tabs: [tab, ...state.tabs], activeId: tab.id };
    }
    case "open-dashboard": {
      const existing = state.tabs.find((t) => t.kind === "dashboard");
      if (existing) return { ...state, activeId: existing.id };
      const tab: Tab = {
        id: "tab-dashboard",
        kind: "dashboard",
        moduleId: "cash-bank",
        title: "Dashboard",
        createdAt: Date.now(),
      };
      return { ...state, tabs: [tab, ...state.tabs], activeId: tab.id };
    }
    case "open-list": {
      const lookup = findSubItemByList(action.subKind);
      if (!lookup) return state;
      const moduleId = lookup.module.id;
      const parent = ensureModuleParent(state, moduleId, lookup.item.label);
      return activateNestedChild(parent, {
        id: newId(`list-${action.subKind}`),
        kind: "list",
        moduleId,
        subKind: action.subKind,
        title: defaultListTitle(action.subKind),
        createdAt: Date.now(),
      });
    }
    case "open-entry-draft": {
      const lookup = findSubItemByList(
        // best-effort module inference for drafts: cash-transfer/money-in/money-out belong to cash-bank;
        // other modules via their entry subkind.
        entryKindToListKind(action.subKind) ?? ("cash-other-receipt" as ListSubKind),
      );
      const moduleId: ModuleId = lookup?.module.id ?? "cash-bank";
      const subLabel = lookup?.item.label ?? findModule(moduleId)?.label ?? moduleId;
      const parent = ensureModuleParent(state, moduleId, subLabel);
      return activateNestedChild(parent, {
        id: newId(`entry-${action.subKind}-draft`),
        kind: "entry",
        moduleId,
        subKind: action.subKind,
        title: defaultEntryTitle(action.subKind),
        draft: true,
        status: draftNumber(action.subKind),
        unsaved: true,
        createdAt: Date.now(),
      });
    }
    case "open-entry-existing": {
      const lookup = findSubItemByList(entryKindToListKind(action.subKind) ?? ("cash-other-receipt" as ListSubKind));
      const moduleId: ModuleId = lookup?.module.id ?? "cash-bank";
      const subLabel = lookup?.item.label ?? findModule(moduleId)?.label ?? moduleId;
      const parent = ensureModuleParent(state, moduleId, subLabel);
      return activateNestedChild(parent, {
        id: `entry-${action.subKind}-${action.entryId}`,
        kind: "entry",
        moduleId,
        subKind: action.subKind,
        title: action.title,
        draft: false,
        status: action.status,
        entryId: action.entryId,
        createdAt: Date.now(),
      });
    }
    case "close": {
      // Dashboard is pinned: closing it just reactivates it.
      if (action.id === "tab-dashboard") return state;
      // Close a module parent: drop it and all its children.
      const parent = state.tabs.find((t) => t.id === action.id);
      if (parent && parent.kind === "module") {
        return closeModule(state, parent.id);
      }
      // Close a nested child of some module.
      for (const parentId of Object.keys(state.nested)) {
        const children = state.nested[parentId];
        const idx = children.findIndex((c) => c.id === action.id);
        if (idx < 0) continue;
        const remaining = children.filter((c) => c.id !== action.id);
        const nextNested = { ...state.nested, [parentId]: remaining };
        let activeChild = state.activeChild;
        if (state.activeChild[parentId] === action.id) {
          const fallback = remaining[Math.min(idx, remaining.length - 1)]?.id ?? null;
          activeChild = { ...state.activeChild, [parentId]: fallback };
        }
        // If the parent has no children left, leave the parent in place
        // (closing it requires explicit user action via the tab strip ×).
        return { ...state, nested: nextNested, activeChild };
      }
      return state;
    }
    case "activate": {
      const tab = state.tabs.find((t) => t.id === action.id);
      if (tab) return { ...state, activeId: action.id };
      // Activating a nested child.
      for (const parentId of Object.keys(state.nested)) {
        if (state.nested[parentId].some((c) => c.id === action.id)) {
          return {
            ...state,
            activeId: parentId,
            activeChild: { ...state.activeChild, [parentId]: action.id },
          };
        }
      }
      return state;
    }
    case "replace-draft": {
      // Find which module owns this nested child.
      const next = { ...state.nested };
      for (const parentId of Object.keys(state.nested)) {
        const idx = next[parentId].findIndex((c) => c.id === action.id);
        if (idx < 0) continue;
        const current = next[parentId][idx];
        if (current.kind === "entry") {
          // Cast through unknown because the discriminator narrowed the kind.
          const updated = current as NestedTab;
          (updated as EntryTab).draft = false;
          (updated as EntryTab).title = action.title;
          (updated as EntryTab).status = action.status;
          (updated as EntryTab).unsaved = false;
        }
        next[parentId] = [...next[parentId]];
        next[parentId][idx] = current;
        return { ...state, nested: next };
      }
      return state;
    }
    case "mark-unsaved": {
      const next = { ...state.nested };
      for (const parentId of Object.keys(state.nested)) {
        const idx = next[parentId].findIndex((c) => c.id === action.id);
        if (idx >= 0) {
          const updated: NestedTab = { ...next[parentId][idx], unsaved: action.unsaved };
          next[parentId] = [...next[parentId]];
          next[parentId][idx] = updated;
          return { ...state, nested: next };
        }
      }
      return state;
    }
  }
}

function ensureModuleParent(state: State, moduleId: ModuleId, title?: string): State {
  const existing = state.tabs.find((t): t is ModuleTab => t.kind === "module" && t.moduleId === moduleId);
  if (existing) {
    // Update the tab title to reflect the latest sub-item opened.
    if (title && existing.title !== title) {
      const updated: ModuleTab = { ...existing, title };
      return {
        ...state,
        tabs: state.tabs.map((t) => (t.id === existing.id ? updated : t)),
      };
    }
    return state;
  }
  const module = findModule(moduleId);
  const tab: ModuleTab = {
    id: `module-${moduleId}`,
    kind: "module",
    moduleId,
    title: title ?? module?.label ?? moduleId,
    createdAt: Date.now(),
  };
  return {
    ...state,
    tabs: [...state.tabs, tab],
  };
}

function activateNestedChild(state: State, child: NestedTab): State {
  const parentId = `module-${child.moduleId}`;
  const parentExists = state.tabs.some((t) => t.id === parentId);
  // Determine the sub-item label for the tab title.
  const childListKind = child.kind === "list" ? child.subKind : entryKindToListKind(child.subKind) ?? ("cash-other-receipt" as ListSubKind);
  const childLookup = findSubItemByList(childListKind);
  const childLabel = childLookup?.item.label ?? findModule(child.moduleId)?.label ?? child.moduleId;
  const next: State = parentExists
    ? state
    : {
        ...state,
        tabs: [
          ...state.tabs,
          {
            id: parentId,
            kind: "module",
            moduleId: child.moduleId,
            title: childLabel,
            createdAt: Date.now(),
          },
        ],
      };
  const existingIdx = (next.nested[parentId] ?? []).findIndex((c) => {
    if (c.kind !== child.kind || c.subKind !== child.subKind) return false;
    if (child.kind === "entry" && c.kind === "entry") {
      return c.draft === child.draft;
    }
    return true;
  });
  const existing = existingIdx >= 0 ? next.nested[parentId][existingIdx] : null;
  let children = next.nested[parentId] ?? [];
  if (existing) {
    // Activate the existing child; no need to add another.
    return {
      ...next,
      nested: { ...next.nested, [parentId]: children },
      activeId: parentId,
      activeChild: { ...next.activeChild, [parentId]: existing.id },
    };
  }
  children = [...children, child];
  // Cap children per module.
  if (children.length > MAX_NESTED_PER_MODULE) {
    children = children.slice(children.length - MAX_NESTED_PER_MODULE);
  }
  return {
    ...next,
    nested: { ...next.nested, [parentId]: children },
    activeId: parentId,
    activeChild: { ...next.activeChild, [parentId]: child.id },
  };
}

function closeModule(state: State, parentId: string): State {
  const tabs = state.tabs.filter((t) => t.id !== parentId);
  const nested = { ...state.nested };
  delete nested[parentId];
  const activeChild = { ...state.activeChild };
  delete activeChild[parentId];
  let activeId = state.activeId;
  if (activeId === parentId) {
    activeId = state.tabs.find((t) => t.kind === "dashboard")?.id ?? tabs[0]?.id ?? null;
  }
  return { tabs, nested, activeId, activeChild };
}

/** Reverse lookup from EntrySubKind to a representative ListSubKind so the
 *  module parent can be inferred when opening a draft. */
function entryKindToListKind(kind: EntrySubKind): ListSubKind | null {
  switch (kind) {
    case "money-in":
      return "cash-other-receipt";
    case "money-out":
      return "cash-other-payment";
    case "cash-transfer":
      return "cash-transfer";
    case "bank-reconciliation-entry":
      return "bank-reconciliation";
    case "sales-invoice":
      return "sales-invoice";
    case "sales-receipt":
      return "sales-receipt";
    case "sales-quotation-entry":
      return "sales-quotation";
    case "sales-order-entry":
      return "sales-order";
    case "delivery-order-entry":
      return "delivery-order";
    case "credit-note-entry":
      return "credit-note";
    case "purchase-order-entry":
      return "purchase-order";
    case "grn-entry":
      return "grn";
    case "purchase-supplier-entry":
      return "purchase-supplier";
    case "supplier-invoice-entry":
      return "supplier-invoice";
    case "purchase-invoice":
      return "purchase-invoice";
    case "purchase-payment":
      return "purchase-payment";
    case "purchase-return-entry":
      return "purchase-return";
    case "inventory-item":
      return "inventory-items";
    case "stock-opname-entry":
      return "stock-opname";
    case "stock-transfer-entry":
      return "stock-transfer";
    case "asset-register":
      return "asset-register";
    case "journal-entry":
      return "journal-entry";
  }
}

interface WorkbenchApi {
  tabs: Tab[];
  nested: Record<string, NestedTab[]>;
  activeId: string | null;
  activeTab: Tab | null;
  activeNested: NestedTab | null;
  activeChildFor: (parentId: string) => string | null;
  openDashboard: () => void;
  openList: (sub: ListSubKind) => void;
  openEntryDraft: (sub: EntrySubKind) => void;
  openEntryExisting: (sub: EntrySubKind, entryId: string | number, title: string, status?: string) => void;
  close: (id: string) => void;
  activate: (id: string) => void;
  replaceDraft: (id: string, title: string, status: string) => void;
  markUnsaved: (id: string, unsaved: boolean) => void;
  /** Look up a nested tab by id, used by entry forms to update themselves. */
  getNested: (id: string) => NestedTab | undefined;
}

const WorkbenchContext = createContext<WorkbenchApi | null>(null);

function loadFromStorage(): State | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as State;
    if (!parsed || !Array.isArray(parsed.tabs) || typeof parsed.nested !== "object") return null;
    return parsed;
  } catch {
    return null;
  }
}

export function WorkbenchProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, EMPTY_STATE);
  const hydrated = useRef(false);

  useEffect(() => {
    if (hydrated.current) return;
    hydrated.current = true;
    const restored = loadFromStorage();
    if (restored) dispatch({ type: "hydrate", state: restored });
    dispatch({ type: "ensure-dashboard" });
  }, []);

  useEffect(() => {
    if (!hydrated.current) return;
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      /* storage full or disabled — ignore */
    }
  }, [state]);

  const openList = useCallback((sub: ListSubKind) => dispatch({ type: "open-list", subKind: sub }), []);
  const openEntryDraft = useCallback((sub: EntrySubKind) => dispatch({ type: "open-entry-draft", subKind: sub }), []);
  const openEntryExisting = useCallback(
    (sub: EntrySubKind, entryId: string | number, title: string, status?: string) =>
      dispatch({ type: "open-entry-existing", subKind: sub, entryId, title, status }),
    [],
  );
  const openDashboard = useCallback(() => dispatch({ type: "open-dashboard" }), []);
  const close = useCallback((id: string) => {
    // Dashboard is pinned — silently no-op instead of closing it.
    if (id === "tab-dashboard") return;
    // Nested child with unsaved changes — confirm before discarding.
    for (const parentId of Object.keys(state.nested)) {
      const child = state.nested[parentId].find((c) => c.id === id);
      if (child?.unsaved && !window.confirm("Close without saving? Unsaved changes will be lost.")) return;
    }
    dispatch({ type: "close", id });
  }, [state.nested]);
  const activate = useCallback((id: string) => dispatch({ type: "activate", id }), []);
  const replaceDraft = useCallback(
    (id: string, title: string, status: string) => dispatch({ type: "replace-draft", id, title, status }),
    [],
  );
  const markUnsaved = useCallback(
    (id: string, unsaved: boolean) => dispatch({ type: "mark-unsaved", id, unsaved }),
    [],
  );

  const api = useMemo<WorkbenchApi>(() => {
    const activeTab = state.tabs.find((t) => t.id === state.activeId) ?? null;
    let activeNested: NestedTab | null = null;
    if (activeTab && activeTab.kind === "module") {
      const childId = state.activeChild[activeTab.id];
      activeNested = childId ? state.nested[activeTab.id]?.find((c) => c.id === childId) ?? null : null;
    }
    return {
      tabs: state.tabs,
      nested: state.nested,
      activeId: state.activeId,
      activeTab,
      activeNested,
      activeChildFor: (parentId: string) => state.activeChild[parentId] ?? null,
      openDashboard,
      openList,
      openEntryDraft,
      openEntryExisting,
      close,
      activate,
      replaceDraft,
      markUnsaved,
      getNested: (id: string) => {
        for (const parentId of Object.keys(state.nested)) {
          const child = state.nested[parentId].find((c) => c.id === id);
          if (child) return child;
        }
        return undefined;
      },
    };
  }, [
    state,
    openDashboard,
    openList,
    openEntryDraft,
    openEntryExisting,
    close,
    activate,
    replaceDraft,
    markUnsaved,
  ]);

  return <WorkbenchContext.Provider value={api}>{children}</WorkbenchContext.Provider>;
}

export function useWorkbench(): WorkbenchApi {
  const ctx = useContext(WorkbenchContext);
  if (!ctx) throw new Error("useWorkbench must be used inside <WorkbenchProvider>.");
  return ctx;
}
