"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { api, type SessionUser } from "@/lib/api";

interface SessionContextValue {
  user: SessionUser | null;
  loading: boolean;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue>({
  user: null,
  loading: true,
  refresh: async () => {},
  signOut: async () => {},
});

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const res = await api.me();
      setUser(res.user);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // Even if the server call fails, clear local state so the UI flips back.
    }
    setUser(null);
    await refresh();
  }, [refresh]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return <SessionContext.Provider value={{ user, loading, refresh, signOut }}>{children}</SessionContext.Provider>;
}

export function useSession() {
  return useContext(SessionContext);
}

// signInWithGoogle bounces the browser to the trading-server's SSO
// start endpoint, which generates CSRF state + redirects to the
// centralized auth service (auth.jaycebordelon.com). Named for the
// user-facing provider — the transport is OAuth via our own auth
// service, and Google is the upstream IdP.
export function signInWithGoogle(returnTo?: string) {
  const target = returnTo ?? (typeof window !== "undefined" ? window.location.pathname + window.location.search : "/");
  window.location.assign(`/auth/sso/start?return_to=${encodeURIComponent(target)}`);
}
