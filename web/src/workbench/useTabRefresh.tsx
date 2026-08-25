/**
 * Tab-activation refresh plumbing.
 *
 * Workbench panes stay mounted while hidden (CSS display toggling), so a
 * list/form that fetches on mount shows stale data after the user creates
 * records elsewhere and switches back. Two pieces fix that:
 *
 *   1. `PaneTabScope` — WorkArea wraps every nested pane so a screen can
 *      learn which workbench tab it is without prop drilling.
 *   2. `useTabRefresh(refresh)` — calls `refresh` (latest closure) whenever
 *      this tab becomes active again or is re-opened via its menu item.
 *      The first activation is skipped because on-mount fetching already
 *      covers it; subsequent triggers are lightly debounced.
 *
 * The activation counter lives in the workbench reducer (`activation`),
 * which bumps whenever a nested child becomes active — including the
 * dedupe path where re-clicking a menu item reactivates an existing tab.
 */

import { createContext, useContext, useEffect, useRef, type ReactNode } from "react";
import type { NestedTab } from "./types";
import { useWorkbench } from "./state";

const PaneTabContext = createContext<NestedTab | null>(null);

/** Wraps one nested pane in WorkArea so inner screens know their tab id. */
export function PaneTabScope({ tab, children }: { tab: NestedTab; children: ReactNode }) {
  return <PaneTabContext.Provider value={tab}>{children}</PaneTabContext.Provider>;
}

interface Options {
  /** Explicit nested-tab id; defaults to the enclosing PaneTabScope tab. */
  tabId?: string;
  /** Debounce before refetching, ms (default 150). */
  debounceMs?: number;
}

export function useTabRefresh(refresh: () => void, options: Options = {}) {
  const workbench = useWorkbench();
  const scoped = useContext(PaneTabContext);
  const id = options.tabId ?? scoped?.id ?? null;

  // -1 = inactive; otherwise how many times this child has been activated.
  const active = id != null && workbench.activeNested?.id === id;
  const version = active ? workbench.activation[id] ?? 0 : -1;

  const refreshRef = useRef(refresh);
  refreshRef.current = refresh;
  const lastVersion = useRef<number | null>(null);
  const sawInactive = useRef(false);

  useEffect(() => {
    if (id == null) return;
    const prev = lastVersion.current;
    lastVersion.current = version;
    if (version < 0) {
      sawInactive.current = true;
      return;
    }
    // Same version seen again (StrictMode double-invoke, unrelated renders).
    if (prev === version) return;
    // First activation coincides with the component's own on-mount fetch.
    if (prev === null && !sawInactive.current) return;
    const timer = window.setTimeout(() => refreshRef.current(), options.debounceMs ?? 150);
    return () => window.clearTimeout(timer);
  }, [id, version, options.debounceMs]);
}

/** Merge freshly fetched dropdown options into existing state by id:
 *  updates known entries in place, appends unknown ones, drops nothing —
 *  so user selections (stored as ids elsewhere) are never disturbed. */
export function mergeById<T>(existing: T[], incoming: T[], keyOf: (row: T) => string | number): T[] {
  const freshByKey = new Map(incoming.map((row) => [keyOf(row), row]));
  const seen = new Set<string | number>();
  const merged = existing.map((row) => {
    const key = keyOf(row);
    seen.add(key);
    return freshByKey.get(key) ?? row;
  });
  for (const row of incoming) {
    const key = keyOf(row);
    if (!seen.has(key)) {
      seen.add(key);
      merged.push(row);
    }
  }
  return merged;
}
