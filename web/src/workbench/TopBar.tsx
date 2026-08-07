import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";

const wordmark = "Ledgerly";

const fmtTime = () =>
  new Intl.DateTimeFormat("en-US", { hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date());

const fmtSession = () =>
  `SES-${new Date().getFullYear()}${String(new Date().getMonth() + 1).padStart(2, "0")}${String(new Date().getDate()).padStart(2, "0")}-01`;

/**
 * Top navbar — fixed across the top of the app shell.
 * Brand mark + wordmark on the left, indicators cluster in the middle,
 * business name + logout on the right.
 */
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
          <span className="brand__name">{wordmark}</span>
        </Link>
      </div>

      <div className="topbar__indicators" aria-label="System indicators">
        <span className="topbar__indicator">
          <span className="dot dot--pos" aria-hidden="true" />
          <span>LIVE</span>
        </span>
        <span className="topbar__indicator topbar__indicator--name">{businessName}</span>
        <span className="topbar__indicator topbar__indicator--clock">CLOCK {clock}</span>
        <span className="topbar__indicator topbar__indicator--session">{fmtSession()}</span>
      </div>

      <div className="topbar__actions">
        <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
          Sign out
        </button>
      </div>
    </header>
  );
}
