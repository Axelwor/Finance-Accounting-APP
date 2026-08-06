import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import type { Transaction, Usaha } from "./types";

export interface SessionUser {
  id: string;
  email: string;
  namaUsaha: string;
}

export interface AppState {
  user: SessionUser | null;
  usaha: Usaha | null;
  transactions: Transaction[];
  /** true saat status awal sedang dibaca dari penyimpanan lokal. */
  hydrating: boolean;
  setUser: (user: SessionUser | null) => void;
  setUsaha: (usaha: Usaha | null) => void;
  setTransactions: (transactions: Transaction[]) => void;
}

const AppStateContext = createContext<AppState | null>(null);

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [usaha, setUsaha] = useState<Usaha | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [hydrating, setHydrating] = useState(true);

  useEffect(() => {
    const state = api.getLocalState();
    setUser(state.user);
    setUsaha(state.usaha);
    setTransactions(state.transactions);
    setHydrating(false);
  }, []);

  return (
    <AppStateContext.Provider
      value={{ user, usaha, transactions, hydrating, setUser, setUsaha, setTransactions }}
    >
      {children}
    </AppStateContext.Provider>
  );
}

export function useAppState(): AppState {
  const ctx = useContext(AppStateContext);
  if (!ctx) throw new Error("useAppState harus dipakai di dalam <AppStateProvider>.");
  return ctx;
}
