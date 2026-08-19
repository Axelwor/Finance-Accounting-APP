/**
 * M3 Autocomplete — custom project wrapper.
 *
 * Composes `md-outlined-text-field` + `md-menu` into an async searchable
 * select (M3 autocomplete pattern). Replaces the legacy `Combobox` /
 * `StaticCombobox` / `AccountPicker` components — same API surface:
 *
 *   <Autocomplete value={v} onChange={(v, o) => ...} loadOptions={fetchFn} />
 *   <Autocomplete value={v} onChange={...} options={[...]} />  // static
 *
 * Options render as "code - label" when a code is present. The text field
 * filters (debounced 300 ms for async), the menu shows results, and
 * keyboard navigation follows the M3 menu pattern (typeahead handled by
 * the underlying input).
 */
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import type { MdMenu } from "@material/web/menu/menu.js";
import { MdMenu as MdMenuC } from "@material/web/menu/menu.js";
import { createM3Component } from "./createM3Component";
import { TextField } from "./TextField";
import type { M3TextFieldProps, TextFieldInternalProps } from "./TextField";
import "@material/web/menu/menu.js";
import "@material/web/menu/menu-item.js";
import "@material/web/progress/circular-progress.js";

interface AutocompleteMenuInternalProps {
  open?: boolean;
  anchor?: string;
  positioning?: "absolute" | "fixed" | "document" | "popover";
  className?: string;
  style?: React.CSSProperties;
  onClose?: (e: Event) => void;
  children?: React.ReactNode;
}

const MdMenuComponent = createM3Component<AutocompleteMenuInternalProps>({
  tagName: "md-menu",
  elementClass: MdMenuC as unknown as new () => HTMLElement,
  events: {
    onClose: "close",
  },
});

/** Minimal shape every autocomplete option must satisfy. */
export interface AutocompleteOption {
  value: string | number;
  label: string;
  code?: string;
}

