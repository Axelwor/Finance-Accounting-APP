import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppStateProvider, useAppState } from "./state";
import { AppShell } from "./components/layout";
import { AuthScreen } from "./screens/AuthScreen";
import { OnboardingScreen } from "./screens/OnboardingScreen";
import { DashboardScreen } from "./screens/DashboardScreen";
import { TransactionFormScreen } from "./screens/TransactionFormScreen";
import { TransactionsScreen } from "./screens/TransactionsScreen";

function ShellRoute({ children }: { children: React.ReactNode }) {
  const { user, hydrating } = useAppState();
  if (hydrating) {
    return (
      <div className="app">
        <main className="app-main">
          <div className="app-main__inner">
            <p className="loading-state" role="status">
              <span className="loading-state__spinner" aria-hidden="true" />
              <span>Loading...</span>
            </p>
          </div>
        </main>
      </div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function DashboardRoute() {
  const { business } = useAppState();
  if (!business) return <Navigate to="/onboarding" replace />;
  return <DashboardScreen />;
}

function TransactionsRoute() {
  const { business } = useAppState();
  if (!business) return <Navigate to="/onboarding" replace />;
  return <TransactionsScreen />;
}

function TransactionFormRoute() {
  const { business } = useAppState();
  if (!business) return <Navigate to="/onboarding" replace />;
  return <TransactionFormScreen />;
}

export default function App() {
  return (
    <AppStateProvider>
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
            <Route index element={<DashboardRoute />} />
            <Route path="dashboard" element={<DashboardRoute />} />
            <Route path="transactions" element={<TransactionsRoute />} />
            <Route path="record/:kindParam" element={<TransactionFormRoute />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AppStateProvider>
  );
}
