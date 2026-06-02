import { api, setTokens } from './api';
import type { TokenResponse, User } from '../types';

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
  async register(input: RegisterInput): Promise<User> {
    const res = await api.post<User>('/auth/register', input);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async login(input: LoginInput): Promise<TokenResponse> {
    const res = await api.post<TokenResponse>('/auth/login', input);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    setTokens(res.data.access_token, res.data.refresh_token);
    return res.data;
  },

  async me(): Promise<User> {
    const res = await api.get<User>('/auth/me');
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },
};
