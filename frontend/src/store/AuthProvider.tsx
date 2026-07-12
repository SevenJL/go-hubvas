import { useEffect, type ReactNode } from 'react';
import { useAuthStore } from './authStore';

/** Initializes the server session while preserving the persisted user cache. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const initialize = useAuthStore(state => state.initialize);

  useEffect(() => {
    void initialize();
  }, [initialize]);

  return children;
}
