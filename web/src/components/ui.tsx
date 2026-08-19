import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { Button as M3Button } from "./m3/Button";
import "@material/web/progress/circular-progress.js";

/**
 * Legacy variant names → M3 button variants.
 * (21 call sites use the old API; Fase 5-6 migrate them directly.)
 */
const VARIANT_MAP = {
  primary: "filled",
  secondary: "outlined",
  ghost: "text",
  danger: "tonal",
} as const;

export type ButtonVariant = keyof typeof VARIANT_MAP;

interface ButtonProps {
  type?: "button" | "submit";
  variant?: ButtonVariant;
  fullWidth?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  to?: string;
  children: ReactNode;
}

/**
 * Button with link support (to) and clear keyboard focus.
 * Backward-compat wrapper over the M3 Button (md-filled/outlined/text/tonal).
 */
export function Button({
  type = "button",
  variant = "primary",
  fullWidth = false,
  disabled = false,
  onClick,
  to,
  children,
}: ButtonProps) {
  return (
    <M3Button
      type={type}
      variant={VARIANT_MAP[variant]}
      fullWidth={fullWidth}
      disabled={disabled}
      onClick={onClick}
      to={to}
    >
      {children}
    </M3Button>
  );
}

/** Primary action bar for mobile / specific pages. */
export function ActionBar({
  children,
  label,
}: {
  children: ReactNode;
  label?: string;
}) {
  return (
    <nav className="action-bar" aria-label={label ?? "Primary actions"}>
      {children}
    </nav>
  );
}

interface FieldShellProps {
  label: string;
  htmlFor?: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}

export function FieldShell({ label, htmlFor, hint, error, children }: FieldShellProps) {
  const errorId = error ? `${htmlFor ?? "field"}-error` : undefined;
  return (
    <div className={`field${error ? " field--invalid" : ""}`}>
      <label className="field__label" htmlFor={htmlFor}>
        {label}
      </label>
      {children}
      {hint && !error ? <p className="field__hint">{hint}</p> : null}
      {error ? (
        <p className="field__error" id={errorId} role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}

interface TextFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: "text" | "email" | "password";
  placeholder?: string;
  hint?: string;
  error?: string;
  autoComplete?: string;
  inputMode?: "text" | "numeric" | "email";
}

export function TextField({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  hint,
  error,
  autoComplete,
  inputMode,
}: TextFieldProps) {
  const id = useId();
  return (
    <FieldShell label={label} htmlFor={id} hint={hint} error={error}>
      <input
        id={id}
        className="input"
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        autoComplete={autoComplete}
        inputMode={inputMode}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
      />
    </FieldShell>
  );
}

interface AmountFieldProps {
  label: string;
  value: string;
  onChange: (raw: string) => void;
  hint?: string;
  error?: string;
  placeholder?: string;
}

/** IDR amount input: formats thousands while typing, stores raw digits. */
export function AmountField({
  label,
  value,
  onChange,
  hint,
  error,
  placeholder,
}: AmountFieldProps) {
  const id = useId();
  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const caret = el.selectionStart ?? value.length;
    el.setSelectionRange(caret, caret);
  }, [value]);

  const handleChange = (raw: string) => {
    const digits = raw.replace(/[^\d]/g, "").slice(0, 15);
    onChange(digits);
  };

  const display = value ? Number(value).toLocaleString("en-US") : "";

  return (
    <FieldShell label={label} htmlFor={id} hint={hint} error={error}>
      <div className="input-suffix">
        <input
          ref={ref}
          id={id}
          className="input input--amount"
          type="text"
          inputMode="numeric"
          value={display}
          placeholder={placeholder ?? "0"}
          onChange={(e) => handleChange(e.target.value)}
          aria-invalid={error ? true : undefined}
        />
        <span className="input-suffix__label">Rp</span>
      </div>
    </FieldShell>
  );
}

interface DateFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  max?: string;
  onBlur?: () => void;
}

export function DateField({ label, value, onChange, error, max, onBlur }: DateFieldProps) {
  const id = useId();
  return (
    <FieldShell label={label} htmlFor={id} error={error}>
      <input
        id={id}
        className="input"
        type="date"
        value={value}
        max={max}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
      />
    </FieldShell>
  );
}

interface SelectFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  placeholder?: string;
  error?: string;
}

export function SelectField({
  label,
  value,
  onChange,
  options,
  placeholder = "Choose...",
  error,
}: SelectFieldProps) {
  const id = useId();
  return (
    <FieldShell label={label} htmlFor={id} error={error}>
      <select
        id={id}
        className="input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
      >
        <option value="">{placeholder}</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </FieldShell>
  );
}

interface TextareaFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  error?: string;
  onBlur?: () => void;
}

