/**
 * M3 Checkbox — Tier 2 wrapper via `@lit/react`.
 * Events: change/input (native-mapped).
 */
import * as React from "react";
import type { MdCheckbox } from "@material/web/checkbox/checkbox.js";
import { MdCheckbox as MdCheckboxC } from "@material/web/checkbox/checkbox.js";
import { createM3Component } from "./createM3Component";
import "@material/web/checkbox/checkbox.js";

export interface CheckboxInternalProps {
  checked?: boolean;
  indeterminate?: boolean;
  disabled?: boolean;
  value?: string;
  name?: string;
  touch?: boolean;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  "aria-label"?: string;
}

const MdCheckboxComponent = createM3Component<CheckboxInternalProps>({
  tagName: "md-checkbox",
  elementClass: MdCheckboxC as unknown as new () => HTMLElement,
  events: {
    onChange: "change",
    onInput: "input",
  },
});

export interface CheckboxProps {
  checked?: boolean;
  indeterminate?: boolean;
  disabled?: boolean;
  /** Value used when participating in a native form. */
  value?: string;
  name?: string;
  touch?: boolean;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  /** Accessible label (aria-label) — checkbox has no visible text. */
  ariaLabel?: string;
}

/** Controlled md-checkbox. */
export function Checkbox({
  checked,
  indeterminate,
  disabled,
  value,
  name,
  touch,
  onChange,
  onInput,
  className,
  style,
  ariaLabel,
}: CheckboxProps) {
  return (
    <MdCheckboxComponent
      checked={checked}
      indeterminate={indeterminate}
      disabled={disabled}
      value={value}
      name={name}
      touch={touch}
      onChange={onChange}
      onInput={onInput}
      className={className}
      style={style}
      aria-label={ariaLabel}
    />
  );
}

export type { MdCheckbox };
