import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from 'react';
import { CheckCircle2, AlertTriangle, Info, X } from 'lucide-react';
import { cn } from '../utils/format';

export type ToastType = 'success' | 'error' | 'info';

export interface Toast {
  id: number;
  type: ToastType;
  message: string;
}

interface ToastState {
  toasts: Toast[];
}

type ToastAction =
  | { type: 'ADD'; toast: Toast }
  | { type: 'REMOVE'; id: number };

let nextId = 1;

function toastReducer(state: ToastState, action: ToastAction): ToastState {
  switch (action.type) {
    case 'ADD':
      return { toasts: [...state.toasts, action.toast].slice(-4) }; // cap stack
    case 'REMOVE':
      return { toasts: state.toasts.filter((t) => t.id !== action.id) };
    default:
      return state;
  }
}

interface ToastContextValue {
  toasts: Toast[];
  push: (type: ToastType, message: string) => void;
  dismiss: (id: number) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(toastReducer, { toasts: [] });

  const dismiss = useCallback((id: number) => dispatch({ type: 'REMOVE', id }), []);

  const push = useCallback(
    (type: ToastType, message: string) => {
      const id = nextId++;
      dispatch({ type: 'ADD', toast: { id, type, message } });
      // Auto-dismiss after a beat; long enough to read, short enough to stay out of the way.
      window.setTimeout(() => dispatch({ type: 'REMOVE', id }), 3800);
    },
    [],
  );

  const value = useMemo(() => ({ toasts: state.toasts, push, dismiss }), [state.toasts, push, dismiss]);

  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>;
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return ctx;
}

const toastIcons: Record<ToastType, ReactNode> = {
  success: <CheckCircle2 size={16} className="text-primary-600" strokeWidth={2.2} />,
  error: <AlertTriangle size={16} className="text-red-600" strokeWidth={2.2} />,
  info: <Info size={16} className="text-blue-500" strokeWidth={2.2} />,
};

/** Stacked toast viewport rendered by ToastProvider at the app root. */
export function ToastViewport() {
  const { toasts, dismiss } = useToast();
  return (
    <div
      className="pointer-events-none fixed bottom-5 right-5 z-[60] flex w-[min(92vw,360px)] flex-col gap-2"
      aria-live="polite"
      role="status"
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={cn(
            'toast-enter pointer-events-auto flex items-start gap-2.5 rounded-md border bg-surface px-3.5 py-3 shadow-pop',
            toast.type === 'success' && 'border-primary-100',
            toast.type === 'error' && 'border-red-100',
            toast.type === 'info' && 'border-blue-100',
          )}
        >
          <span className="mt-0.5 shrink-0">{toastIcons[toast.type]}</span>
          <p className="flex-1 text-[13px] leading-snug text-ink">{toast.message}</p>
          <button
            onClick={() => dismiss(toast.id)}
            aria-label="Dismiss notification"
            className="shrink-0 rounded p-0.5 text-ink-faint transition-colors hover:text-ink"
          >
            <X size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