export interface AutocompleteProps {
  value: AutocompleteOption["value"] | null;
  onChange: (value: AutocompleteOption["value"] | null, option: AutocompleteOption | null) => void;
  /** Async loader (debounced 300 ms). Omit when using static `options`. */
  loadOptions?: (search: string) => Promise<AutocompleteOption[]>;
  /** Static option list — filtered client-side. */
  options?: AutocompleteOption[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  /** Optional label rendered inside the field. */
  label?: string;
  /** Extra props forwarded to the inner text field. */
  fieldProps?: Partial<TextFieldInternalProps>;
}

const DEBOUNCE_MS = 300;

/** Format an option for the closed control: "1201 - Accounts Receivable". */
function formatOption(option: AutocompleteOption): string {
  return option.code ? `${option.code} - ${option.label}` : option.label;
}

export function Autocomplete({
  value,
  onChange,
  loadOptions,
  options: staticOptions,
  placeholder = "Select...",
  disabled,
  className,
  label,
  fieldProps,
}: AutocompleteProps) {
  const baseId = useId();
  const menuId = `${baseId}-menu`;
  const containerRef = useRef<HTMLDivElement>(null);

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [remoteOptions, setRemoteOptions] = useState<AutocompleteOption[]>([]);
  const [loading, setLoading] = useState(false);

  // Cache of every option ever loaded so external `value` props can resolve
  // their label without an extra fetch when possible.
  const cacheRef = useRef(new Map<string | number, AutocompleteOption>());
  const loadOptionsRef = useRef(loadOptions);
  loadOptionsRef.current = loadOptions;

  // Debounced async load while the menu is open.
  useEffect(() => {
    if (!open || !loadOptionsRef.current) return;
    let cancelled = false;
    setLoading(true);
    const handle = window.setTimeout(() => {
      loadOptionsRef
        .current?.(query)
        .then((result) => {
          if (cancelled) return;
          setRemoteOptions(result);
          for (const o of result) cacheRef.current.set(o.value, o);
        })
        .catch(() => {
          if (!cancelled) setRemoteOptions([]);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, DEBOUNCE_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(handle);
    };
  }, [open, query]);

  // Best-effort label resolution when a value is set externally.
  useEffect(() => {
    if (value == null || !loadOptionsRef.current) return;
    if (cacheRef.current.has(value)) return;
    let cancelled = false;
    loadOptionsRef
      .current("")
      .then((result) => {
        if (cancelled) return;
        for (const o of result) cacheRef.current.set(o.value, o);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [value]);

  // Close on outside pointer down.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [open]);

  const isAsync = Boolean(loadOptions);
  const list = useMemo(() => {
    if (isAsync) return remoteOptions;
    const opts = staticOptions ?? [];
    const q = query.trim().toLowerCase();
    if (!q) return opts;
    return opts.filter(
      (o) => o.label.toLowerCase().includes(q) || (o.code != null && o.code.toLowerCase().includes(q)),
    );
  }, [isAsync, remoteOptions, staticOptions, query]);

  const selectedOption: AutocompleteOption | null = useMemo(() => {
    if (value == null) return null;
    if (staticOptions) return staticOptions.find((o) => o.value === value) ?? null;
    return cacheRef.current.get(value) ?? null;
  }, [value, staticOptions]);

  const displayValue = selectedOption
    ? formatOption(selectedOption)
    : value != null
      ? String(value)
      : "";

  const handleSelect = useCallback(
    (option: AutocompleteOption) => {
      cacheRef.current.set(option.value, option);
      onChange(option.value, option);
      setOpen(false);
      setQuery("");
    },
    [onChange],
  );

  const handleClear = useCallback(() => {
    onChange(null, null);
    setQuery("");
  }, [onChange]);

  const handleInput = (e: Event) => {
    const el = e.target as HTMLElement & { value: string };
    setQuery(el.value);
    if (!open) setOpen(true);
  };

  return (
    <div
      className={`m3-autocomplete${open ? " m3-autocomplete--open" : ""}${className ? ` ${className}` : ""}`}
      ref={containerRef}
    >
      <div className="m3-autocomplete__field-wrap">
        <TextField
          {...fieldProps}
          label={label}
          value={open ? query : displayValue}
          placeholder={placeholder}
          disabled={disabled}
          onInput={handleInput}
          onFocus={() => {
            if (!disabled) {
              setOpen(true);
              setQuery("");
            }
          }}
          trailingIcon={
            displayValue && !disabled && !open ? (
              <button
                type="button"
                className="m3-autocomplete__clear"
                aria-label="Clear selection"
                onClick={handleClear}
              >
                <md-icon>close</md-icon>
              </button>
            ) : null
          }
          className={`m3-autocomplete__field${fieldProps?.className ? ` ${fieldProps.className}` : ""}`}
        />
        {loading && open ? (
          <md-circular-progress
            indeterminate
            aria-label="Loading options"
            className="m3-autocomplete__spinner"
            style={{ width: 18, height: 18 }}
          />
        ) : null}
      </div>

      <MdMenuComponent
        open={open}
        anchor={menuId}
        positioning="fixed"
        className="m3-autocomplete__menu"
        onClose={() => setOpen(false)}
      >
        <div id={menuId} className="m3-autocomplete__anchor" />
        {list.length === 0 ? (
          <div className="m3-autocomplete__empty" role="status">
            {loading ? "Searching..." : "No matches"}
          </div>
        ) : (
          list.map((option) => (
            <md-menu-item
              key={String(option.value)}
              onclick={() => handleSelect(option)}
              className={`m3-autocomplete__option${
                selectedOption?.value === option.value ? " m3-autocomplete__option--selected" : ""
              }`}
            >
              <div slot="headline">
                {option.code ? <span className="m3-autocomplete__code">{option.code}</span> : null}
                <span className="m3-autocomplete__label">{option.label}</span>
              </div>
            </md-menu-item>
          ))
        )}
      </MdMenuComponent>
    </div>
  );
}

export default Autocomplete;
