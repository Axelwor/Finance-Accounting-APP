import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

/**
 * Toast — lightweight notification system.
 *
 * Wrap the app with <ToastProvider>, then call `useToast()` anywhere inside
 * to push messages:
 *
 *   const toast = useToast();
 *   toast.success(`✓ Saved INV-2026-00001`);
 *   toast.error(`Failed to save: ${error.message}`);
 *
 * Toasts stack in the top-right corner, auto-dismiss after 4 s (configurable),
 * and cap at 5 visible (oldest dropped). Hover pauses the timer.
 */

export type ToastType = "success" | "error" | "info" | "warning";

export interface ToastOptions {
  /** Auto-dismiss delay in ms. Use 0 for a sticky toast. */
  duration?: number;
}

interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
  duration: number;
}

export interface ToastApi {
  success: (message: string, options?: ToastOptions) => void;
  error: (message: string, options?: ToastOptions) => void;
  info: (message: string, options?: ToastOptions) => void;
  warning: (message: string, options?: ToastOptions) => void;
  /** Dismiss a toast by id (used by the manual close button). */
  dismiss: (id: number) => void;
}

const DEFAULT_DURATION = 4000;
const MAX_VISIBLE = 5;

const ToastContext = createContext<ToastApi | null>(null);

let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const push = useCallback((type: ToastType, message: string, options?: ToastOptions) => {
    const id = nextId++;
    const duration = options?.duration ?? DEFAULT_DURATION;
    setToasts((prev) => {
      const next = [...prev, { id, type, message, duration }];
      // Keep only the newest MAX_VISIBLE toasts (drop oldest).
      if (next.length > MAX_VISIBLE) return next.slice(next.length - MAX_VISIBLE);
      return next;
    });
  }, []);

  const api = useMemo<ToastApi>(
    () => ({
      success: (m, o) => push("success", m, o),
      error: (m, o) => push("error", m, o),
      info: (m, o) => push("info", m, o),
      warning: (m, o) => push("warning", m, o),
      dismiss,
    }),
    [push, dismiss],
  );

  return (
    <ToastContext.Provider value={api}>
      {children}
      <ToastViewport toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  );
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>.");
  return ctx;
}

function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: ToastItem[];
  onDismiss: (id: number) => void;
}) {
  return (
    <div className="toast-viewport" role="region" aria-label="Notifications" aria-live="polite">
      {toasts.map((t) => (
        <ToastCard key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  );
}

function ToastCard({ toast, onDismiss }: { toast: ToastItem; onDismiss: (id: number) => void }) {
  const timerRef = useRef<number | null>(null);

  const dismiss = useCallback(() => onDismiss(toast.id), [onDismiss, toast.id]);

  const start = useCallback(() => {
    if (toast.duration <= 0) return;
    if (timerRef.current != null) return;
    timerRef.current = window.setTimeout(dismiss, toast.duration);
  }, [toast.duration, dismiss]);

  const stop = useCallback(() => {
    if (timerRef.current != null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  // Start the auto-dismiss timer on mount; clear on unmount.
  useEffect(() => {
    start();
    return stop;
  }, [start, stop]);

  return (
    <div
      className={`toast toast--${toast.type}`}
      role={toast.type === "error" ? "alert" : "status"}
      onMouseEnter={stop}
      onMouseLeave={start}
      onFocus={stop}
      onBlur={start}
    >
      <span className="toast__icon" aria-hidden="true">
        {iconFor(toast.type)}
      </span>
      <span className="toast__message">{toast.message}</span>
      <button type="button" className="toast__close" aria-label="Dismiss notification" onClick={dismiss}>
        ×
      </button>
    </div>
  );
}

function iconFor(type: ToastType): string {
  switch (type) {
    case "success":
      return "✓";
    case "error":
      return "✕";
    case "warning":
      return "!";
    case "info":
      return "i";
  }
}

export default ToastProvider;
