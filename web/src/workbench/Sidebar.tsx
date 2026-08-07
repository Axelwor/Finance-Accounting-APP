import { useEffect, useRef, useState } from "react";
import { useWorkbench } from "./state";
import { MODULES } from "./modules";
import type { Module, SubItem } from "./types";

const Icon = ({ name }: { name: Module["icon"] }) => {
  switch (name) {
    case "wallet":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3" y="6" width="18" height="13" rx="2" />
          <path d="M16 12h2" />
          <path d="M3 10h18" />
        </svg>
      );
    case "sale":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 12l4 4L19 6" />
          <path d="M9 6h6" />
          <path d="M9 18h6" />
        </svg>
      );
    case "purchase":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 7h16" />
          <path d="M5 7l1 12h12l1-12" />
          <path d="M9 11h6" />
        </svg>
      );
    case "box":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M3 7l9-4 9 4-9 4-9-4z" />
          <path d="M3 7v10l9 4 9-4V7" />
          <path d="M12 11v10" />
        </svg>
      );
    case "building":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4" y="3" width="16" height="18" />
          <path d="M8 7h2" />
          <path d="M14 7h2" />
          <path d="M8 11h2" />
          <path d="M14 11h2" />
          <path d="M8 15h2" />
          <path d="M14 15h2" />
          <path d="M10 21v-3h4v3" />
        </svg>
      );
    case "report":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 4h12l3 3v13H5z" />
          <path d="M5 4v14" />
          <path d="M9 11v6" />
          <path d="M13 8v9" />
          <path d="M17 14v3" />
        </svg>
      );
  }
};

/**
 * Left sidebar with module tree. Hover or focus on a module row reveals
 * its sub-item popup on the right (desktop) or expands in place (mobile).
 * Sub-items open list tabs in the work area.
 */
export function Sidebar() {
  const workbench = useWorkbench();
  const [openModule, setOpenModule] = useState<string | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Close popup on click outside or Escape.
  useEffect(() => {
    if (!openModule) return;
    const onPointer = (e: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpenModule(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpenModule(null);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [openModule]);

  const handleSubClick = (sub: SubItem) => {
    workbench.openList(sub.openList);
    setOpenModule(null);
    setMobileOpen(false);
  };

  return (
    <>
      <button
        type="button"
        className="sidebar-toggle"
        aria-expanded={mobileOpen}
        aria-label={mobileOpen ? "Close menu" : "Open menu"}
        onClick={() => setMobileOpen((v) => !v)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 6h16" />
          <path d="M4 12h16" />
          <path d="M4 18h16" />
        </svg>
      </button>
      <div
        className={`sidebar-scrim${mobileOpen ? " is-visible" : ""}`}
        aria-hidden="true"
        onClick={() => setMobileOpen(false)}
      />
      <aside ref={rootRef} className={`sidebar${mobileOpen ? " is-open" : ""}`} aria-label="Modules">
        <div className="sidebar__brand">
          <span className="sidebar__brand-label">Console</span>
        </div>
        <nav className="sidebar__nav">
          {MODULES.map((mod) => (
            <div
              key={mod.id}
              className={`sidebar-module${openModule === mod.id ? " is-open" : ""}${mobileOpen && openModule === mod.id ? " is-mobile-open" : ""}`}
              onMouseEnter={() => setOpenModule(mod.id)}
              onMouseLeave={() => setOpenModule((current) => (current === mod.id && !mobileOpen ? null : current))}
            >
              <button
                type="button"
                className="sidebar-module__trigger"
                aria-haspopup="menu"
                aria-expanded={openModule === mod.id}
                onFocus={() => setOpenModule(mod.id)}
                onClick={() => setOpenModule((current) => (current === mod.id ? null : mod.id))}
              >
                <span className="sidebar-module__icon" aria-hidden="true">
                  <Icon name={mod.icon} />
                </span>
                <span className="sidebar-module__label">{mod.label}</span>
                <span className="sidebar-module__count">{mod.items.length}</span>
                <span className="sidebar-module__chevron" aria-hidden="true">›</span>
              </button>
              <div className="sidebar-module__popup" role="menu">
                <p className="sidebar-module__popup-title">{mod.label}</p>
                <ul className="sidebar-module__items">
                  {mod.items.map((sub) => (
                    <li key={sub.id}>
                      <button
                        type="button"
                        role="menuitem"
                        className="sidebar-subitem"
                        onClick={() => handleSubClick(sub)}
                      >
                        <span className="sidebar-subitem__label">{sub.label}</span>
                        {sub.hint ? <span className="sidebar-subitem__hint">{sub.hint}</span> : null}
                        {sub.mockData ? <span className="sidebar-subitem__mock">DEMO</span> : null}
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          ))}
        </nav>
      </aside>
    </>
  );
}
