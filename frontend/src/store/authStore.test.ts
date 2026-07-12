import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { User } from '../types';

vi.mock('../services/auth', () => {
  class AuthRequestError extends Error {
    readonly status: number;

    constructor(message: string, status: number) {
      super(message);
      this.name = 'AuthRequestError';
      this.status = status;
    }
  }

  return {
    AuthRequestError,
    authService: {
      me: vi.fn(),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    },
  };
});

vi.mock('../services/api', () => ({ clearTokens: vi.fn() }));

import { AuthRequestError, authService } from '../services/auth';
import { useAuthStore } from './authStore';

const user: User = {
  id: '1',
  username: 'alice',
  email: 'alice@example.com',
  display_name: 'Alice',
  bio: '',
  website: '',
  avatar_url: '',
  account_role: 'user',
  status: 'active',
  created_at: '2026-07-12T00:00:00Z',
  updated_at: '2026-07-12T00:00:00Z',
};

const mockedMe = vi.mocked(authService.me);

function resetStore() {
  useAuthStore.setState({ user: null, loading: true, error: null });
  useAuthStore.persist.clearStorage();
}

describe('authStore persistence', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetStore();
  });

  afterEach(resetStore);

  it('persists the authenticated user without persisting access tokens', () => {
    useAuthStore.getState().setUser(user);

    const persisted = JSON.parse(localStorage.getItem('hubvas-auth') || '{}');
    expect(persisted.state).toEqual({ user });
    expect(JSON.stringify(persisted)).not.toContain('access_token');
  });

  it('keeps the cached user when session validation fails temporarily', async () => {
    useAuthStore.getState().setUser(user);
    mockedMe.mockRejectedValueOnce(new Error('network unavailable'));

    await useAuthStore.getState().initialize();

    expect(useAuthStore.getState()).toMatchObject({
      user,
      loading: false,
      error: 'network unavailable',
    });
  });

  it('clears the cached user when the refresh session is rejected', async () => {
    useAuthStore.getState().setUser(user);
    mockedMe.mockRejectedValueOnce(new AuthRequestError('not authenticated', 401));

    await useAuthStore.getState().initialize();

    expect(useAuthStore.getState()).toMatchObject({
      user: null,
      loading: false,
      error: null,
    });
  });
});
