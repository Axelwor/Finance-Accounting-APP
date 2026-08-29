import { Component, lazy, Suspense, type ErrorInfo, type ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppStateProvider, useAppState } from "./state";
import { WorkbenchProvider } from "./workbench/state";
import { ToastProvider } from "./components/Toast";
import { Button } from "./components/m3";

// Route-level code splitting: the login/onboarding screens and the main
// workbench each load on first visit instead of shipping as one megabyte-
// plus bundle on the login page.
const AuthScreen = lazy(() =>
  import("./screens/AuthScreen").then((m) => ({ default: m.AuthScreen })),
);
const OnboardingScreen = lazy(() =>
  import("./screens/OnboardingScreen").then((m) => ({ default: m.OnboardingScreen })),
);
const AppShell = lazy(() =>
  import("./workbench/AppShell").then((m) => ({ default: m.AppShell })),
);

function RouteFallback() {
  return (
    <div className="app">
      <main className="app-main">
        <div className="app-main__inner">
          <p className="loading-state" role="status">
            <span className="loading-state__spinner" aria-hidden="true" />
            <span>Memuat modul...</span>
          </p>
        </div>
      </main>
    </div>
  );
}

function ShellRoute({ children }: { children: React.ReactNode }) {
  const { user, hydrating } = useAppState();
  if (hydrating) {
    return (
      <div className="app">
        <main className="app-main">
          <div className="app-main__inner">
            <p className="loading-state" role="status">
              <span className="loading-state__spinner" aria-hidden="true" />
              <span>Loading console...</span>
            </p>
          </div>
        </main>
      </div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function OnboardingRoute() {
  const { business } = useAppState();
  if (!business) return <Navigate to="/onboarding" replace />;
  return null;
}

/**
 * React error boundaries must be class components. This top-level
 * boundary catches any uncaught render error so the app never shows a
 * blank screen; it offers a reload button to recover.
 */
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean; message: string }
> {
  state = { hasError: false, message: "" };

  static getDerivedStateFromError(error: unknown) {
    return { hasError: true, message: error instanceof Error ? error.message : String(error) };
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) return this.props.children;
    return (
      <div className="app" role="alert">
        <main className="app-main">
          <div className="app-main__inner">
            <div className="error-state" style={{ maxWidth: 520, margin: "var(--md-sys-spacing-7) auto" }}>
              <h2 className="error-state__title">Something went wrong</h2>
              <p className="error-state__message">
                An unexpected application error occurred. The technical details were
                written to the browser console.
                <br />
                Reloading the app will restore your open tabs — unsaved form data may be lost.
              </p>
              <Button variant="filled" onClick={() => window.location.reload()}>
                Reload
              </Button>
            </div>
          </div>
        </main>
      </div>
    );
  }
}

/**
 * Per-tab boundary: a crash inside one tab's content renders an inline
 * error card instead of killing the whole workbench. "Try again" resets
 * the boundary and remounts just that tab.
 */
export class TabErrorBoundary extends Component<
  { children: ReactNode; title?: string },
  { hasError: boolean; message: string }
> {
  state = { hasError: false, message: "" };

  static getDerivedStateFromError(error: unknown) {
    return { hasError: true, message: error instanceof Error ? error.message : String(error) };
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error("[TabErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) return this.props.children;
    return (
      <div className="error-state" role="alert">
        <h3 className="error-state__title">{this.props.title ? `${this.props.title} crashed` : "This tab crashed"}</h3>
        <p className="error-state__message">
          An unexpected error occurred in this tab. Other tabs are unaffected.
          Technical details were written to the browser console.
        </p>
        <div className="quick-actions">
          <Button variant="filled" onClick={() => this.setState({ hasError: false, message: "" })}>
            Try again
          </Button>
          <Button variant="outlined" onClick={() => window.location.reload()}>
            Reload app
          </Button>
        </div>
      </div>
    );
  }
}

/** Skip-link: first focusable element on every page; styles are inline
 * because style files are owned by another agent. */
const SKIP_LINK_STYLE = `
.skip-link {
  position: absolute;
  left: -9999px;
  top: 0;
  width: 1px;
  height: 1px;
  overflow: hidden;
}
.skip-link:focus {
  left: 12px;
  top: 12px;
  width: auto;
  height: auto;
  z-index: 10000;
  background: var(--bg-surface, #fff);
  color: var(--text-primary, #111827);
  border: 2px solid var(--brand-primary, #2f80ed);
  border-radius: 8px;
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.18);
}
`;

export default function App() {
  return (
    <>
      <style>{SKIP_LINK_STYLE}</style>
      <a href="#app-main" className="skip-link">
        Lewati ke konten utama (Skip to main content)
      </a>
      <ErrorBoundary>
        <AppStateProvider>
          <ToastProvider>
          <WorkbenchProvider>
            <BrowserRouter>
              <Routes>
                <Route path="/login" element={<Suspense fallback={<RouteFallback />}><AuthScreen /></Suspense>} />
                <Route path="/onboarding" element={<Suspense fallback={<RouteFallback />}><OnboardingScreen /></Suspense>} />
                <Route
                  path="/"
                  element={
                    <ShellRoute>
                      <Suspense fallback={<RouteFallback />}>
                        <AppShell />
                      </Suspense>
                    </ShellRoute>
                  }
                >
                  <Route index element={<OnboardingRoute />} />
                  <Route path="*" element={<WorkbenchRedirect />} />
                </Route>
              </Routes>
            </BrowserRouter>
          </WorkbenchProvider>
          </ToastProvider>
        </AppStateProvider>
      </ErrorBoundary>
    </>
  );
}

/**
 * Fallback inside the shell: send any unknown path to onboarding if the
 * tenant isn't set up yet, otherwise land on the workbench (which shows
 * its empty state when no tabs are open).
 */
function WorkbenchRedirect() {
  const { business } = useAppState();
  if (!business) return <Navigate to="/onboarding" replace />;
  return null;
}
