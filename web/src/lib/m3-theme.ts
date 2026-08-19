/**
 * M3 Dynamic Color — theme generation & application.
 *
 * Uses @material/material-color-utilities (the same engine behind Google's
 * Material Theme Builder) to generate a full light + dark token set from a
 * single source color, then writes them as CSS custom properties.
 *
 * The static `styles/m3-tokens.css` file is the build-time fallback for the
 * default source color; at runtime this module regenerates and overrides
 * tokens when the user picks a different source color (persisted in
 * localStorage).
 */
import {
  argbFromHex,
  hexFromArgb,
  Hct,
  SchemeTonalSpot,
  type DynamicScheme,
} from "@material/material-color-utilities";

const STORAGE_KEY = "m3-source-color";

export const DEFAULT_SOURCE_COLOR = "#1a6dc4";

/* ------------------------------------------------------------------ */
/* Token generation                                                    */
/* ------------------------------------------------------------------ */

/**
 * Scheme roles published as `--md-sys-color-*` custom properties.
 * `DynamicScheme` subclasses (SchemeTonalSpot, …) expose all M3 roles
 * including the 5-level surface-container scale.
 */
const TOKEN_ROLES = [
  "primary",
  "onPrimary",
  "primaryContainer",
  "onPrimaryContainer",
  "secondary",
  "onSecondary",
  "secondaryContainer",
  "onSecondaryContainer",
  "tertiary",
  "onTertiary",
  "tertiaryContainer",
  "onTertiaryContainer",
  "error",
  "onError",
  "errorContainer",
  "onErrorContainer",
  "background",
  "onBackground",
  "outline",
  "outlineVariant",
  "shadow",
  "scrim",
  "inverseSurface",
  "inverseOnSurface",
  "inversePrimary",
  "surfaceDim",
  "surfaceBright",
  "surfaceContainerLowest",
  "surfaceContainerLow",
  "surfaceContainer",
  "surfaceContainerHigh",
  "surfaceContainerHighest",
  "onSurface",
  "onSurfaceVariant",
  "surface",
  "surfaceTint",
  "surfaceVariant",
];

/** camelCase role -> M3 CSS custom property (onPrimaryContainer -> on-primary-container). */
function roleToProp(role: string): string {
  return `--md-sys-color-${role.replace(/([A-Z])/g, "-$1").toLowerCase()}`;
}

/** Read every published role off a generated scheme and map it to hex tokens. */
function buildTokenSet(scheme: DynamicScheme): Record<string, string> {
  const tokens: Record<string, string> = {};
  for (const role of TOKEN_ROLES) {
    const value = (scheme as unknown as Record<string, unknown>)[role];
    if (typeof value === "number") {
      tokens[roleToProp(role)] = hexFromArgb(value);
    }
  }
  return tokens;
}

/** Generate the full CSS text for a source color (light + dark blocks). */
export function generateM3ThemeCSS(sourceColor: string): string {
  const hct = Hct.fromInt(argbFromHex(sourceColor));
  const light = buildTokenSet(new SchemeTonalSpot(hct, false, 0.0));
  const dark = buildTokenSet(new SchemeTonalSpot(hct, true, 0.0));

  const lines: string[] = [];
  lines.push(`/* M3 dynamic color tokens — source: ${sourceColor} */`);
  lines.push(":root {");
  for (const [prop, val] of Object.entries(light)) {
    lines.push(`  ${prop}: ${val};`);
  }
  lines.push("}");
  lines.push("");
  lines.push('[data-theme="dark"] {');
  for (const [prop, val] of Object.entries(dark)) {
    lines.push(`  ${prop}: ${val};`);
  }
  lines.push("}");
  return lines.join("\n");
}

/* ------------------------------------------------------------------ */
/* Runtime application                                                 */
/* ------------------------------------------------------------------ */

/** Load the persisted source color (falls back to the default). */
export function getStoredSourceColor(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) || DEFAULT_SOURCE_COLOR;
  } catch {
    return DEFAULT_SOURCE_COLOR;
  }
}

/**
 * Apply a theme generated from `sourceColor` by injecting a `<style>` tag.
 * Persists the choice to localStorage. Pass `null` to only apply without
 * persisting (used at app init when the stored value already exists).
 */
export function applyM3Theme(sourceColor: string, persist = true): void {
  const css = generateM3ThemeCSS(sourceColor);
  let styleEl = document.getElementById("m3-dynamic-theme");
  if (!styleEl) {
    styleEl = document.createElement("style");
    styleEl.id = "m3-dynamic-theme";
    document.head.appendChild(styleEl);
  }
  styleEl.textContent = css;

  if (persist) {
    try {
      localStorage.setItem(STORAGE_KEY, sourceColor);
    } catch {
      /* private mode — ignore */
    }
  }
}

/** Apply the stored theme at startup (before first render). */
export function initM3Theme(): void {
  applyM3Theme(getStoredSourceColor(), false);
}
