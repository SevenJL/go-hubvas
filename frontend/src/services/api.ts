import type { ApiResponse } from '../types';

const BASE_URL = '/api';

// ---- Token management ----

let accessToken: string | null = localStorage.getItem('access_token');
let refreshToken: string | null = localStorage.getItem('refresh_token');

export function setTokens(access: string, refresh: string) {
  accessToken = access;
  refreshToken = refresh;
  localStorage.setItem('access_token', access);
  localStorage.setItem('refresh_token', refresh);
}

export function clearTokens() {
  accessToken = null;
  refreshToken = null;
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
}

export function getAccessToken(): string | null {
  return accessToken;
}

// Track whether a refresh is in progress to avoid multiple concurrent refreshes.
let refreshPromise: Promise<boolean> | null = null;

async function tryRefreshToken(): Promise<boolean> {
  if (!refreshToken) return false;

  // Reuse an in-flight refresh to avoid thundering-herd.
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    try {
      const res = await fetch(`${BASE_URL}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });

      if (!res.ok) {
        clearTokens();
        return false;
      }

      const body: ApiResponse<{ access_token: string; refresh_token: string }> = await res.json();
      if (body.code !== 0 || !body.data) {
        clearTokens();
        return false;
      }

      setTokens(body.data.access_token, body.data.refresh_token);
      return true;
    } catch {
      // Network error — don't clear tokens (might be a transient issue).
      return false;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

// ---- Request helper ----

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (accessToken) {
    headers['Authorization'] = `Bearer ${accessToken}`;
  }

  let res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  // If the access token is expired, try to refresh and retry ONCE.
  if (res.status === 401 && refreshToken) {
    const refreshed = await tryRefreshToken();
    if (refreshed) {
      // Retry with the new access token.
      headers['Authorization'] = `Bearer ${accessToken}`;
      res = await fetch(`${BASE_URL}${path}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });
    } else {
      // Refresh failed — clear tokens so the UI can show the login page.
      clearTokens();
    }
  }

  // Parse the response body.
  let json: ApiResponse<T>;
  try {
    json = await res.json();
  } catch {
    // If the response isn't JSON (e.g., a network error or HTML error page),
    // synthesize an error response.
    return {
      code: res.status || 500,
      message: `Request failed (${res.status})`,
    };
  }

  return json;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
};
