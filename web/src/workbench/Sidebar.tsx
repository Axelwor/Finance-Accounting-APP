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
    case "ledger":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 3h14v18H5z" />
          <path d="M8 7h8" />
          <path d="M8 11h8" />
          <path d="M8 15h5" />
        </svg>
      );
  }
};

/**
 * Icon rail sidebar. Each module is a compact icon button.
 * On hover the module label appears as small text below the icon,
 * and a flyout submenu shows the module's sub-items to the right.
 * On mobile the rail becomes a slide-over with full-width rows.
 */
export function Sidebar() {
  const workbench = useWorkbench();
  const [hoveredModule, setHoveredModule] = useState<string | null>(null);
  const [pinnedModule, setPinnedModule] = useState<string | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const closeTimer = useRef<number | null>(null);

  // Close on Escape.
  useEffect(() => {
    if (!hoveredModule && !pinnedModule) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setHoveredModule(null);
        setPinnedModule(null);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [hoveredModule, pinnedModule]);

  const openModuleId = pinnedModule ?? hoveredModule;

  const handleEnter = (id: string) => {
    if (closeTimer.current) {
      window.clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
    setHoveredModule(id);
  };

  const handleLeave = () => {
    if (closeTimer.current) window.clearTimeout(closeTimer.current);
    closeTimer.current = window.setTimeout(() => {
      setHoveredModule(null);
    }, 200);
  };

  const handleSubClick = (sub: SubItem) => {
    workbench.openList(sub.openList);
    setPinnedModule(null);
    setHoveredModule(null);
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
      <aside className={`sidebar${mobileOpen ? " is-open" : ""}`} aria-label="Modules">
        {/* Brand mark at top of rail */}
        <div className="sidebar__brand">
          <span className="brand__mark" aria-hidden="true" />
        </div>

        {/* Icon rail */}
        <nav className="sidebar__rail" aria-label="Modules">
          {MODULES.map((mod) => {
            const isOpen = openModuleId === mod.id;
            return (
              <div
                key={mod.id}
                className={`rail-item${isOpen ? " is-open" : ""}`}
                onMouseEnter={() => handleEnter(mod.id)}
                onMouseLeave={handleLeave}
              >
                <button
                  type="button"
                  className="rail-item__btn"
                  aria-haspopup="menu"
                  aria-expanded={isOpen}
                  aria-label={mod.label}
                  onClick={() =>
                    setPinnedModule((cur) => (cur === mod.id ? null : mod.id))
                  }
                  onFocus={() => handleEnter(mod.id)}
                  onBlur={handleLeave}
                >
                  <span className="rail-item__icon">
                    <Icon name={mod.icon} />
                  </span>
                  <span className="rail-item__label">{mod.label}</span>
                </button>

                {/* Flyout submenu */}
                <div
                  className={`rail-flyout${isOpen ? " is-open" : ""}`}
                  role="menu"
                  onMouseEnter={() => handleEnter(mod.id)}
                  onMouseLeave={handleLeave}
                >
                  <p className="rail-flyout__title">{mod.label}</p>
                  <ul className="rail-flyout__items">
                    {mod.items.map((sub) => (
                      <li key={sub.id}>
                        <button
                          type="button"
                          role="menuitem"
                          className="rail-flyout__item"
                          onClick={() => handleSubClick(sub)}
                        >
                          <span>{sub.label}</span>
                          {sub.mockData ? (
                            <span className="rail-flyout__badge">Demo</span>
                          ) : null}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            );
          })}
        </nav>
      </aside>
    </>
  );
}
