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
      } else {
        const { user } = await api.login({ email, password });
        setUser(user);
      }
      navigate("/onboarding", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
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
          <h1 className="auth__title">Simple bookkeeping, no accounting degree needed.</h1>
          <p className="auth__lede">
            Record money in and out like writing in a notebook. Reports for taxes, banks, and
            investors are prepared automatically in the background.
          </p>
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
              Create account
            </button>
          </div>

          {mode === "register" ? (
            <TextField
              label="Business name"
              value={businessName}
              onChange={setBusinessName}
              placeholder="e.g. Sari Corner Store"
              autoComplete="organization"
            />
          ) : null}

          <TextField
            label="Email address"
            type="email"
            value={email}
            onChange={setEmail}
            placeholder="name@example.com"
            autoComplete="email"
            inputMode="email"
          />

          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={setPassword}
            placeholder="At least 6 characters"
            autoComplete={mode === "register" ? "new-password" : "current-password"}
          />

          <FormError message={error} />

          <Button type="submit" variant="primary" fullWidth disabled={loading}>
            {loading
              ? "Processing..."
              : mode === "register"
                ? "Create account"
                : "Sign in"}
          </Button>

          <p className="auth-card__switch">
            {mode === "login" ? (
              <>
                No account yet?{" "}
                <button type="button" className="link-button" onClick={() => switchMode("register")}>
                  Create one here
                </button>
              </>
            ) : (
              <>
                Already have an account?{" "}
                <button type="button" className="link-button" onClick={() => switchMode("login")}>
                  Sign in here
                </button>
              </>
            )}
          </p>

          <p className="auth-card__note">
            <Link to="/onboarding" className="link-inline">
              Continue as example without account
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
