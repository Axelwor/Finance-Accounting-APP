import { useEffect, useRef, type RefObject } from "react";

/**
 * Shared dialog a11y behavior for manual (non-md-dialog) modals:
 *
 *  - focuses the first focusable element inside `dialogRef` on open
 *    (or `initialFocusRef` when provided),
 *  - traps Tab/Shift+Tab inside the dialog while open,
 *  - closes on Escape anywhere (document-level, capture),
 *  - restores focus to the previously focused element on close/unmount.
 */
export function useDialogA11y(
  open: boolean,
  onClose: () => void,
  dialogRef: RefObject<HTMLElement | null>,
  initialFocusRef?: RefObject<HTMLElement | null>,
) {
  // Keep callbacks in refs so the effect only runs on open/close transitions.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const initialFocusRefSafe = useRef(initialFocusRef);
  initialFocusRefSafe.current = initialFocusRef;

  useEffect(() => {
    if (!open) return;
    if (dialogRef.current == null) return;
    // Non-null alias so nested handlers keep the narrowed type.
    const root: HTMLElement = dialogRef.current;

    const previouslyFocused =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const focusables = (): HTMLElement[] =>
      Array.from(
        root.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ),
      ).filter((el) => {
        const style = window.getComputedStyle(el);
        return style.display !== "none" && style.visibility !== "hidden";
      });

    const initial =
      (initialFocusRefSafe.current?.current as HTMLElement | null) ??
      focusables()[0] ??
      root;
    initial.focus();
    // Web components / non-focusable content may swallow .focus(); fall back
    // to the dialog container so focus never stays outside the modal.
    if (!root.contains(document.activeElement)) {
      root.setAttribute("tabindex", "-1");
      root.focus();
    }

    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const items = focusables();
      if (items.length === 0) {
        e.preventDefault();
        return;
      }
      const first = items[0];
      const last = items[items.length - 1];
      const active = document.activeElement;
      if (e.shiftKey) {
        if (active === first || !root.contains(active)) {
          e.preventDefault();
          last.focus();
        }
      } else if (active === last || !root.contains(active)) {
        e.preventDefault();
        first.focus();
      }
    }

    // Capture so the trap wins even if focus sits on a non-dialog node.
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
      previouslyFocused?.focus();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
}
