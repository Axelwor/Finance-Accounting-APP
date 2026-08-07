/**
 * Workbench state: tab list + active tab id, persisted to sessionStorage
 * so a refresh keeps the user's open tabs (and unsaved-entry warning).
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
import type { EntrySubKind, ListSubKind, Tab } from "./types";
import { defaultEntryTitle, defaultListTitle, draftNumber, findSubItemByList } from "./modules";

const STORAGE_KEY = "ledgerly.workbench.v1";
const MAX_TABS = 12;

interface State {
  tabs: Tab[];
  activeId: string | null;
}

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

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "hydrate": {
      return action.state;
    }
    case "ensure-dashboard": {
      // Always keep the Dashboard tab as the first tab. If it was closed
      // (or never opened), re-add it and activate it. This guarantees the
      // workbench always lands on the Dashboard.
      const existing = state.tabs.find((t) => t.kind === "dashboard");
      if (existing) {
        if (state.activeId && state.activeId !== existing.id) return state;
        return { ...state, activeId: existing.id };
      }
      const tab: Tab = {
        id: "tab-dashboard",
        kind: "dashboard",
        moduleId: "cash-bank",
        title: "Dashboard",
        createdAt: Date.now(),
      };
      return { tabs: [tab, ...state.tabs], activeId: tab.id };
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
      return { tabs: [tab, ...state.tabs], activeId: tab.id };
    }
    case "open-list": {
      const existing = state.tabs.find((t) => t.kind === "list" && t.subKind === action.subKind);
      if (existing) {
        return { ...state, activeId: existing.id };
      }
      const id = `list-${action.subKind}-${Date.now()}`;
      const tab: Tab = {
        id,
        kind: "list",
        moduleId: findSubItemByList(action.subKind)?.module.id ?? "cash-bank",
        subKind: action.subKind,
        title: defaultListTitle(action.subKind),
        createdAt: Date.now(),
      };
      return addTab(state, tab);
    }
    case "open-entry-draft": {
      const id = `entry-${action.subKind}-draft-${Date.now()}`;
      const tab: Tab = {
        id,
        kind: "entry",
        moduleId: "cash-bank", // refined by the form component if needed
        subKind: action.subKind,
        title: defaultEntryTitle(action.subKind),
        draft: true,
        status: draftNumber(action.subKind),
        unsaved: true,
        createdAt: Date.now(),
      };
      return addTab(state, tab);
    }
    case "open-entry-existing": {
      const id = `entry-${action.subKind}-${action.entryId}`;
      const tab: Tab = {
        id,
        kind: "entry",
        moduleId: "cash-bank",
        subKind: action.subKind,
        title: action.title,
        draft: false,
        status: action.status,
        entryId: action.entryId,
        createdAt: Date.now(),
      };
      return addTab(state, tab);
    }
    case "close": {
      const idx = state.tabs.findIndex((t) => t.id === action.id);
      if (idx < 0) return state;
      const tabs = state.tabs.filter((t) => t.id !== action.id);
      let activeId = state.activeId;
      if (activeId === action.id) {
        activeId = tabs[idx]?.id ?? tabs[idx - 1]?.id ?? null;
      }
      return { tabs, activeId };
    }
    case "activate": {
      return { ...state, activeId: action.id };
    }
    case "replace-draft": {
      const tabs = state.tabs.map((t) =>
        t.id === action.id ? { ...t, title: action.title, status: action.status, draft: false, unsaved: false } : t,
      );
      return { ...state, tabs };
    }
    case "mark-unsaved": {
      const tabs = state.tabs.map((t) => (t.id === action.id ? { ...t, unsaved: action.unsaved } : t));
      return { ...state, tabs };
    }
  }
}

function addTab(state: State, tab: Tab): State {
  let tabs = [...state.tabs, tab];
  // Cap tabs at MAX_TABS, closing the oldest inactive first.
  if (tabs.length > MAX_TABS) {
    const idx = tabs.findIndex((t) => t.id !== state.activeId);
    if (idx >= 0) tabs = tabs.filter((_, i) => i !== idx);
    else tabs = tabs.slice(tabs.length - MAX_TABS);
  }
  return { tabs, activeId: tab.id };
}

interface WorkbenchApi {
  tabs: Tab[];
  activeId: string | null;
  activeTab: Tab | null;
  openDashboard: () => void;
  openList: (sub: ListSubKind) => void;
  openEntryDraft: (sub: EntrySubKind) => void;
  openEntryExisting: (sub: EntrySubKind, entryId: string | number, title: string, status?: string) => void;
  close: (id: string) => void;
  activate: (id: string) => void;
  replaceDraft: (id: string, title: string, status: string) => void;
  markUnsaved: (id: string, unsaved: boolean) => void;
}

const WorkbenchContext = createContext<WorkbenchApi | null>(null);

function loadFromStorage(): State | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as State;
    if (!parsed || !Array.isArray(parsed.tabs)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function WorkbenchProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, { tabs: [], activeId: null });
  const hydrated = useRef(false);

  useEffect(() => {
    if (hydrated.current) return;
    hydrated.current = true;
    const restored = loadFromStorage();
    if (restored) {
      dispatch({ type: "hydrate", state: restored });
    }
    // Always make sure the Dashboard tab exists and is active on first
    // load (or after a refresh that wiped all tabs).
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
    const tab = state.tabs.find((t) => t.id === id);
    if (tab?.unsaved && !window.confirm("Close without saving? Unsaved changes will be lost.")) return;
    dispatch({ type: "close", id });
  }, [state.tabs]);
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
    return {
      tabs: state.tabs,
      activeId: state.activeId,
      activeTab,
      openDashboard,
      openList,
      openEntryDraft,
      openEntryExisting,
      close,
      activate,
      replaceDraft,
      markUnsaved,
    };
  }, [state, openDashboard, openList, openEntryDraft, openEntryExisting, close, activate, replaceDraft, markUnsaved]);

  return <WorkbenchContext.Provider value={api}>{children}</WorkbenchContext.Provider>;
}

export function useWorkbench(): WorkbenchApi {
  const ctx = useContext(WorkbenchContext);
  if (!ctx) throw new Error("useWorkbench must be used inside <WorkbenchProvider>.");
  return ctx;
}
