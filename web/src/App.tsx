import { Component, type ErrorInfo, type ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppStateProvider, useAppState } from "./state";
import { WorkbenchProvider } from "./workbench/state";
import { ToastProvider } from "./components/Toast";
import { AppShell } from "./workbench/AppShell";
import { AuthScreen } from "./screens/AuthScreen";
import { OnboardingScreen } from "./screens/OnboardingScreen";

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
            <div className="error-state" style={{ maxWidth: 520, margin: "var(--u-7) auto" }}>
              <h2 className="error-state__title">Something went wrong</h2>
              <p className="error-state__message">
                An unexpected error occurred{this.state.message ? `: ${this.state.message}` : "."}
                <br />
                Reloading the app will restore your open tabs — unsaved form data may be lost.
              </p>
              <button type="button" className="btn btn--primary" onClick={() => window.location.reload()}>
                Reload
              </button>
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
          {this.state.message || "An unexpected error occurred in this tab."} Other tabs are unaffected.
        </p>
        <div className="quick-actions">
          <button type="button" className="btn btn--primary" onClick={() => this.setState({ hasError: false, message: "" })}>
            Try again
          </button>
          <button type="button" className="btn btn--secondary" onClick={() => window.location.reload()}>
            Reload app
          </button>
        </div>
      </div>
    );
  }
}

export default function App() {
  return (
    <ErrorBoundary>
      <AppStateProvider>
        <ToastProvider>
        <WorkbenchProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<AuthScreen />} />
              <Route path="/onboarding" element={<OnboardingScreen />} />
              <Route
                path="/"
                element={
                  <ShellRoute>
                    <AppShell />
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
