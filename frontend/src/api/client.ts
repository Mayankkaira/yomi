import axios, {
  AxiosError,
  AxiosInstance,
  InternalAxiosRequestConfig,
} from 'axios';
import type { ApiError, AuthResponse } from '../types';

// TokenProvider lets the AuthContext register callbacks so the axios layer
// stays decoupled from React state: it asks for the current access token and
// pushes refreshed tokens back into context.
export interface TokenProvider {
  getAccessToken: () => string | null;
  setAccessToken: (token: string) => void;
  onAuthFailure: () => void;
}

let tokenProvider: TokenProvider | null = null;

/** Register the token provider (called once by AuthContext on mount). */
export function registerTokenProvider(provider: TokenProvider): void {
  tokenProvider = provider;
}

/** Base axios instance. All requests carry credentials for the httpOnly
 *  refresh cookie and the Authorization header for the in-memory access token. */
export const api: AxiosInstance = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 15000,
});

// --- Single-flight refresh ---------------------------------------------------
let refreshPromise: Promise<string | null> | null = null;

async function doRefresh(): Promise<string | null> {
  try {
    // The httpOnly refresh cookie rides along automatically (withCredentials).
    const res = await axios.post<{ data: AuthResponse }>('/api/auth/refresh', null, {
      withCredentials: true,
      timeout: 15000,
    });
    const token = res.data.data.accessToken;
    tokenProvider?.setAccessToken(token);
    return token;
  } catch {
    tokenProvider?.onAuthFailure();
    return null;
  }
}

function refreshAccessToken(): Promise<string | null> {
  // Collapse concurrent 401s into a single refresh round-trip.
  if (!refreshPromise) {
    refreshPromise = doRefresh().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

// --- Request interceptor: attach the current access token --------------------
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = tokenProvider?.getAccessToken();
  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`);
  }
  return config;
});

// --- Response interceptor: transparently refresh and retry once --------------
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiError>) => {
    const original = error.config as InternalAxiosRequestConfig & { _retried?: boolean };

    // Refresh endpoint itself must not recurse.
    const isRefreshCall = original?.url?.includes('/auth/refresh');
    const isAuthCall = original?.url?.includes('/auth/login') || original?.url?.includes('/auth/register');

    if (error.response?.status === 401 && original && !original._retried && !isRefreshCall && !isAuthCall) {
      original._retried = true;
      const token = await refreshAccessToken();
      if (token) {
        original.headers.set('Authorization', `Bearer ${token}`);
        return api(original);
      }
    }
    if (error.response?.status === 401 && isRefreshCall) {
      tokenProvider?.onAuthFailure();
    }
    return Promise.reject(error);
  },
);

/** Normalize any thrown error into a friendly message with the API code. */
export function errorMessage(err: unknown): { message: string; code: string } {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as ApiError | undefined;
    if (data?.error) {
      return { message: data.error, code: data.code ?? 'API_ERROR' };
    }
    if (err.code === 'ECONNABORTED') {
      return { message: 'Request timed out. Please try again.', code: 'TIMEOUT' };
    }
    if (!err.response) {
      return { message: 'Network error — is the gateway reachable?', code: 'NETWORK' };
    }
  }
  return { message: 'Something went wrong.', code: 'UNKNOWN' };
}
