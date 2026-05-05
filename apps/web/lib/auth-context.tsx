"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError } from "./api";
import type { User } from "./types";

type Ctx = {
  user: User | null;
  loading: boolean;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
  setSessionToken: (token: string, user: User) => void;
};

const AuthContext = createContext<Ctx | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try {
      const u = await api<User>("/v1/me");
      setUser(u);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        setUser(null);
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        refresh,
        signOut: async () => {
          try {
            await api("/v1/auth/logout", { method: "POST" });
          } catch {
            // ignore
          }
          if (typeof window !== "undefined") {
            window.localStorage.removeItem("lunarleague_token");
          }
          setUser(null);
        },
        setSessionToken: (token, user) => {
          if (typeof window !== "undefined") {
            window.localStorage.setItem("lunarleague_token", token);
          }
          setUser(user);
        },
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