export function TextareaField({ label, value, onChange, placeholder, hint, error, onBlur }: TextareaFieldProps) {
  const id = useId();
  return (
    <FieldShell label={label} htmlFor={id} hint={hint} error={error}>
      <textarea
        id={id}
        className="input"
        rows={3}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
      />
    </FieldShell>
  );
}

export function FormError({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <p className="form-error" role="alert">
      {message}
    </p>
  );
}

/** Loading status for whole data pages (dashboard, lists, etc.) — M3
 *  circular progress indicator. */
export function LoadingState({ label = "Loading console..." }: { label?: string }) {
  return (
    <div className="loading-state" role="status" aria-live="polite">
      <md-circular-progress indeterminate aria-hidden="true" style={{ width: 24, height: 24 }} />
      <span>{label}</span>
    </div>
  );
}

/** Skeleton placeholder that mimics a list of transaction rows. */
export function ListSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="list-skeleton" aria-hidden="true">
      {Array.from({ length: rows }).map((_, i) => (
        <div className="list-skeleton__row" key={i}>
          <div className="list-skeleton__line list-skeleton__line--wide" />
          <div className="list-skeleton__line list-skeleton__line--narrow" />
        </div>
      ))}
    </div>
  );
}

/** Message when a list is empty, with a call to action. */
export function EmptyState({
  title,
  message,
  action,
}: {
  title: string;
  message: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <h3 className="empty-state__title">{title}</h3>
      <p className="empty-state__message">{message}</p>
      {action}
    </div>
  );
}

/** Reloadable error message. */
export function ErrorState({
  title = "Connection lost",
  message,
  onRetry,
}: {
  title?: string;
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="error-state" role="alert">
      <h3 className="error-state__title">{title}</h3>
      <p className="error-state__message">{message}</p>
      {onRetry ? (
        <Button variant="secondary" onClick={onRetry}>
          Reconnect
        </Button>
      ) : null}
    </div>
  );
}

interface CardProps {
  title?: string;
  description?: string;
  children: ReactNode;
  className?: string;
}

export function Card({ title, description, children, className }: CardProps) {
  return (
    <section className={`card${className ? ` ${className}` : ""}`}>
      {title ? (
        <header className="card__header">
          <h2 className="card__title">{title}</h2>
          {description ? <p className="card__description">{description}</p> : null}
        </header>
      ) : null}
      {children}
    </section>
  );
}

interface MultiSelectOption {
  value: number;
  label: string;
}

interface MultiSelectComboboxProps {
  options: MultiSelectOption[];
  selected: number[];
  onChange: (next: number[]) => void;
  /** Text shown on the trigger while nothing is selected. */
  placeholder?: string;
  ariaLabel: string;
}

/**
 * Compact multi-select combobox: a trigger button that shows the current
 * selection and opens a searchable checklist panel. Empty selection means
 * "no filter" (callers render it as the placeholder, e.g. "All dimensions").
 * Closes on outside click or Escape.
 */
export function MultiSelectCombobox({
  options,
  selected,
  onChange,
  placeholder = "All",
  ariaLabel,
}: MultiSelectComboboxProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const toggle = (value: number) => {
    onChange(
      selected.includes(value) ? selected.filter((v) => v !== value) : [...selected, value],
    );
  };

  const trimmed = query.trim().toLowerCase();
  const filtered = trimmed === "" ? options : options.filter((o) => o.label.toLowerCase().includes(trimmed));

  const triggerText =
    selected.length === 0
      ? placeholder
      : options.filter((o) => selected.includes(o.value)).map((o) => o.label).join(", ");

  return (
    <div className="msel" ref={rootRef}>
      <button
        type="button"
        className={`msel__trigger${selected.length > 0 ? " is-filtered" : ""}`}
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="true"
        aria-label={ariaLabel}
        title={triggerText}
      >
        <span className="msel__trigger-text">{triggerText}</span>
        <span className="msel__caret" aria-hidden="true">
          ▾
        </span>
      </button>
      {open ? (
        <div className="msel__panel">
          <input
            className="msel__search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search..."
            aria-label={`Search ${ariaLabel}`}
          />
          <div className="msel__options">
            {filtered.length === 0 ? (
              <p className="msel__empty">No matches.</p>
            ) : (
              filtered.map((o) => (
                <label key={o.value} className="msel__option">
                  <input
                    type="checkbox"
                    checked={selected.includes(o.value)}
                    onChange={() => toggle(o.value)}
                  />
                  <span>{o.label}</span>
                </label>
              ))
            )}
          </div>
          {selected.length > 0 ? (
            <button type="button" className="msel__clear" onClick={() => onChange([])}>
              Clear selection
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
