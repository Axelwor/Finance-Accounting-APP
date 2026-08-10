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

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setBusiness(null);
    setTransactions([]);
    window.location.assign("/login");
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
        <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
          Sign out
        </button>
      </div>
    </header>
  );
}
