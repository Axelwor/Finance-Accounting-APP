import { useState, useEffect, useRef } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { useWorkbench } from "./state";
import { TenantSwitcher } from "./TenantSwitcher";
import { Icon } from "../components/m3/Icon";
import { ThemePicker } from "../components/ThemePicker";

/**
 * TopBar: Top navigation header with business selector, quick entry shortcut,
 * theme toggle, and account actions.
 */
export function TopBar({ onOpenPalette }: { onOpenPalette?: () => void }) {
  const { user, setUser, setBusiness, setTransactions } = useAppState();
  const workbench = useWorkbench();
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (localStorage.getItem("theme") as "light" | "dark") || "light"
  );
  const [quickMenuOpen, setQuickMenuOpen] = useState(false);
  const quickTriggerRef = useRef<HTMLButtonElement>(null);
  const quickMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("theme", theme);
  }, [theme]);

  // Quick menu: move focus into the menu on open, close on Escape (restoring
  // focus to the trigger), and restore focus after any close.
  useEffect(() => {
    if (!quickMenuOpen) return;
    const focusTimer = window.setTimeout(() => {
      quickMenuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    }, 0);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        setQuickMenuOpen(false);
        quickTriggerRef.current?.focus();
      } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        const items = Array.from(
          quickMenuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [],
        );
        if (items.length === 0) return;
        e.preventDefault();
        const idx = items.indexOf(document.activeElement as HTMLElement);
        const next =
          e.key === "ArrowDown"
            ? items[(idx + 1) % items.length]
            : items[(idx - 1 + items.length) % items.length];
        next.focus();
      }
    };
    document.addEventListener("keydown", onKey, true);
    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", onKey, true);
      // Any close path (backdrop click, item click) returns focus to the trigger
      // when nothing else grabbed it while the dropdown unmounted.
      const active = document.activeElement;
      if (!active || active === document.body) quickTriggerRef.current?.focus();
    };
  }, [quickMenuOpen]);

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setBusiness(null);
    setTransactions([]);
    window.location.assign("/login");
  };

  const toggleTheme = () => {
    const newTheme = theme === "light" ? "dark" : "light";
    setTheme(newTheme);
  };

  return (
    <header className="topbar" role="banner">
      <div className="topbar__left">
        <Link to="/" className="brand brand--inline" aria-label="Ledgerly Home">
          <span className="brand__logo-icon">
            <Icon name="book_open" size={18} />
          </span>
          <span className="brand__name">Ledgerly</span>
          <span className="brand__badge">v2.0</span>
        </Link>

        <div className="topbar__divider" />

        <div className="topbar__business" aria-label="Business identity">
          <TenantSwitcher />
        </div>

        {/* Global Search / Command Palette Trigger */}
        <button
          type="button"
          className="topbar__search-btn"
          onClick={onOpenPalette}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "8px",
            padding: "4px 12px",
            backgroundColor: "var(--bg-surface-secondary)",
            border: "1px solid var(--border-color)",
            borderRadius: "var(--radius-sm)",
            fontSize: "12px",
            color: "var(--text-muted)",
            cursor: "pointer",
            transition: "all 0.15s ease",
          }}
          title="Buka Command Palette (Ctrl+K / Cmd+K)"
        >
          <Icon name="search" size={14} />
          <span>Cari modul & perintah…</span>
          <span
            style={{
              fontSize: "10px",
              fontFamily: "var(--font-mono)",
              padding: "1px 4px",
              backgroundColor: "var(--bg-surface-tertiary)",
              borderRadius: "var(--radius-xs)",
              border: "1px solid var(--border-color)",
            }}
          >
            ⌘K
          </span>
        </button>
      </div>

      <div className="topbar__center">
        {/* Quick Action Dropdown */}
        <div className="topbar__quick-action">
          <button
            ref={quickTriggerRef}
            type="button"
            className="btn-quick-add"
            onClick={() => setQuickMenuOpen(!quickMenuOpen)}
            aria-expanded={quickMenuOpen}
            aria-haspopup="menu"
          >
            <Icon name="plus" size={16} />
            <span>+ Buat Baru</span>
            <Icon name="chevron_down" size={14} />
          </button>

          {quickMenuOpen && (
            <>
              <div
                className="quick-menu-backdrop"
                onClick={() => setQuickMenuOpen(false)}
              />
              <div ref={quickMenuRef} className="quick-menu-dropdown" role="menu" aria-label="Buat dokumen baru">
                <div className="quick-menu-group">
                  <span className="quick-menu-group-label">Kas & Bank</span>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("money-in");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="arrow_down_left" size={16} className="text-success" />
                    <span>Kas Masuk (Other Receipt)</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("money-out");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="arrow_up_right" size={16} className="text-danger" />
                    <span>Kas Keluar (Other Payment)</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("cash-transfer");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="refresh" size={16} />
                    <span>Transfer Bank</span>
                  </button>
                </div>

                <div className="quick-menu-group">
                  <span className="quick-menu-group-label">Penjualan & Pembelian</span>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("sales-invoice");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="receipt" size={16} />
                    <span>Faktur Penjualan (Invoice)</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("purchase-invoice");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="shopping_cart" size={16} />
                    <span>Faktur Pembelian</span>
                  </button>
                </div>

                <div className="quick-menu-group">
                  <span className="quick-menu-group-label">Buku Besar</span>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      workbench.openEntryDraft("journal-entry");
                      setQuickMenuOpen(false);
                    }}
                  >
                    <Icon name="book_open" size={16} />
                    <span>Jurnal Umum Manual</span>
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      <div className="topbar__actions">
        <ThemePicker />

        <button
          type="button"
          className="topbar__icon-btn"
          onClick={toggleTheme}
          aria-label={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
          title={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
        >
          <Icon name={theme === "light" ? "dark_mode" : "light_mode"} size={18} />
        </button>

        <div className="topbar__user-pill" title={user?.email || "User profile"}>
          <span className="topbar__user-avatar">
            <Icon name="person" size={14} />
          </span>
          <span className="topbar__user-email">{user?.email || "User"}</span>
        </div>

        <button
          type="button"
          className="topbar__icon-btn topbar__icon-btn--logout"
          onClick={handleLogout}
          aria-label="Sign out"
          title="Sign out"
        >
          <Icon name="logout" size={18} />
        </button>
      </div>
    </header>
  );
}
