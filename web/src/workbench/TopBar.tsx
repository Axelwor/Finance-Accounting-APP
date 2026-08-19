import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { TenantSwitcher } from "./TenantSwitcher";
import { Icon } from "../components/m3/Icon";
import { ThemePicker } from "../components/ThemePicker";
import "@material/web/iconbutton/icon-button.js";

/**
 * Top navigation — M3 top app bar: brand, business identity (tenant
 * switcher), theme toggle, and sign out. Uses Material Symbols icons and
 * M3 token typography (title-large, on-surface).
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
        <ThemePicker />
        <md-icon-button
          onclick={toggleTheme}
          aria-label={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
          title={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
        >
          <Icon name={theme === "light" ? "dark_mode" : "light_mode"} />
        </md-icon-button>
        <md-icon-button onclick={handleLogout} aria-label="Sign out" title="Sign out">
          <Icon name="logout" />
        </md-icon-button>
      </div>
    </header>
  );
}
