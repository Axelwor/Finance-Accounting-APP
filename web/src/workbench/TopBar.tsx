import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { TenantSwitcher } from "./TenantSwitcher";

/**
 * Top navigation: brand, business identity (tenant switcher), and sign out.
 * Corporate Wave-style bar — no live clock or connection-status
 * dots that added visual noise.
 */
export function TopBar() {
  const { user, setUser, setBusiness, setTransactions } = useAppState();
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (localStorage.getItem("theme") as "light" | "dark") || "light"
  );

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

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
      <div className="topbar__brand">
        <Link to="/" className="brand brand--inline">
          <span className="brand__mark" aria-hidden="true" />
          <span className="brand__name">Ledgerly</span>
        </Link>
      </div>

      <div className="topbar__business" aria-label="Business identity">
        <TenantSwitcher />
      </div>

      <div className="topbar__actions">
        <button
          type="button"
          className="btn btn--ghost btn--sm"
          onClick={toggleTheme}
          aria-label={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
          title={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
        >
          {theme === "light" ? (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="5"/>
                <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/>
              </svg>
              Dark
            </>
          ) : (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
              </svg>
              Light
            </>
          )}
        </button>
        <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
          Sign out
        </button>
      </div>
    </header>
  );
}
