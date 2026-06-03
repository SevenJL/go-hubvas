import { api, setTokens } from './api';
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
      setTokens(res.data.tokens.access_token, res.data.tokens.refresh_token);
    }
    return res.data;
  },

  /** Login with email and password. */
  async login(input: LoginInput): Promise<TokenResponse> {
    const res = await api.post<TokenResponse>('/auth/login', input);
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Invalid email or password');
    }
    setTokens(res.data.access_token, res.data.refresh_token);
    return res.data;
  },

  /** Get the currently authenticated user. */
  async me(): Promise<User> {
    const res = await api.get<User>('/auth/me');
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Not authenticated');
    }
    return res.data;
  },

  /** Update the user profile (avatar URL). */
  async updateProfile(input: { avatar_url?: string }): Promise<User> {
    const res = await api.put<User>('/auth/profile', input);
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || 'Update failed');
    }
    return res.data;
  },
};
