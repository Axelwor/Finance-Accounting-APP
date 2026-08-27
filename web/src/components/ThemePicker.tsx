/**
 * Dynamic color picker — Material You source color selection.
 *
 * Renders a small palette of preset source colors plus a native color
 * input. Selecting one regenerates the entire M3 theme live (all
 * --md-sys-color-* tokens) via applyM3Theme() and persists the choice.
 */
import { useEffect, useRef, useState } from "react";
import { applyM3Theme, getStoredSourceColor, DEFAULT_SOURCE_COLOR } from "../lib/m3-theme";
import { Icon } from "./m3/Icon";
import "@material/web/iconbutton/icon-button.js";

/** Curated Material You palette (hex source colors). */
const PRESET_COLORS = [
  { value: "#1a6dc4", label: "Trexo Blue" },
  { value: "#6750a4", label: "Violet" },
  { value: "#006a60", label: "Teal" },
  { value: "#00629d", label: "Ocean" },
  { value: "#984061", label: "Rose" },
  { value: "#006e17", label: "Green" },
  { value: "#825500", label: "Amber" },
  { value: "#7d5260", label: "Mauve" },
];

export function ThemePicker() {
  const [open, setOpen] = useState(false);
  const [color, setColor] = useState(() => {
    try {
      return getStoredSourceColor();
    } catch {
      return DEFAULT_SOURCE_COLOR;
    }
  });
  const popoverRef = useRef<HTMLDivElement>(null);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const pick = (value: string) => {
    setColor(value);
    applyM3Theme(value);
  };

  return (
    <div className="theme-picker" ref={popoverRef}>
      <md-icon-button
        onclick={() => setOpen((v) => !v)}
        aria-label="Change theme color"
        title="Change theme color"
        aria-expanded={open}
      >
        <Icon name="palette" />
      </md-icon-button>
      {open ? (
        <div className="theme-picker__popover" role="dialog" aria-label="Theme color">
          <p className="theme-picker__title">Theme color</p>
          <div className="theme-picker__swatches">
            {PRESET_COLORS.map((preset) => (
              <button
                key={preset.value}
                type="button"
                className={`theme-picker__swatch${color.toLowerCase() === preset.value ? " is-active" : ""}`}
                style={{ background: preset.value }}
                aria-label={preset.label}
                title={preset.label}
                onClick={() => pick(preset.value)}
              />
            ))}
          </div>
          <label className="theme-picker__custom">
            <span>Custom</span>
            <input
              type="color"
              value={color}
              onChange={(e) => pick(e.target.value)}
              aria-label="Custom source color"
            />
          </label>
          {color.toLowerCase() !== DEFAULT_SOURCE_COLOR ? (
            <button
              type="button"
              className="theme-picker__reset"
              onClick={() => pick(DEFAULT_SOURCE_COLOR)}
            >
              Reset to default
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
