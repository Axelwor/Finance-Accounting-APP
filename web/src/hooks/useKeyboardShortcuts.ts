import { useEffect } from "react";

/**
 * useKeyboardShortcuts — app-wide keyboard shortcuts listened on `document`.
 *
 *  - Ctrl/Cmd+S: dispatches `"app:save"` so the active entry form can save.
 *    Prevents the browser's native save dialog.
 *  - Esc: dispatches `"app:close-tab"` so the workbench can close the active
 *    tab. Components that use Esc for their own dismissal (e.g. the Combobox
 *    dropdown) call `stopPropagation` so the tab is not closed underneath
 *    them.
 *
 * Call once near the top of the mounted work area (e.g. <WorkArea />) so the
 * shortcuts are only active inside the shell, not on auth/onboarding screens.
 *
 * Custom events carry no payload; consumers add their own listeners:
 *
 *   useEffect(() => {
 *     const onSave = () => handleSave();
 *     document.addEventListener("app:save", onSave);
 *     return () => document.removeEventListener("app:save", onSave);
 *   }, [handleSave]);
 */

export interface KeyboardShortcutOptions {
  /** Toggle the listeners off without unmounting the host. Defaults to true. */
  enabled?: boolean;
}

export function useKeyboardShortcuts({ enabled = true }: KeyboardShortcutOptions = {}) {
  useEffect(() => {
    if (!enabled) return;

    function onKeyDown(e: KeyboardEvent) {
      const mod = e.ctrlKey || e.metaKey;

      // Ctrl/Cmd+S — save the active form.
      if (mod && (e.key === "s" || e.key === "S" || e.code === "KeyS")) {
        e.preventDefault();
        document.dispatchEvent(new CustomEvent("app:save"));
        return;
      }

      // Esc — close the active tab (no modifiers).
      if (!mod && !e.altKey && e.key === "Escape") {
        document.dispatchEvent(new CustomEvent("app:close-tab"));
      }
    }

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [enabled]);
}

/** Stable event names so consumers and this hook stay in sync. */
export const SAVE_EVENT = "app:save" as const;
export const CLOSE_TAB_EVENT = "app:close-tab" as const;
