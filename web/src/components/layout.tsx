import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAppState } from "../state";
import { api } from "../api";

const wordmark = "Ledgerly";

const NAV_ITEMS = [
  { to: "/dashboard", label: "Dashboard" },
  { to: "/transactions", label: "Transactions" },
  { to: "/record/money-in", label: "Money In" },
  { to: "/record/money-out", label: "Money Out" },
  { to: "/record/transfer", label: "Transfer" },
];

/** Brand mark shown in the sidebar and auth/onboarding headers. */
export function Brand() {
  return (
    <Link to="/" className="brand" aria-label={`${wordmark} - back to home`}>
      <span className="brand__mark" aria-hidden="true" />
      <span className="brand__name">{wordmark}</span>
    </Link>
  );
}

/** Main navigation shell for signed-in users (left sidebar). */
export function AppShell() {
  const { user, business, setUser, setBusiness, setTransactions } = useAppState();
  const navigate = useNavigate();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    document.title = business ? `${business.name} - ${wordmark}` : `${wordmark}`;
    return () => {
      document.title = wordmark;
    };
  }, [business]);

  const closeSidebar = () => setSidebarOpen(false);

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setBusiness(null);
    setTransactions([]);
    navigate("/login");
  };

  return (
    <div className="app-shell">
      <button
        type="button"
        className="app-sidebar__toggle"
        aria-expanded={sidebarOpen}
        aria-label={sidebarOpen ? "Close menu" : "Open menu"}
        onClick={() => setSidebarOpen((open) => !open)}
      >
        ☰
      </button>
      <div
        className={`app-sidebar__scrim${sidebarOpen ? " is-visible" : ""}`}
        aria-hidden="true"
        onClick={closeSidebar}
      />
      <aside className={`app-sidebar${sidebarOpen ? " is-open" : ""}`}>
        <div className="app-sidebar__brand">
          <Brand />
        </div>
        <nav className="app-sidebar__nav" aria-label="Main menu">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => `app-sidebar__link${isActive ? " is-active" : ""}`}
              onClick={closeSidebar}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="app-sidebar__footer">
          <span className="app-sidebar__user" title={user?.email}>
            {user?.businessName}
          </span>
          <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="app-main">
        <div className="app-main__inner">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
