import { useEffect, useRef, useState } from "react";
import { useWorkbench } from "./state";
import { MODULES } from "./modules";
import { Icon } from "../components/m3/Icon";
import type { Module, SubItem } from "./types";

/** Module icon mapping to clean SVG icon names */
const MODULE_ICON_NAMES: Record<Module["icon"], string> = {
  wallet: "wallet",
  sale: "sale",
  purchase: "purchase",
  box: "box",
  building: "building",
  report: "report",
  ledger: "ledger",
  factory: "factory",
  email: "email",
};

/**
 * High-Density Rail Sidebar (64px slim rail + Smooth Flyout Submenu).
 * 100% Pure SVG Icons — completely immune to web font ligature glitches.
 */
export function Sidebar() {
  const workbench = useWorkbench();
  const [hoveredModule, setHoveredModule] = useState<string | null>(null);
  const [pinnedModule, setPinnedModule] = useState<string | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const closeTimer = useRef<number | null>(null);

  const activeModuleId = workbench.tabs.find((t) => t.id === workbench.activeId)?.moduleId ?? null;

  // Close on Escape and return focus to the open module's trigger button.
  useEffect(() => {
    if (!hoveredModule && !pinnedModule) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setHoveredModule(null);
        setPinnedModule(null);
        if (closeTimer.current) {
          window.clearTimeout(closeTimer.current);
          closeTimer.current = null;
        }
        const openItem = document.querySelector(".rail-item.is-open");
        const trigger = openItem?.querySelector<HTMLElement>(".rail-item__btn");
        const active = document.activeElement;
        if (
          trigger &&
          openItem &&
          (active === trigger || (active instanceof Node && openItem.contains(active)))
        ) {
          trigger.focus();
        }
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
    }, 180);
  };

  // Focus moving anywhere inside the module block (trigger or flyout items)
  // keeps the flyout alive; only leaving the whole block starts the timer.
  const handleModuleBlur = (e: React.FocusEvent<HTMLDivElement>) => {
    if (e.relatedTarget && e.currentTarget.contains(e.relatedTarget as Node)) {
      if (closeTimer.current) {
        window.clearTimeout(closeTimer.current);
        closeTimer.current = null;
      }
      return;
    }
    handleLeave();
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
        <Icon name={mobileOpen ? "close" : "tune"} size={20} />
      </button>

      <div
        className={`sidebar-scrim${mobileOpen ? " is-visible" : ""}`}
        aria-hidden="true"
        onClick={() => setMobileOpen(false)}
      />

      <aside className={`sidebar${mobileOpen ? " is-open" : ""}`} aria-label="Modules">
        {/* Brand mark */}
        <div className="sidebar__brand" title="Trexo Accounting">
          <div className="brand-badge">
            <Icon name="book_open" size={20} className="text-white" />
          </div>
        </div>

        {/* Navigation Rail */}
        <nav className="sidebar__rail" aria-label="Modules">
          {MODULES.map((mod) => {
            const isOpen = openModuleId === mod.id;
            const isActive = activeModuleId === mod.id;
            const iconName = MODULE_ICON_NAMES[mod.icon] || "file_text";

            return (
              <div
                key={mod.id}
                className={`rail-item${isOpen ? " is-open" : ""}${isActive ? " is-active" : ""}`}
                onMouseEnter={() => handleEnter(mod.id)}
                onMouseLeave={handleLeave}
                onBlur={handleModuleBlur}
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
                >
                  <span className="rail-item__icon">
                    <Icon name={iconName} size={20} strokeWidth={2} />
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
                  <div className="rail-flyout__header">
                    <span className="rail-flyout__header-icon">
                      <Icon name={iconName} size={16} />
                    </span>
                    <p className="rail-flyout__title">{mod.label}</p>
                  </div>
                  <ul className="rail-flyout__items">
                    {mod.items.map((sub) => (
                      <li key={sub.id}>
                        <button
                          type="button"
                          role="menuitem"
                          className="rail-flyout__item"
                          onClick={() => handleSubClick(sub)}
                        >
                          <span className="rail-flyout__item-text">{sub.label}</span>
                          {sub.mockData ? (
                            <span className="rail-flyout__badge">Demo</span>
                          ) : (
                            <Icon name="chevron_right" size={14} className="rail-flyout__arrow" />
                          )}
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
