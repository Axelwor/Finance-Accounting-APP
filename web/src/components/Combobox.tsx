import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";

/**
 * Combobox — searchable select that replaces the plain <select> used for
 * foreign-key lookups (customer, supplier, item, account).
 *
 * Two flavours share one view:
 *  - {@link Combobox} lazy-loads options through an async `loadOptions`
 *    callback (debounced 300 ms) for large remote lists.
 *  - {@link StaticCombobox} filters a static `options` array client-side
 *    for small lists (payment terms, tax rates).
 *
 * Options render as "code - label" when a code is present. Keyboard
 * navigation: ArrowUp/Down to move, Enter to select, Escape to close.
 */

/** Minimal shape every combobox option must satisfy. */
export interface ComboboxOption {
  value: string | number;
  label: string;
  code?: string;
}

export interface ComboboxProps<T extends ComboboxOption> {
  value: T["value"] | null;
  onChange: (value: T["value"] | null, option: T | null) => void;
  loadOptions: (search: string) => Promise<T[]>;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

export interface StaticComboboxProps<T extends ComboboxOption> {
  value: T["value"] | null;
  onChange: (value: T["value"] | null, option: T | null) => void;
  options: T[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

const DEBOUNCE_MS = 300;

/** Format an option for the closed control: "1201 - Accounts Receivable". */
function formatOption<T extends ComboboxOption>(option: T): string {
  return option.code ? `${option.code} - ${option.label}` : option.label;
}

// ---------------------------------------------------------------------------
// Shared view (control + dropdown panel + keyboard navigation)
// ---------------------------------------------------------------------------

interface ViewProps<T extends ComboboxOption> {
  open: boolean;
  setOpen: (open: boolean) => void;
  query: string;
  setQuery: (q: string) => void;
  options: T[];
  loading: boolean;
  selectedOption: T | null;
  displayValue: string;
  onSelect: (option: T) => void;
  onClear: () => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

function ComboboxView<T extends ComboboxOption>({
  open,
  setOpen,
  query,
  setQuery,
  options,
  loading,
  selectedOption,
  displayValue,
  onSelect,
  onClear,
  placeholder = "Select...",
  disabled = false,
  className,
}: ViewProps<T>) {
  const baseId = useId();
  const listboxId = `${baseId}-listbox`;
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const optionRefs = useRef<Array<HTMLLIElement | null>>([]);
  const [highlighted, setHighlighted] = useState(-1);

  // Reset the highlight to the first option whenever the list changes.
  useEffect(() => {
    setHighlighted(options.length > 0 ? 0 : -1);
  }, [options, open]);

  // Focus the search field when the panel opens; clear the query when closed.
  useEffect(() => {
    if (!open) {
      setQuery("");
      return;
    }
    const t = window.setTimeout(() => inputRef.current?.focus(), 0);
    return () => window.clearTimeout(t);
  }, [open, setQuery]);

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
  }, [open, setOpen]);

  // Keep the highlighted option scrolled into view.
  useEffect(() => {
    const el = optionRefs.current[highlighted];
    if (el) el.scrollIntoView({ block: "nearest" });
  }, [highlighted]);

  const openPanel = useCallback(() => {
    if (!disabled) setOpen(true);
  }, [disabled, setOpen]);

  function handleControlKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (disabled) return;
    if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openPanel();
    }
  }

