import { api, clearTokens, setTokens } from './api';
import type { User, TokenResponse, RegisterResponse } from '../types';

export interface RegisterInput {
  username: string;
  email: string;
  password: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export const authService = {
  /**
   * Register a new account.
   * The backend now returns both user AND tokens, so no separate login is needed.
   */
  async register(input: RegisterInput): Promise<RegisterResponse> {
    const res = await api.post<RegisterResponse>('/auth/register', input);
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Registration failed');
    }
    // Auto-save the tokens so the user is immediately authenticated.
    if (res.data.tokens) {
      setTokens(res.data.tokens.access_token);
    }
    return res.data;
  },

  /** Login with email and password. */
  async login(input: LoginInput): Promise<TokenResponse> {
    const res = await api.post<TokenResponse>('/auth/login', input);
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Invalid email or password');
    }
    setTokens(res.data.access_token);
    return res.data;
  },

  async logout(): Promise<void> {
    try { await api.post('/auth/logout'); } finally { clearTokens(); }
  },

  /** Get the currently authenticated user. */
  async me(): Promise<User> {
    const res = await api.get<User>('/auth/me');
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Not authenticated');
    }
    return res.data;
  },

  /** Update mutable public profile fields. */
  async updateProfile(input: { display_name: string; bio: string; website: string }): Promise<User> {
    const res = await api.patch<User>('/auth/profile', input);
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Update failed');
    }
    return res.data;
  },
};
