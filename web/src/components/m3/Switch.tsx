/**
 * M3 Switch — Tier 2 wrapper via `@lit/react`.
 * Events: change/input (native-mapped). Selected state via `selected`.
 */
import * as React from "react";
import type { MdSwitch } from "@material/web/switch/switch.js";
import { MdSwitch as MdSwitchC } from "@material/web/switch/switch.js";
import { createM3Component } from "./createM3Component";
import "@material/web/switch/switch.js";

export interface SwitchInternalProps {
  selected?: boolean;
  disabled?: boolean;
  icons?: boolean;
  value?: string;
  name?: string;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  "aria-label"?: string;
}

const MdSwitchComponent = createM3Component<SwitchInternalProps>({
  tagName: "md-switch",
  elementClass: MdSwitchC as unknown as new () => HTMLElement,
  events: {
    onChange: "change",
    onInput: "input",
  },
});

export interface SwitchProps {
  /** Whether the switch is on. */
  selected?: boolean;
  disabled?: boolean;
  /** Show selected/deselected icons in the thumb. */
  icons?: boolean;
  /** Value used when participating in a native form. */
  value?: string;
  name?: string;
  onChange?: (e: Event) => void;
  onInput?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  /** Accessible label (aria-label). */
  ariaLabel?: string;
}

/** Controlled md-switch. */
export function Switch({
  selected,
  disabled,
  icons,
  value,
  name,
  onChange,
  onInput,
  className,
  style,
  ariaLabel,
}: SwitchProps) {
  return (
    <MdSwitchComponent
      selected={selected}
      disabled={disabled}
      icons={icons}
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

export type { MdSwitch };
