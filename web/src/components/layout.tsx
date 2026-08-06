import { useEffect } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAppState } from "../state";
import { api } from "../api";

const wordmark = "Pembukuan Mudah";

/** Judul aplikasi di bilah atas (halaman yang sudah masuk). */
export function Brand() {
  return (
    <Link to="/" className="brand" aria-label={`${wordmark} - kembali ke beranda`}>
      <span className="brand__mark" aria-hidden="true" />
      <span className="brand__name">{wordmark}</span>
    </Link>
  );
}

const NAV_ITEMS = [
  { to: "/dashboard", label: "Ringkasan" },
  { to: "/transaksi", label: "Catatan" },
  { to: "/catat/uang-masuk", label: "Uang Masuk" },
  { to: "/catat/uang-keluar", label: "Uang Keluar" },
  { to: "/catat/pindah-uang", label: "Pindah Uang" },
];

/** Bilah navigasi utama untuk pengguna yang sudah masuk. */
export function AppShell() {
  const { user, usaha, setUser, setUsaha, setTransactions } = useAppState();
  const navigate = useNavigate();

  useEffect(() => {
    document.title = usaha ? `${usaha.nama} - ${wordmark}` : `${wordmark}`;
    return () => {
      document.title = wordmark;
    };
  }, [usaha]);

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setUsaha(null);
    setTransactions([]);
    navigate("/login");
  };

  return (
    <div className="app">
      <header className="app-bar">
        <div className="app-bar__inner">
          <Brand />
          <nav className="app-nav" aria-label="Menu utama">
            {NAV_ITEMS.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) => `app-nav__link${isActive ? " is-active" : ""}`}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="app-bar__actions">
            <span className="app-bar__user" title={user?.email}>
              {user?.namaUsaha}
            </span>
            <button type="button" className="btn btn--ghost btn--sm" onClick={handleLogout}>
              Keluar
            </button>
          </div>
        </div>
      </header>
      <main className="app-main">
        <div className="app-main__inner">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
