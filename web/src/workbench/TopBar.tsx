import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";

const fmtTime = () =>
  new Intl.DateTimeFormat("en-US", { hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date());

/** Top navigation: brand, system status, business identity, and sign out. */
export function TopBar() {
  const { user, business, setUser, setBusiness, setTransactions } = useAppState();
  const [clock, setClock] = useState(fmtTime());

  useEffect(() => {
    const id = window.setInterval(() => setClock(fmtTime()), 30 * 1000);
    return () => window.clearInterval(id);
  }, []);

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setBusiness(null);
    setTransactions([]);
    window.location.assign("/login");
  };

  const businessName = business?.name || user?.businessName || "Your business";

  return (
    <header className="topbar" role="banner">
      <div className="topbar__brand">
        <Link to="/" className="brand brand--inline">
          <span className="brand__mark" aria-hidden="true" />
          <span className="brand__name">Ledgerly</span>
        </Link>
      </div>

      <div className="topbar__indicators" aria-label="Application status">
        <span className="topbar__indicator">
          <span className="dot dot--pos" aria-hidden="true" />
          <span>Connected</span>
        </span>
        <span className="topbar__indicator topbar__indicator--name">{businessName}</span>
        <span className="topbar__indicator topbar__indicator--clock">{clock}</span>
      </div>

      <div className="topbar__actions">
        <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
          Sign out
        </button>
      </div>
    </header>
  );
}
