import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';
import type { User } from '../types';
import { AuthRequestError, authService } from '../services/auth';
import { clearTokens } from '../services/api';

export interface AuthState {
  /** The last server-validated user, restored from local storage during reloads. */
  user: User | null;
  /** True while the HttpOnly refresh session is being validated. */
  loading: boolean;
  /** Last authentication error message. */
  error: string | null;
  login: (email: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  setUser: (user: User) => void;
  logout: () => void;
  initialize: () => Promise<void>;
}

type PersistedAuthState = Pick<AuthState, 'user'>;

let initialization: Promise<void> | null = null;
let authRevision = 0;

function isRejectedSession(error: unknown): boolean {
  return error instanceof AuthRequestError && [401, 403, 404].includes(error.status);
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      loading: true,
      error: null,

      initialize: async () => {
        if (initialization) return initialization;

        set({ loading: true, error: null });
        const revision = authRevision;
        const task = (async (): Promise<void> => {
          try {
            const user = await authService.me();
            if (revision === authRevision) set({ user, error: null });
          } catch (error) {
            clearTokens();
            if (revision !== authRevision) return;
            if (isRejectedSession(error)) {
              set({ user: null, error: null });
              return;
            }
            // A temporary network/server failure must not erase the persisted
            // user cache. API authorization still remains server-enforced.
            set({ error: error instanceof Error ? error.message : 'Could not validate session' });
          } finally {
            set({ loading: false });
            initialization = null;
          }
        })();
        initialization = task;
        return task;
      },

      login: async (email, password) => {
        authRevision += 1;
        set({ error: null });
        try {
          await authService.login({ email, password });
          const user = await authService.me();
          set({ user, loading: false, error: null });
        } catch (error) {
          set({ error: error instanceof Error ? error.message : 'Login failed' });
          throw error;
        }
      },

      register: async (username, email, password) => {
        authRevision += 1;
        set({ error: null });
        try {
          const result = await authService.register({ username, email, password });
          set({ user: result.user, loading: false, error: null });
        } catch (error) {
          set({ error: error instanceof Error ? error.message : 'Registration failed' });
          throw error;
        }
      },

      setUser: user => {
        authRevision += 1;
        set({ user, error: null });
      },

      logout: () => {
        authRevision += 1;
        // Clear the persisted UI state immediately; the server call revokes the
        // HttpOnly refresh session and also clears the in-memory access token.
        set({ user: null, loading: false, error: null });
        clearTokens();
        void authService.logout().catch(() => undefined);
      },
    }),
    {
      name: 'hubvas-auth',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: state => ({ user: state.user }) as PersistedAuthState,
    },
  ),
);

/** Compatibility hook used by existing components, now backed by Zustand. */
export const useAuth = useAuthStore;
