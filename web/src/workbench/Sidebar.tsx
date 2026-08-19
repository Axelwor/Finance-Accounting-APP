import { useEffect, useRef, useState } from "react";
import { useWorkbench } from "./state";
import { MODULES } from "./modules";
import { Icon as M3Icon } from "../components/m3/Icon";
import "@material/web/icon/icon.js";
import type { Module, SubItem } from "./types";

/** Legacy module icon key → Material Symbols ligature name. */
const MODULE_ICON_SYMBOLS: Record<Module["icon"], string> = {
  wallet: "account_balance_wallet",
  sale: "sell",
  purchase: "shopping_cart",
  box: "inventory_2",
  building: "apartment",
  report: "description",
  ledger: "menu_book",
  factory: "factory",
  email: "mail",
};

/** Module icon — Material Symbols ligature (replaces inline SVGs). */
function ModuleIcon({ name }: { name: Module["icon"] }) {
  return <M3Icon name={MODULE_ICON_SYMBOLS[name]} size={24} />;
}

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

  // The active module is whichever one owns the currently-active top tab
  // (M3 navigation rail highlights the active destination).
  const activeModuleId = workbench.tabs.find((t) => t.id === workbench.activeId)?.moduleId ?? null;

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
            const isActive = activeModuleId === mod.id;
            return (
              <div
                key={mod.id}
                className={`rail-item${isOpen ? " is-open" : ""}${isActive ? " is-active" : ""}`}
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
                    <ModuleIcon name={mod.icon} />
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
