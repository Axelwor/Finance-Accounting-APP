import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api, ApiError } from "../api";
import { useAppState } from "../state";
import { Icon } from "../components/m3/Icon";

const AUTH_ERROR_COPY: Record<string, string> = {
  EMAIL_EXISTS: "Email sudah terdaftar. Gunakan email lain atau masuk.",
  INVALID_CREDENTIALS: "Email atau kata sandi salah.",
};

function authErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return AUTH_ERROR_COPY[err.code] ?? err.message;
  }
  return "Tidak dapat terhubung ke server. Silakan coba lagi.";
}

export function AuthScreen() {
  const [params] = useSearchParams();
  const registerMode = params.get("mode") === "register";
  const [mode, setMode] = useState<"login" | "register">(registerMode ? "register" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [businessName, setBusinessName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const navigate = useNavigate();
  const { setUser, setBusiness } = useAppState();

  const switchMode = (m: "login" | "register") => {
    setMode(m);
    setError(null);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      if (mode === "register") {
        const { user } = await api.register({ email, password, businessName });
        setUser(user);
        navigate("/onboarding", { replace: true });
      } else {
        const { user, hasTenant, business } = await api.login({ email, password });
        setUser(user);
        if (business) setBusiness(business);
        navigate(hasTenant ? "/" : "/onboarding", { replace: true });
      }
    } catch (err) {
      setError(authErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-container">
      {/* Left Banner: Accounting Values */}
      <div className="auth-banner">
        <div className="auth-banner__header">
          <div className="auth-brand-logo">
            <Icon name="book_open" size={24} />
          </div>
          <span className="auth-brand-name">Ledgerly</span>
        </div>

        <div className="auth-banner__content">
          <div className="auth-badge">
            <Icon name="security" size={14} />
            <span>Sistem Akuntansi Enterprise Berbasis Standar PSAK</span>
          </div>

          <h1 className="auth-title">
            Pengelolaan Keuangan & Pembukuan Presisi Tinggi.
          </h1>

          <p className="auth-description">
            Otomatisasi jurnal berpasangan (*double-entry*), manajemen arus kas real-time,
            dan kepatuhan pajak siap audit untuk bisnis Anda.
          </p>

          <div className="auth-feature-list">
            <div className="auth-feature-item">
              <span className="auth-feature-check">
                <Icon name="check" size={14} />
              </span>
              <span>Jurnal otomatis real-time (Zero unposted transactions)</span>
            </div>
            <div className="auth-feature-item">
              <span className="auth-feature-check">
                <Icon name="check" size={14} />
              </span>
              <span>Proteksi integritas rantai audit (Hash-chain verified)</span>
            </div>
            <div className="auth-feature-item">
              <span className="auth-feature-check">
                <Icon name="check" size={14} />
              </span>
              <span>Multi-cabang & multi-termin pembayaran</span>
            </div>
          </div>
        </div>

        <div className="auth-banner__footer">
          <span>&copy; 2026 Ledgerly Financial Engine &bull; PSAK / EMKM Compliant</span>
        </div>
      </div>

      {/* Right Form Card */}
      <div className="auth-form-panel">
        <div className="auth-card-v2">
          <div className="auth-card-v2__header">
            <h2>{mode === "login" ? "Masuk ke Pembukuan" : "Daftarkan Bisnis Baru"}</h2>
            <p>
              {mode === "login"
                ? "Masukkan email dan kata sandi akun Anda untuk melanjutkan"
                : "Mulai pencatatan akuntansi profesional dalam hitungan detik"}
            </p>
          </div>

          <div className="auth-mode-switch">
            <button
              type="button"
              className={`auth-mode-btn${mode === "login" ? " is-active" : ""}`}
              aria-pressed={mode === "login"}
              onClick={() => switchMode("login")}
            >
              Masuk (Sign In)
            </button>
            <button
              type="button"
              className={`auth-mode-btn${mode === "register" ? " is-active" : ""}`}
              aria-pressed={mode === "register"}
              onClick={() => switchMode("register")}
            >
              Buka Buku Baru
            </button>
          </div>

          {error && (
            <div className="auth-error-alert" role="alert">
              <Icon name="error" size={16} />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="auth-form">
            {mode === "register" && (
              <div className="auth-field">
                <label htmlFor="businessName">Nama Perusahaan / Bisnis *</label>
                <div className="auth-input-box">
                  <Icon name="building" size={16} className="auth-input-icon" />
                  <input
                    id="businessName"
                    type="text"
                    required
                    autoComplete="organization"
                    value={businessName}
                    onChange={(e) => setBusinessName(e.target.value)}
                    placeholder="Contoh: PT Maju Bersama Makmur"
                  />
                </div>
              </div>
            )}

            <div className="auth-field">
              <label htmlFor="email">Alamat Email *</label>
              <div className="auth-input-box">
                <Icon name="mail" size={16} className="auth-input-icon" />
                <input
                  id="email"
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="akuntan@perusahaan.co.id"
                />
              </div>
            </div>

            <div className="auth-field">
              <label htmlFor="password">Kata Sandi (Password) *</label>
              <div className="auth-input-box">
                <Icon name="lock" size={16} className="auth-input-icon" />
                <input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  required
                  autoComplete={mode === "login" ? "current-password" : "new-password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Minimal 8 karakter"
                />
                <button
                  type="button"
                  className="auth-password-toggle"
                  onClick={() => setShowPassword(!showPassword)}
                  aria-label={showPassword ? "Sembunyikan password" : "Tampilkan password"}
                >
                  <Icon name={showPassword ? "visibility" : "visibility"} size={16} />
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="btn-auth-submit"
            >
              {loading ? (
                <span>Memproses...</span>
              ) : mode === "login" ? (
                <>
                  <span>Masuk ke Dashboard</span>
                  <Icon name="arrow_forward" size={16} />
                </>
              ) : (
                <>
                  <span>Buat Buku Perusahaan</span>
                  <Icon name="arrow_forward" size={16} />
                </>
              )}
            </button>
          </form>

          <div className="auth-card-footer">
            <Link to="/onboarding" className="auth-demo-link">
              <Icon name="open_in_new" size={14} />
              <span>Jelajahi Demo Interaktif (Tanpa Akun)</span>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
