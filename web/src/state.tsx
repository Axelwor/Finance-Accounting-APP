import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import type { Business, Transaction } from "./types";

export interface SessionUser {
  id: string;
  email: string;
  businessName: string;
}

export interface AppState {
  user: SessionUser | null;
  business: Business | null;
  transactions: Transaction[];
  /** true while initial state is being read from local storage. */
  hydrating: boolean;
  setUser: (user: SessionUser | null) => void;
  setBusiness: (business: Business | null) => void;
  setTransactions: (transactions: Transaction[]) => void;
}

const AppStateContext = createContext<AppState | null>(null);

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [business, setBusiness] = useState<Business | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [hydrating, setHydrating] = useState(true);

  useEffect(() => {
    const state = api.getLocalState();
    setUser(state.user);
    setBusiness(state.business);
    setTransactions(state.transactions);
    setHydrating(false);
  }, []);

  return (
    <AppStateContext.Provider
      value={{ user, business, transactions, hydrating, setUser, setBusiness, setTransactions }}
    >
      {children}
    </AppStateContext.Provider>
  );
}

export function useAppState(): AppState {
  const ctx = useContext(AppStateContext);
  if (!ctx) throw new Error("useAppState must be used inside <AppStateProvider>.");
  return ctx;
}
