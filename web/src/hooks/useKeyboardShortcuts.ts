import { useEffect, useRef } from "react";

type Handler = (e: KeyboardEvent) => void;
type Shortcuts = Record<string, Handler>;

const MODIFIER_KEYS = ["Meta", "Control", "Alt", "Shift"];
const SPECIAL_KEYS = ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Enter", "Escape", "Space"];

function normalizeKey(key: string): string {
  return key.replace(/Key/g, "").replace(/Digit/g, "");
}

function parseShortcut(shortcut: string): { modifiers: string[]; key: string } {
  const parts = shortcut.split("+").map((p) => p.trim());
  const modifiers: string[] = [];
  let key = "";

  for (const part of parts) {
    if (MODIFIER_KEYS.includes(part) || part === "Cmd" || part === "Command") {
      if (part === "Cmd" || part === "Command") {
        modifiers.push("Meta");
      } else {
        modifiers.push(part);
      }
    } else {
      key = part;
    }
  }

  return { modifiers, key };
}

export function useKeyboardShortcuts(shortcuts: Shortcuts) {
  const shortcutsRef = useRef(shortcuts);
  shortcutsRef.current = shortcuts;

  const ignoreKeysRef = useRef(new Set(["Input", "textarea", "input", "select", "button"]));

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const tagName = (e.target as HTMLElement)?.tagName?.toLowerCase();
      const isEditable =
        tagName === "textarea" ||
        tagName === "input" ||
        tagName === "select" ||
        ((e.target as HTMLElement)?.isContentEditable ?? false);

      if (isEditable && e.key !== "Escape" && e.key !== "Enter") {
        return;
      }

      const { modifiers, key } = parseShortcut(e.code);
      const normalizedKey = normalizeKey(e.key);

      const expectedModifiers = Object.keys(shortcutsRef.current);
      for (const shortcut of expectedModifiers) {
        const { modifiers: expectedMod, key: expectedKey } = parseShortcut(shortcut);
        const actualKey = normalizeKey(e.key);

        if (actualKey.toUpperCase() !== expectedKey.toUpperCase()) continue;

        const hasAllModifiers = expectedMod.every((mod) => {
          if (mod === "Meta") return e.metaKey;
          if (mod === "Control") return e.ctrlKey;
          if (mod === "Alt") return e.altKey;
          if (mod === "Shift") return e.shiftKey;
          return false;
        });

        if (hasAllModifiers) {
          shortcutsRef.current[shortcut]?.(e);
          break;
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);
}
