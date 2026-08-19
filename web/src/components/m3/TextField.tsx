/**
 * M3 TextField — Tier 2 wrapper via `@lit/react`.
 *
 * `md-text-field` (and its outlined/filled variants) emit Lit custom events
 * (`input`, `change`) that React 19's native custom-element support does NOT
 * map to `onInput`/`onChange` props. `createComponent()` fixes exactly this:
 * its `events` option maps React event props to the element's custom events,
 * making the standard controlled-input pattern work:
 *
 *   <TextField value={v} onChange={(e) => setV(e.target.value)} />
 */
import * as React from "react";
import type { MdFilledTextField } from "@material/web/textfield/filled-text-field.js";
import type { MdOutlinedTextField } from "@material/web/textfield/outlined-text-field.js";
import type { TextFieldType, UnsupportedTextFieldType } from "@material/web/textfield/internal/text-field.js";
import { MdFilledTextField as MdFilledTextFieldC } from "@material/web/textfield/filled-text-field.js";
import { MdOutlinedTextField as MdOutlinedTextFieldC } from "@material/web/textfield/outlined-text-field.js";
import { createM3Component } from "./createM3Component";
import "@material/web/textfield/filled-text-field.js";
import "@material/web/textfield/outlined-text-field.js";

/**
 * Props passed straight through to the md text field element. Kept loose
 * (the wrapper's M3TextFieldProps is the documented public API).
 */
export interface TextFieldInternalProps {
  label?: string;
  value?: string;
  type?: TextFieldType | UnsupportedTextFieldType;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  error?: boolean;
  errorText?: string;
  supportingText?: string;
  maxLength?: number;
  autoComplete?: string;
  inputMode?:
    | "text"
    | "numeric"
    | "decimal"
    | "email"
    | "tel"
    | "search"
    | "url"
    | "none";
  min?: string;
  max?: string;
  step?: string;
  autofocus?: boolean;
  name?: string;
  rows?: number;
  prefixText?: string;
  suffixText?: string;
  className?: string;
  style?: React.CSSProperties;
  onInput?: (e: Event) => void;
  onChange?: (e: Event) => void;
  onFocus?: (e: Event) => void;
  onBlur?: (e: Event) => void;
  children?: React.ReactNode;
}

const events = {
  onInput: "input",
  onChange: "change",
} as const;

export const FilledTextField = createM3Component<TextFieldInternalProps>({
  tagName: "md-filled-text-field",
  elementClass: MdFilledTextFieldC as unknown as new () => HTMLElement,
  events,
});

export const OutlinedTextField = createM3Component<TextFieldInternalProps>({
  tagName: "md-outlined-text-field",
  elementClass: MdOutlinedTextFieldC as unknown as new () => HTMLElement,
  events,
});

/**
 * Form-event props shared by both text field variants.
 * Note: events are mapped to native DOM events by @lit/react, so handlers
 * receive the native Event (use `e.target.value`).
 */
export interface TextFieldEvents {
  onInput?: (e: Event) => void;
  onChange?: (e: Event) => void;
}

export type TextFieldVariant = "outlined" | "filled";

export interface M3TextFieldProps extends TextFieldEvents {
  label?: string;
  value?: string;
  type?: TextFieldType | UnsupportedTextFieldType;
  variant?: TextFieldVariant;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  error?: boolean;
  errorText?: string;
  supportingText?: string;
  maxLength?: number;
  autoComplete?: string;
  inputMode?:
    | "text"
    | "numeric"
    | "decimal"
    | "email"
    | "tel"
    | "search"
    | "url"
    | "none";
  min?: string;
  max?: string;
  step?: string;
  autoFocus?: boolean;
  name?: string;
  rows?: number;
  prefixText?: string;
  suffixText?: string;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  /** Rendered into the leading-icon slot. */
  leadingIcon?: React.ReactNode;
  /** Rendered into the trailing-icon slot. */
  trailingIcon?: React.ReactNode;
  onFocus?: (e: Event) => void;
  onBlur?: (e: Event) => void;
}

/** Variant-switching wrapper for both text-field variants. */
export function TextField({
  variant = "outlined",
  label,
  value,
  type = "text",
  placeholder,
  required,
  disabled,
  error = false,
  errorText,
  supportingText,
  maxLength,
  autoComplete,
  inputMode,
  min,
  max,
  step,
  autoFocus,
  name,
  rows,
  prefixText,
  suffixText,
  className,
  style,
  children,
  leadingIcon,
  trailingIcon,
  onInput,
  onChange,
  onFocus,
  onBlur,
}: M3TextFieldProps) {
  const props: TextFieldInternalProps = {
    label,
    value,
    type,
    placeholder,
    required,
    disabled,
    error,
    errorText,
    supportingText,
    maxLength,
    autoComplete,
    inputMode,
    min,
    max,
    step,
    autofocus: autoFocus,
    name,
    rows,
    prefixText,
    suffixText,
    className,
    style,
    onInput,
    onChange,
    onFocus,
    onBlur,
  };
  const hasIconSlot = Boolean(leadingIcon || trailingIcon);
  if (hasIconSlot) {
    return variant === "filled" ? (
      <FilledTextField {...props}>
        {leadingIcon ? <span slot="leading-icon">{leadingIcon}</span> : null}
        {children}
        {trailingIcon ? <span slot="trailing-icon">{trailingIcon}</span> : null}
      </FilledTextField>
    ) : (
      <OutlinedTextField {...props}>
        {leadingIcon ? <span slot="leading-icon">{leadingIcon}</span> : null}
        {children}
        {trailingIcon ? <span slot="trailing-icon">{trailingIcon}</span> : null}
      </OutlinedTextField>
    );
  }
  return variant === "filled" ? (
    <FilledTextField {...props}>{children}</FilledTextField>
  ) : (
    <OutlinedTextField {...props}>{children}</OutlinedTextField>
  );
}
