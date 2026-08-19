/**
 * M3 Button — Tier 1 wrapper.
 *
 * `@material/web` buttons are display components: React 19's native custom
 * element support handles string/number/boolean props as attributes, so no
 * `@lit/react` wrapper is needed for them. This wrapper adds:
 *
 *  - variant → md-* element mapping (filled / outlined / tonal / text / elevated)
 *  - `to` prop → renders a react-router `<Link>` wrapping the button element
 *  - `size` density variants for data-entry grids (sm 32px / xs 28px)
 *  - `danger` / `success` semantic color overrides (error / success tokens)
 *  - common button attributes (title, id, name, value, form) + data-* rest
 */
import type { CSSProperties, ReactNode } from "react";
import { Link } from "react-router-dom";
import "@material/web/button/filled-button.js";
import "@material/web/button/outlined-button.js";
import "@material/web/button/filled-tonal-button.js";
import "@material/web/button/text-button.js";
import "@material/web/button/elevated-button.js";

export type ButtonVariant = "filled" | "outlined" | "tonal" | "text" | "elevated";
export type ButtonSize = "md" | "sm" | "xs";

/** Variant → element tag map (keeps TS honest about which tag we render). */
const VARIANT_TAGS: Record<ButtonVariant, string> = {
  filled: "md-filled-button",
  outlined: "md-outlined-button",
  tonal: "md-filled-tonal-button",
  text: "md-text-button",
  elevated: "md-elevated-button",
};

export interface M3ButtonProps {
  variant?: ButtonVariant;
  /** React-router destination — renders the button inside a <Link>. */
  to?: string;
  type?: "button" | "submit";
  disabled?: boolean;
  fullWidth?: boolean;
  /** Density variant for data-entry grids (default 40px → 32px / 28px). */
  size?: ButtonSize;
  /** Destructive styling — error tokens on the chosen variant. */
  danger?: boolean;
  /** Positive styling — success tokens on the chosen variant. */
  success?: boolean;
  onClick?: (e: React.MouseEvent) => void;
  children: ReactNode;
  /** Accessible name when children are icon-only. */
  ariaLabel?: string;
  className?: string;
  style?: CSSProperties;
  title?: string;
  id?: string;
  name?: string;
  value?: string;
  form?: string;
  /** aria-* passthrough for icon-only / popup buttons. */
  "aria-label"?: string;
  "aria-haspopup"?: boolean | string;
  "aria-expanded"?: boolean;
  /** Arbitrary data-* attributes pass through to the element. */
  [key: `data-${string}`]: string | number | boolean | undefined;
}

export function Button({
  variant = "filled",
  to,
  type = "button",
  disabled,
  fullWidth,
  size = "md",
  danger,
  success,
  onClick,
  children,
  ariaLabel,
  className,
  style,
  title,
  id,
  name,
  value,
  form,
  ...rest
}: M3ButtonProps) {
  const Tag = VARIANT_TAGS[variant] as React.ElementType;
  const classes = [
    className,
    size !== "md" ? `m3-button--${size}` : "",
    danger ? "m3-button--danger" : "",
    success ? "m3-button--success" : "",
  ]
    .filter(Boolean)
    .join(" ");
  const combinedStyle = fullWidth ? { width: "100%", ...style } : style;
  const button = (
    <Tag
      type={type}
      disabled={disabled}
      onclick={onClick}
      style={combinedStyle}
      aria-label={ariaLabel}
      title={title}
      id={id}
      name={name}
      value={value}
      form={form}
      className={classes || undefined}
      {...rest}
    >
      {children}
    </Tag>
  );

  if (to) {
    return <Link to={to}>{button}</Link>;
  }
  return button;
}
