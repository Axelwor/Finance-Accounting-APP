import { useEffect, useRef, useState, type ReactNode } from "react";

export interface RowAction {
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  destructive?: boolean;
  disabled?: boolean;
}

interface RowActionsProps {
  actions: RowAction[];
  label?: string;
}

export function RowActions({ actions, label = "Row actions" }: RowActionsProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const itemsRef = useRef<HTMLButtonElement[]>([]);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const focusItem = (index: number) => {
    const items = itemsRef.current.filter(Boolean);
    if (!items.length) return;
    const safe = ((index % items.length) + items.length) % items.length;
    items[safe].focus();
  };

  useEffect(() => {
    if (!open) return;
    const id = requestAnimationFrame(() => focusItem(0));
    return () => cancelAnimationFrame(id);
  }, [open]);

  const onTriggerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      setOpen((o) => !o);
    }
  };

  const onItemKeyDown = (e: React.KeyboardEvent, index: number) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      focusItem(index + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (index === 0) {
        setOpen(false);
        triggerRef.current?.focus();
      } else {
        focusItem(index - 1);
      }
    } else if (e.key === "Tab") {
      setOpen(false);
    }
  };

  return (
    <div className="row-actions" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="row-actions__trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={onTriggerKeyDown}
      >
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false">
          <circle cx="12" cy="5" r="1.6" fill="currentColor" />
          <circle cx="12" cy="12" r="1.6" fill="currentColor" />
          <circle cx="12" cy="19" r="1.6" fill="currentColor" />
        </svg>
      </button>
      {open ? (
        <div className="row-actions__menu" role="menu" aria-label={label}>
          {actions.map((a, i) => (
            <button
              key={a.label}
              type="button"
              ref={(el) => {
                if (el) itemsRef.current[i] = el;
              }}
              className={`row-actions__item${a.destructive ? " is-destructive" : ""}`}
              role="menuitem"
              disabled={a.disabled}
              onClick={() => {
                setOpen(false);
                a.onClick();
              }}
              onKeyDown={(e) => onItemKeyDown(e, i)}
            >
              {a.icon ? <span className="row-actions__icon" aria-hidden="true">{a.icon}</span> : null}
              <span>{a.label}</span>
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
