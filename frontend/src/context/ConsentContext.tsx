import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from 'react';
import { consentApi } from '../api/consent';
import { errorMessage } from '../api/client';
import type { ApproveResult, ConsentRequest } from '../types';

interface ConsentState {
  pending: ConsentRequest[];
  history: ConsentRequest[];
  loadingPending: boolean;
  loadingHistory: boolean;
  error: string | null;
}

type ConsentAction =
  | { type: 'PENDING_START' }
  | { type: 'PENDING_LOADED'; items: ConsentRequest[] }
  | { type: 'HISTORY_START' }
  | { type: 'HISTORY_LOADED'; items: ConsentRequest[] }
  | { type: 'ITEM_UPDATED'; item: ConsentRequest }
  | { type: 'ERROR'; message: string };

function consentReducer(state: ConsentState, action: ConsentAction): ConsentState {
  switch (action.type) {
    case 'PENDING_START':
      return { ...state, loadingPending: true, error: null };
    case 'PENDING_LOADED':
      return { ...state, pending: action.items, loadingPending: false };
    case 'HISTORY_START':
      return { ...state, loadingHistory: true, error: null };
    case 'HISTORY_LOADED':
      return { ...state, history: action.items, loadingHistory: false };
    case 'ITEM_UPDATED': {
      const upsert = (list: ConsentRequest[]) =>
        list.map((r) => (r.id === action.item.id ? action.item : r));
      return {
        ...state,
        pending: upsert(state.pending),
        history: upsert(state.history),
      };
    }
    case 'ERROR':
      return { ...state, error: action.message, loadingPending: false, loadingHistory: false };
    default:
      return state;
  }
}

interface ConsentContextValue {
  pending: ConsentRequest[];
  history: ConsentRequest[];
  loadingPending: boolean;
  loadingHistory: boolean;
  error: string | null;
  loadPending: (citizenId: string) => Promise<void>;
  loadHistory: (citizenId: string) => Promise<void>;
  approve: (requestId: string) => Promise<ApproveResult>;
  reject: (requestId: string) => Promise<void>;
  revoke: (requestId: string) => Promise<void>;
  clearError: () => void;
}

const ConsentContext = createContext<ConsentContextValue | null>(null);

export function ConsentProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(consentReducer, {
    pending: [],
    history: [],
    loadingPending: false,
    loadingHistory: false,
    error: null,
  });

  const loadPending = useCallback(async (citizenId: string) => {
    dispatch({ type: 'PENDING_START' });
    try {
      const items = await consentApi.pending(citizenId);
      dispatch({ type: 'PENDING_LOADED', items });
    } catch (err) {
      dispatch({ type: 'ERROR', message: errorMessage(err).message });
    }
  }, []);

  const loadHistory = useCallback(async (citizenId: string) => {
    dispatch({ type: 'HISTORY_START' });
    try {
      const items = await consentApi.history(citizenId);
      dispatch({ type: 'HISTORY_LOADED', items });
    } catch (err) {
      dispatch({ type: 'ERROR', message: errorMessage(err).message });
    }
  }, []);

  const approve = useCallback(async (requestId: string) => {
    const result = await consentApi.approve(requestId);
    dispatch({ type: 'ITEM_UPDATED', item: result.request });
    return result;
  }, []);

  const reject = useCallback(async (requestId: string) => {
    const item = await consentApi.reject(requestId);
    dispatch({ type: 'ITEM_UPDATED', item });
  }, []);

  const revoke = useCallback(async (requestId: string) => {
    const item = await consentApi.revoke(requestId);
    dispatch({ type: 'ITEM_UPDATED', item });
  }, []);

  const clearError = useCallback(() => dispatch({ type: 'ERROR', message: '' }), []);

  const value = useMemo<ConsentContextValue>(
    () => ({
      pending: state.pending,
      history: state.history,
      loadingPending: state.loadingPending,
      loadingHistory: state.loadingHistory,
      error: state.error,
      loadPending,
      loadHistory,
      approve,
      reject,
      revoke,
      clearError,
    }),
    [state, loadPending, loadHistory, approve, reject, revoke, clearError],
  );

  return <ConsentContext.Provider value={value}>{children}</ConsentContext.Provider>;
}

export function useConsent(): ConsentContextValue {
  const ctx = useContext(ConsentContext);
  if (!ctx) {
    throw new Error('useConsent must be used within ConsentProvider');
  }
  return ctx;
}
