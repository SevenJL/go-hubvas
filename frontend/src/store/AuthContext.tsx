import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import type { User } from '../types';
import { authService } from '../services/auth';
import { clearTokens, getAccessToken } from '../services/api';

interface AuthState {
  /** The currently authenticated user, or null if not logged in. */
  user: User | null;
  /** True while the initial auth check is in progress. */
  loading: boolean;
  /** Last auth error message (cleared on successful operations). */
  error: string | null;
  /** Login with email and password. Throws on failure. */
  login: (email: string, password: string) => Promise<void>;
  /** Register a new account. Automatically logs in on success. Throws on failure. */
  register: (username: string, email: string, password: string) => Promise<void>;
  /** Update the current user state (for profile changes). */
  setUser: (user: User) => void;
  /** Log out and clear all tokens. */
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(() => Boolean(getAccessToken()));
  const [error, setError] = useState<string | null>(null);

  // On mount, check if there's a stored token and validate it.
  useEffect(() => {
    const token = getAccessToken();
    if (!token) return;

    let cancelled = false;

    authService.me()
      .then(u => {
        if (!cancelled) setUser(u);
      })
      .catch(() => {
        // Token is expired or invalid — clear it.
        if (!cancelled) clearTokens();
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    setError(null);
    await authService.login({ email, password });
    const u = await authService.me();
    setUser(u);
  }, []);

  const register = useCallback(async (username: string, email: string, password: string) => {
    setError(null);
    // The backend now returns both user and tokens in one call.
    const result = await authService.register({ username, email, password });
    // User is already in the response; no need for a separate /me call.
    setUser(result.user);
  }, []);

  const logout = useCallback(() => {
    clearTokens();
    setUser(null);
    setError(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, error, login, register, setUser, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
}
