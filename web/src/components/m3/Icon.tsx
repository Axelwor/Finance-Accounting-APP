/**
 * M3 Icon — Material Symbols wrapper.
 *
 * `md-icon` renders the ligature text of Google's Material Symbols font
 * (loaded in index.html). Icon names use snake_case ligatures:
 *   <Icon name="receipt_long" />
 *
 * For slotted usage inside other md-* components (e.g. the leading icon of
 * a list item), pass `slot="leading-icon"`.
 */
import "@material/web/icon/icon.js";

export interface IconProps {
  /** Material Symbols ligature name, snake_case (e.g. "receipt_long"). */
  name: string;
  /** Slot name for embedding inside other md-* components. */
  slot?: string;
  /** Filled style (default is outlined). */
  filled?: boolean;
  size?: number | string;
  className?: string;
  style?: Record<string, string>;
}

export function Icon({ name, slot, filled = false, size, className, style }: IconProps) {
  const iconStyle: Record<string, string> = {
    fontVariationSettings: `'FILL' ${filled ? 1 : 0}, 'wght' 400, 'GRAD' 0, 'opsz' 24`,
    ...(size != null ? { fontSize: typeof size === "number" ? `${size}px` : size } : {}),
    ...style,
  };
  return (
    <md-icon slot={slot} className={className} style={iconStyle}>
      {name}
    </md-icon>
  );
}
