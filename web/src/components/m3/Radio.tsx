/**
 * M3 Radio — Tier 2 wrapper via `@lit/react`.
 * Radios participate in a group via shared `name`; the checked one carries
 * the group's value.
 */
import * as React from "react";
import type { MdRadio } from "@material/web/radio/radio.js";
import { MdRadio as MdRadioC } from "@material/web/radio/radio.js";
import { createM3Component } from "./createM3Component";
import "@material/web/radio/radio.js";

export interface RadioInternalProps {
  checked?: boolean;
  disabled?: boolean;
  value: string;
  name?: string;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  "aria-label"?: string;
}

const MdRadioComponent = createM3Component<RadioInternalProps>({
  tagName: "md-radio",
  elementClass: MdRadioC as unknown as new () => HTMLElement,
  events: {
    onChange: "change",
    onInput: "input",
  },
});

export interface RadioProps {
  checked?: boolean;
  disabled?: boolean;
  value: string;
  name?: string;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  ariaLabel?: string;
}

/** Controlled md-radio (group by shared name). */
export function Radio({
  checked,
  disabled,
  value,
  name,
  onChange,
  onInput,
  className,
  style,
  ariaLabel,
}: RadioProps) {
  return (
    <MdRadioComponent
      checked={checked}
      disabled={disabled}
      value={value}
      name={name}
      onChange={onChange}
      onInput={onInput}
      className={className}
      style={style}
      aria-label={ariaLabel}
    />
  );
}

export type { MdRadio };
