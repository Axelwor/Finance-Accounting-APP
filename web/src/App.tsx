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

export default function App() {
  return (
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
