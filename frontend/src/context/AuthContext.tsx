import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
} from 'react';
import { authApi, LoginPayload, RegisterPayload } from '../api/auth';
import { errorMessage, registerTokenProvider } from '../api/client';
import type { Citizen } from '../types';

interface AuthState {
  citizen: Citizen | null;
  accessToken: string | null;
  status: 'idle' | 'loading' | 'authenticated' | 'unauthenticated';
  error: string | null;
}

type AuthAction =
  | { type: 'AUTH_START' }
  | { type: 'AUTH_SUCCESS'; citizen: Citizen; accessToken: string }
  | { type: 'AUTH_FAILURE'; message: string }
  | { type: 'AUTH_CLEAR' };

function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'AUTH_START':
      return { ...state, status: 'loading', error: null };
    case 'AUTH_SUCCESS':
      return {
        citizen: action.citizen,
        accessToken: action.accessToken,
        status: 'authenticated',
        error: null,
      };
    case 'AUTH_FAILURE':
      return { ...state, status: 'unauthenticated', error: action.message };
    case 'AUTH_CLEAR':
      return { citizen: null, accessToken: null, status: 'unauthenticated', error: null };
    default:
      return state;
  }
}

interface AuthContextValue {
  citizen: Citizen | null;
  accessToken: string | null;
  status: AuthState['status'];
  error: string | null;
  login: (payload: LoginPayload) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<void>;
  logout: () => Promise<void>;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, {
    citizen: null,
    accessToken: null,
    status: 'idle',
    error: null,
  });

  // The axios layer pulls the token from context and pushes refreshed tokens
  // back in. On refresh failure the session is ended.
  useEffect(() => {
    registerTokenProvider({
      getAccessToken: () => state.accessToken,
      setAccessToken: (token: string) => {
        dispatch({ type: 'AUTH_SUCCESS', citizen: state.citizen!, accessToken: token });
      },
      onAuthFailure: () => dispatch({ type: 'AUTH_CLEAR' }),
    });
  }, [state.accessToken, state.citizen]);

  // On first load, attempt a silent session restore using the httpOnly
  // refresh cookie. This makes a page reload (which clears the in-memory
  // access token by design) resume the session instead of stranding the user
  // on the loading screen. A ref guard makes the effect StrictMode-safe: the
  // dev double-invoke must not fire two concurrent refreshes against the same
  // (rotating) refresh token. Note: no cancellation flag here — StrictMode
  // runs the cleanup immediately after the first invoke, and a cancellation
  // flag combined with the ref guard would drop the resolved dispatch (the
  // second invoke never runs to re-arm it).
  const restoreAttempted = useRef(false);
  useEffect(() => {
    if (restoreAttempted.current) return;
    restoreAttempted.current = true;
    (async () => {
      try {
        const res = await authApi.refresh();
        dispatch({ type: 'AUTH_SUCCESS', citizen: res.citizen, accessToken: res.accessToken });
      } catch {
        dispatch({ type: 'AUTH_CLEAR' });
      }
    })();
  }, []);

  const login = useCallback(async (payload: LoginPayload) => {
    dispatch({ type: 'AUTH_START' });
    try {
      const res = await authApi.login(payload);
      dispatch({ type: 'AUTH_SUCCESS', citizen: res.citizen, accessToken: res.accessToken });
    } catch (err) {
      const { message } = errorMessage(err);
      dispatch({ type: 'AUTH_FAILURE', message });
      throw err;
    }
  }, []);

  const register = useCallback(async (payload: RegisterPayload) => {
    dispatch({ type: 'AUTH_START' });
    try {
      await authApi.register(payload);
      // Registration succeeds with no session; the user then logs in.
      dispatch({ type: 'AUTH_CLEAR' });
    } catch (err) {
      const { message } = errorMessage(err);
      dispatch({ type: 'AUTH_FAILURE', message });
      throw err;
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch {
      // The server session may already be gone; clear locally regardless.
    } finally {
      dispatch({ type: 'AUTH_CLEAR' });
    }
  }, []);

  const clearError = useCallback(() => dispatch({ type: 'AUTH_FAILURE', message: '' }), []);

  const value = useMemo<AuthContextValue>(
    () => ({
      citizen: state.citizen,
      accessToken: state.accessToken,
      status: state.status,
      error: state.error,
      login,
      register,
      logout,
      clearError,
    }),
    [state, login, register, logout, clearError],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
