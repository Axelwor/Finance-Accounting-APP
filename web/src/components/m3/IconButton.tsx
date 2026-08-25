/**
 * M3 IconButton — Tier 1 wrapper around `md-icon-button`.
 *
 * Icon-only action button (40px default, `size="sm"` → 32px for dense
 * toolbars/rows). Children render into the default slot — inline SVG
 * components work as-is; use `<Icon name="..." />` for named icons.
 */
import type { CSSProperties, ReactNode } from "react";
import "@material/web/iconbutton/icon-button.js";

export type IconButtonSize = "md" | "sm" | "xs";

export interface IconButtonProps {
  /** Accessible name — required for icon-only buttons. */
  label: string;
  size?: IconButtonSize;
  disabled?: boolean;
  onClick?: (e: React.MouseEvent) => void;
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  title?: string;
  id?: string;
  /** aria-* passthrough (label prop covers aria-label). */
  "aria-haspopup"?: boolean | string;
  "aria-expanded"?: boolean;
  /** Arbitrary data-* attributes pass through to the element. */
  [key: `data-${string}`]: string | number | boolean | undefined;
}

export function IconButton({
  label,
  size = "md",
  disabled,
  onClick,
  children,
  className,
  style,
  title,
  id,
  ...rest
}: IconButtonProps) {
  const classes = [
    className,
    size !== "md" ? `m3-icon-button--${size}` : "",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <md-icon-button
      aria-label={label}
      disabled={disabled}
      onclick={onClick}
      title={title}
      id={id}
      className={classes || undefined}
      style={style}
      {...rest}
    >
      {children}
    </md-icon-button>
  );
}
