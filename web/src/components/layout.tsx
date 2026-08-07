import { useEffect, useState, type ReactNode } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAppState } from "../state";
import { api } from "../api";

const wordmark = "Ledgerly";

const Icon = ({ children }: { children: ReactNode }) => (
  <svg viewBox="0 0 24 24" aria-hidden="true">{children}</svg>
);

const NAV_OVERVIEW = [
  {
    to: "/dashboard",
    label: "Today",
    hint: "T",
    icon: (
      <Icon>
        <rect x="3" y="4" width="18" height="17" rx="1.5" />
        <path d="M3 9h18" />
        <path d="M8 2v4" />
        <path d="M16 2v4" />
      </Icon>
    ),
  },
  {
    to: "/transactions",
    label: "Ledger",
    hint: "L",
    icon: (
      <Icon>
        <path d="M4 5h16" />
        <path d="M4 12h16" />
        <path d="M4 19h16" />
        <path d="M8 5v14" />
      </Icon>
    ),
  },
];

const NAV_RECORD = [
  {
    to: "/record/money-in",
    label: "Money in",
    hint: "M+",
    icon: (
      <Icon>
        <path d="M12 4v12" />
        <path d="M6 10l6-6 6 6" />
        <path d="M5 20h14" />
      </Icon>
    ),
  },
  {
    to: "/record/money-out",
    label: "Money out",
    hint: "M-",
    icon: (
      <Icon>
        <path d="M12 16V4" />
        <path d="M6 10l6 6 6-6" />
        <path d="M5 20h14" />
      </Icon>
    ),
  },
  {
    to: "/record/transfer",
    label: "Transfer",
    hint: "Xfer",
    icon: (
      <Icon>
        <path d="M4 8h13l-3-3" />
        <path d="M20 16H7l3 3" />
      </Icon>
    ),
  },
];

/** Brand mark for the sidebar and auth screens. */
export function Brand() {
  return (
    <Link to="/" className="brand" aria-label={`${wordmark} - back to home`}>
      <span className="brand__mark" aria-hidden="true" />
      <span className="brand__name">{wordmark}</span>
    </Link>
  );
}

/** Main navigation shell for signed-in users (slim dark sidebar). */
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
        <Icon>
          <path d="M4 6h16" />
          <path d="M4 12h16" />
          <path d="M4 18h16" />
        </Icon>
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
          <SidebarGroup label="Console" items={NAV_OVERVIEW} onSelect={closeSidebar} />
          <SidebarGroup label="Record" items={NAV_RECORD} onSelect={closeSidebar} />
        </nav>
        <div className="app-sidebar__footer">
          <div className="app-sidebar__user">
            <span>Signed in</span>
            <strong>{user?.businessName}</strong>
          </div>
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

interface SidebarItem {
  to: string;
  label: string;
  hint: string;
  icon: ReactNode;
}

function SidebarGroup({
  label,
  items,
  onSelect,
}: {
  label: string;
  items: SidebarItem[];
  onSelect: () => void;
}) {
  return (
    <div className="sidebar-group">
      <p className="sidebar-group__label">{label}</p>
      <div className="sidebar-group__items">
        {items.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) => `app-sidebar__link${isActive ? " is-active" : ""}`}
            onClick={onSelect}
          >
            {item.icon}
            <span>{item.label}</span>
            <span className="app-sidebar__link__meta">{item.hint}</span>
          </NavLink>
        ))}
      </div>
    </div>
  );
}