  function handleInputKeyDown(e: ReactKeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlighted((h) => (options.length ? (h + 1) % options.length : -1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlighted((h) => (options.length ? (h - 1 + options.length) % options.length : -1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const option = options[highlighted];
      if (option) {
        onSelect(option);
        setOpen(false);
      }
    } else if (e.key === "Escape") {
      // Stop propagation so the global "close tab" shortcut does not fire
      // while the dropdown is open.
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
    }
  }

  const showClear = !!selectedOption && !disabled;

  return (
    <div
      className={`combobox${disabled ? " combobox--disabled" : ""}${className ? ` ${className}` : ""}`}
      ref={containerRef}
    >
      <div
        className="combobox__control"
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        onClick={openPanel}
        onKeyDown={handleControlKeyDown}
      >
        <span className={`combobox__value${displayValue ? "" : " combobox__value--placeholder"}`}>
          {displayValue || placeholder}
        </span>
        <span className="combobox__icons">
          {showClear && (
            <button
              type="button"
              className="combobox__clear"
              aria-label="Clear selection"
              onClick={(e) => {
                e.stopPropagation();
                onClear();
              }}
            >
              ×
            </button>
          )}
          <span className={`combobox__chevron${open ? " combobox__chevron--open" : ""}`} aria-hidden="true" />
        </span>
      </div>

      {open && (
        <div className="combobox__panel">
          <div className="combobox__search">
            <input
              ref={inputRef}
              type="text"
              className="combobox__search-input"
              placeholder="Search..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleInputKeyDown}
              role="combobox"
              aria-expanded="true"
              aria-controls={listboxId}
              aria-autocomplete="list"
              aria-activedescendant={highlighted >= 0 ? `${baseId}-opt-${highlighted}` : undefined}
              aria-label="Search options"
            />
            {loading && <span className="combobox__spinner" aria-hidden="true" />}
          </div>
          <ul className="combobox__list" id={listboxId} role="listbox">
            {!loading && options.length === 0 ? (
              <li className="combobox__empty" role="status">
                No matches
              </li>
            ) : (
              options.map((option, i) => (
                <li
                  key={String(option.value)}
                  id={`${baseId}-opt-${i}`}
                  ref={(el) => {
                    optionRefs.current[i] = el;
                  }}
                  role="option"
                  aria-selected={selectedOption?.value === option.value}
                  className={`combobox__option${i === highlighted ? " combobox__option--active" : ""}`}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    onSelect(option);
                    setOpen(false);
                  }}
                  onMouseEnter={() => setHighlighted(i)}
                >
                  {option.code ? (
                    <span className="combobox__option-code">{option.code}</span>
                  ) : null}
                  <span className="combobox__option-label">{option.label}</span>
                </li>
              ))
            )}
          </ul>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Async Combobox — debounced remote loading
// ---------------------------------------------------------------------------

export function Combobox<T extends ComboboxOption>({
  value,
  onChange,
  loadOptions,
  placeholder,
  disabled,
  className,
}: ComboboxProps<T>) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [options, setOptions] = useState<T[]>([]);
  const [loading, setLoading] = useState(false);
  const [resolveTick, setResolveTick] = useState(0);

  // Cache of every option ever loaded so external `value` props can resolve
  // their label without an extra fetch when possible.
  const cacheRef = useRef(new Map<string | number, T>());
  // Keep the latest loadOptions in a ref so the debounce effect can depend on
  // [open, query] only — inline callbacks won't restart the timer.
  const loadOptionsRef = useRef(loadOptions);
  loadOptionsRef.current = loadOptions;

  // Debounced async load while the panel is open.
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    const handle = window.setTimeout(() => {
      loadOptionsRef
        .current(query)
        .then((result) => {
          if (cancelled) return;
          setOptions(result);
          for (const o of result) cacheRef.current.set(o.value, o);
        })
        .catch(() => {
          if (!cancelled) setOptions([]);
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

  // Best-effort label resolution when a value is set externally (e.g. editing
  // an existing record) and is not yet in the cache.
  useEffect(() => {
    if (value == null) return;
    if (cacheRef.current.has(value)) return;
    let cancelled = false;
    loadOptionsRef
      .current("")
      .then((result) => {
        if (cancelled) return;
        for (const o of result) cacheRef.current.set(o.value, o);
        setResolveTick((t) => t + 1);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [value]);

  const selectedOption: T | null =
    value == null ? null : (cacheRef.current.get(value) ?? null);
  // resolveTick is read to recompute after async label resolution.
  void resolveTick;

  const displayValue = selectedOption
    ? formatOption(selectedOption)
    : value != null
      ? String(value)
      : "";

  const handleSelect = useCallback(
    (option: T) => {
      cacheRef.current.set(option.value, option);
      onChange(option.value, option);
    },
    [onChange],
  );

  const handleClear = useCallback(() => {
    onChange(null, null);
  }, [onChange]);

  return (
    <ComboboxView
      open={open}
      setOpen={setOpen}
      query={query}
      setQuery={setQuery}
      options={options}
      loading={loading}
      selectedOption={selectedOption}
      displayValue={displayValue}
      onSelect={handleSelect}
      onClear={handleClear}
      placeholder={placeholder}
      disabled={disabled}
      className={className}
    />
  );
}

// ---------------------------------------------------------------------------
// StaticCombobox — client-side filtering of a fixed option list
// ---------------------------------------------------------------------------

export function StaticCombobox<T extends ComboboxOption>({
  value,
  onChange,
  options,
  placeholder,
  disabled,
  className,
}: StaticComboboxProps<T>) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const selectedOption = useMemo(
    () => (value == null ? null : (options.find((o) => o.value === value) ?? null)),
    [value, options],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter(
      (o) =>
        o.label.toLowerCase().includes(q) ||
        (o.code != null && o.code.toLowerCase().includes(q)),
    );
  }, [options, query]);

  const displayValue = selectedOption
    ? formatOption(selectedOption)
    : value != null
      ? String(value)
      : "";

  const handleSelect = useCallback(
    (option: T) => {
      onChange(option.value, option);
    },
    [onChange],
  );

  const handleClear = useCallback(() => {
    onChange(null, null);
  }, [onChange]);

  return (
    <ComboboxView
      open={open}
      setOpen={setOpen}
      query={query}
      setQuery={setQuery}
      options={filtered}
      loading={false}
      selectedOption={selectedOption}
      displayValue={displayValue}
      onSelect={handleSelect}
      onClear={handleClear}
      placeholder={placeholder}
      disabled={disabled}
      className={className}
    />
  );
}

export default Combobox;
