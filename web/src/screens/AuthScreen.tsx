import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Button, FormError, TextField } from "../components/ui";

const wordmark = "Pembukuan Mudah";

/** Halaman masuk / daftar (login & register dalam satu alur). */
export function AuthScreen() {
  const [params] = useSearchParams();
  const modeDaftar = params.get("mode") === "register";
  const [mode, setMode] = useState<"login" | "register">(modeDaftar ? "register" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [namaUsaha, setNamaUsaha] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const navigate = useNavigate();
  const { setUser } = useAppState();

  const gantiMode = (m: "login" | "register") => {
    setMode(m);
    setError(null);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      if (mode === "register") {
        const { user } = await api.register({ email, password, namaUsaha });
        setUser(user);
      } else {
        const { user } = await api.login({ email, password });
        setUser(user);
      }
      navigate("/onboarding", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Terjadi kesalahan. Coba lagi.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth">
      <div className="auth__hero">
        <div className="auth__brand">
          <span className="brand__mark" aria-hidden="true" />
          <span className="brand__name">{wordmark}</span>
        </div>
        <div className="auth__hero-copy">
          <h1 className="auth__title">Pembukuan mudah, tanpa perlu paham akuntansi.</h1>
          <p className="auth__lede">
            Catat uang masuk dan keluar seperti menulis di buku. Laporan untuk pajak, bank, dan
            investor tersusun otomatis di belakang layar.
          </p>
        </div>
      </div>

      <div className="auth__panel">
        <form className="auth-card" onSubmit={handleSubmit} noValidate>
          <div className="auth-card__tabs" role="tablist" aria-label="Masuk atau daftar">
            <button
              type="button"
              role="tab"
              aria-selected={mode === "login"}
              className={`auth-card__tab${mode === "login" ? " is-active" : ""}`}
              onClick={() => gantiMode("login")}
            >
              Masuk
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "register"}
              className={`auth-card__tab${mode === "register" ? " is-active" : ""}`}
              onClick={() => gantiMode("register")}
            >
              Daftar
            </button>
          </div>

          {mode === "register" ? (
            <TextField
              label="Nama usaha"
              value={namaUsaha}
              onChange={setNamaUsaha}
              placeholder="mis. Warung Bu Sari"
              autoComplete="organization"
            />
          ) : null}

          <TextField
            label="Alamat email"
            type="email"
            value={email}
            onChange={setEmail}
            placeholder="nama@contoh.com"
            autoComplete="email"
            inputMode="email"
          />

          <TextField
            label="Kata sandi"
            type="password"
            value={password}
            onChange={setPassword}
            placeholder="Minimal 6 karakter"
            autoComplete={mode === "register" ? "new-password" : "current-password"}
          />

          <FormError message={error} />

          <Button type="submit" variant="primary" fullWidth disabled={loading}>
            {loading
              ? "Memproses..."
              : mode === "register"
                ? "Buat akun"
                : "Masuk"}
          </Button>

          <p className="auth-card__switch">
            {mode === "login" ? (
              <>
                Belum punya akun?{" "}
                <button type="button" className="link-button" onClick={() => gantiMode("register")}>
                  Daftar di sini
                </button>
              </>
            ) : (
              <>
                Sudah punya akun?{" "}
                <button type="button" className="link-button" onClick={() => gantiMode("login")}>
                  Masuk di sini
                </button>
              </>
            )}
          </p>

          <p className="auth-card__note">
            <Link to="/onboarding" className="link-inline">
              Lanjut sebagai contoh tanpa akun
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
