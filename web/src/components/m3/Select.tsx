/**
 * M3 Select — Tier 2 wrapper via `@lit/react`.
 *
 * `md-outlined-select` / `md-filled-select` with `md-select-option`
 * children. The `change`/`input` events are Lit custom events mapped
 * through createComponent. Note: the value prop is a string (the selected
 * option's value), and options are light-DOM children.
 */
import * as React from "react";
import type { MdOutlinedSelect } from "@material/web/select/outlined-select.js";
import { MdOutlinedSelect as MdOutlinedSelectC } from "@material/web/select/outlined-select.js";
import { MdFilledSelect as MdFilledSelectC } from "@material/web/select/filled-select.js";
import { MdSelectOption as MdSelectOptionC } from "@material/web/select/select-option.js";
import { createM3Component } from "./createM3Component";
import "@material/web/select/outlined-select.js";
import "@material/web/select/filled-select.js";
import "@material/web/select/select-option.js";

export interface SelectInternalProps {
  value?: string;
  label?: string;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  error?: boolean;
  supportingText?: string;
  menuFixed?: boolean;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

const events = {
  onChange: "change",
  onInput: "input",
} as const;

export const OutlinedSelect = createM3Component<SelectInternalProps>({
  tagName: "md-outlined-select",
  elementClass: MdOutlinedSelectC as unknown as new () => HTMLElement,
  events,
});

export const FilledSelect = createM3Component<SelectInternalProps>({
  tagName: "md-filled-select",
  elementClass: MdFilledSelectC as unknown as new () => HTMLElement,
  events,
});

export const SelectOption = createM3Component<{
  value?: string;
  disabled?: boolean;
  selected?: boolean;
  className?: string;
  children?: React.ReactNode;
}>({
  tagName: "md-select-option",
  elementClass: MdSelectOptionC as unknown as new () => HTMLElement,
  events: {},
});

export interface SelectOptionData {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  /** Value of the selected option (controlled). */
  value?: string;
  label?: string;
  variant?: "outlined" | "filled";
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  error?: boolean;
  supportingText?: string;
  menuFixed?: boolean;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  /** Shorthand: render options from data instead of children. */
  options?: SelectOptionData[];
}

/** Controlled md-select with option shorthand. */
export function Select({
  value,
  label,
  variant = "outlined",
  placeholder,
  disabled,
  required,
  error,
  supportingText,
  menuFixed,
  onChange,
  onInput,
  className,
  style,
  children,
  options,
}: SelectProps) {
  const Tag = variant === "filled" ? FilledSelect : OutlinedSelect;
  return (
    <Tag
      value={value}
      label={label}
      placeholder={placeholder}
      disabled={disabled}
      required={required}
      error={error}
      supportingText={supportingText}
      menuFixed={menuFixed}
      onChange={onChange}
      onInput={onInput}
      className={className}
      style={style}
    >
      {options
        ? options.map((o) => (
            <SelectOption key={o.value} value={o.value} disabled={o.disabled}>
              {o.label}
            </SelectOption>
          ))
        : children}
    </Tag>
  );
}

export type { MdOutlinedSelect };
