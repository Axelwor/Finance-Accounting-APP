import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Button, FormError, TextField } from "../components/ui";

const wordmark = "Ledgerly";

/** Sign in / create account page (login & register in one flow). */
export function AuthScreen() {
  const [params] = useSearchParams();
  const registerMode = params.get("mode") === "register";
  const [mode, setMode] = useState<"login" | "register">(registerMode ? "register" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [businessName, setBusinessName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const navigate = useNavigate();
  const { setUser } = useAppState();

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
        const { user, hasTenant } = await api.login({ email, password });
        setUser(user);
        // Repeat logins with an existing tenant skip onboarding and go
        // straight to the dashboard; first-time logins need onboarding.
        navigate(hasTenant ? "/" : "/onboarding", { replace: true });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not reach the server. Try again.");
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
        <div className="auth__copy-meta">
          <span className="pos-dot" aria-hidden="true" />
          <span>No sync, no double-entry — one book</span>
        </div>
        <div className="auth__copy">
          <h1 className="auth__title">
            Double-entry, simple<span className="slash">.</span>
            <span> No accounting degree.</span>
          </h1>
          <p className="auth__lede">
            Money in, money out, transfers, and period close — written to one
            ledger your accountant trusts, ready for tax season.
          </p>
        </div>
        <div className="auth__signoff">
          <span><strong>Ledgerly</strong> &middot; bookkeeping for small business</span>
          <span className="auth__badge">M1 &middot; IDR</span>
        </div>
      </div>

      <div className="auth__panel">
        <form className="auth-card" onSubmit={handleSubmit} noValidate>
          <div className="auth-card__tabs" role="tablist" aria-label="Sign in or create account">
            <button
              type="button"
              role="tab"
              aria-selected={mode === "login"}
              className={`auth-card__tab${mode === "login" ? " is-active" : ""}`}
              onClick={() => switchMode("login")}
            >
              Sign in
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "register"}
              className={`auth-card__tab${mode === "register" ? " is-active" : ""}`}
              onClick={() => switchMode("register")}
            >
              Open book
            </button>
          </div>

          {mode === "register" ? (
            <div className="field">
              <label className="field__label" htmlFor="businessName">Business name</label>
              <input
                id="businessName"
                className="input"
                value={businessName}
                onChange={(e) => setBusinessName(e.target.value)}
                placeholder="e.g. Sari Corner Store"
                autoComplete="organization"
              />
            </div>
          ) : null}

          <div className="field">
            <label className="field__label" htmlFor="email">Email</label>
            <input
              id="email"
              className="input"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="name@example.com"
              autoComplete="email"
              inputMode="email"
            />
          </div>

          <div className="field">
            <label className="field__label" htmlFor="password">Password</label>
            <input
              id="password"
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
              autoComplete={mode === "register" ? "new-password" : "current-password"}
            />
          </div>

          <FormError message={error} />

          <Button type="submit" variant="primary" fullWidth disabled={loading}>
            {loading
              ? "Connecting..."
              : mode === "register"
                ? "Open the book"
                : "Sign in"}
          </Button>

          <p className="auth-card__switch">
            {mode === "login" ? (
              <>
                First time here?{" "}
                <button type="button" className="link-button" onClick={() => switchMode("register")}>
                  Open a new book
                </button>
              </>
            ) : (
              <>
                Already on the books?{" "}
                <button type="button" className="link-button" onClick={() => switchMode("login")}>
                  Sign back in
                </button>
              </>
            )}
          </p>

          <p className="auth-card__note">
            <Link to="/onboarding" className="link-inline">
              Try example data, no account needed
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
