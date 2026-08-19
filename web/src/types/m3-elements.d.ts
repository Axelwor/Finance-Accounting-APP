/**
 * TypeScript JSX augmentation for `@material/web` Tier 1 elements used
 * directly in JSX (display-only components where React 19's native custom
 * element support — primitives→attributes, objects→properties — is enough).
 *
 * Tier 2 elements (form/event components: TextField, Select, Switch, …) get
 * their types from `@lit/react`'s `createComponent()` wrappers in
 * `src/components/m3/` and are NOT declared here.
 *
 * Rule of thumb for props:
 *  - string/number/boolean → React 19 sets them as attributes
 *  - functions/objects → React 19 sets them as properties
 *  - custom events (md-* "changed" style events) → not mapped; use wrappers
 */
import type * as React from "react";
import type { MdFilledButton } from "@material/web/button/filled-button.js";
import type { MdOutlinedButton } from "@material/web/button/outlined-button.js";
import type { MdElevatedButton } from "@material/web/button/elevated-button.js";
import type { MdTextButton } from "@material/web/button/text-button.js";
import type { MdFilledTonalButton } from "@material/web/button/filled-tonal-button.js";
import type { MdIcon } from "@material/web/icon/icon.js";
import type { MdList } from "@material/web/list/list.js";
import type { MdCircularProgress } from "@material/web/progress/circular-progress.js";
import type { MdLinearProgress } from "@material/web/progress/linear-progress.js";
import type { MdAssistChip } from "@material/web/chips/assist-chip.js";
import type { MdFilterChip } from "@material/web/chips/filter-chip.js";
import type { MdInputChip } from "@material/web/chips/input-chip.js";
import type { MdSuggestionChip } from "@material/web/chips/suggestion-chip.js";
import type { MdFab } from "@material/web/fab/fab.js";
import type { MdBrandedFab } from "@material/web/fab/branded-fab.js";
import type { MdDivider } from "@material/web/divider/divider.js";
import type { MdFocusRing } from "@material/web/focus/focus-ring.js";
import type { MdElevation } from "@material/web/elevation/elevation.js";
import type { MdRipple } from "@material/web/labs/behavior/ripple/ripple.js";
import type { MdPrimaryTab } from "@material/web/tabs/primary-tab.js";
import type { MdSecondaryTab } from "@material/web/tabs/secondary-tab.js";
import type { MdIconButton } from "@material/web/iconbutton/icon-button.js";
import type { MdFilledIconButton } from "@material/web/iconbutton/filled-icon-button.js";
import type { MdFilledTonalIconButton } from "@material/web/iconbutton/filled-tonal-icon-button.js";
import type { MdOutlinedIconButton } from "@material/web/iconbutton/outlined-icon-button.js";
import type { MdMenuItem } from "@material/web/menu/menu-item.js";
import type { MdListItem } from "@material/web/list/list-item.js";
import type { MdCircularProgress } from "@material/web/progress/circular-progress.js";

/**
 * Minimal props for a web component element: every React prop is passed
 * through to the custom element (attribute for primitives, property
 * otherwise), plus the usual React DOM extras.
 *
 * Native DOM `style`/`children`/event-handler properties are omitted so
 * React's versions win; React 19 sets `onclick`/`onkeydown` etc. as
 * element properties with synthetic event semantics.
 */
export type MdElementProps<T> = Omit<
  Partial<T>,
  | "style"
  | "children"
  | "className"
  | "slot"
  | "part"
  | "exportparts"
  | "onclick"
  | "onkeydown"
  | "onkeyup"
  | "onfocus"
  | "onblur"
  | "onpointerdown"
  | "onpointerup"
  | "oninput"
  | "onchange"
> & {
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  key?: React.Key | null;
  ref?: React.Ref<T>;
  slot?: string;
  part?: string;
  exportparts?: string;
  onclick?: React.MouseEventHandler<T>;
  onkeydown?: React.KeyboardEventHandler<T>;
  onkeyup?: React.KeyboardEventHandler<T>;
  onfocus?: React.FocusEventHandler<T>;
  onblur?: React.FocusEventHandler<T>;
  onpointerdown?: React.PointerEventHandler<T>;
  onpointerup?: React.PointerEventHandler<T>;
};

declare module "react" {
  namespace JSX {
    interface IntrinsicElements {
      /** Filled button (highest emphasis action). */
      "md-filled-button": MdElementProps<MdFilledButton>;
      /** Outlined button. */
      "md-outlined-button": MdElementProps<MdOutlinedButton>;
      /** Elevated button (surface + shadow). */
      "md-elevated-button": MdElementProps<MdElevatedButton>;
      /** Text button (lowest emphasis). */
      "md-text-button": MdElementProps<MdTextButton>;
      /** Tonal button (secondary, container fill). */
      "md-filled-tonal-button": MdElementProps<MdFilledTonalButton>;
      /** Material Symbols icon. */
      "md-icon": MdElementProps<MdIcon>;
      /** List container for md-item elements. */
      "md-list": MdElementProps<MdList>;
      /** Indeterminate/determinate circular progress indicator. */
      "md-circular-progress": MdElementProps<MdCircularProgress>;
      /** Linear progress indicator. */
      "md-linear-progress": MdElementProps<MdLinearProgress>;
      /** Assist chip (helper action). */
      "md-assist-chip": MdElementProps<MdAssistChip>;
      /** Filter chip (toggleable, checkmark). */
      "md-filter-chip": MdElementProps<MdFilterChip>;
      /** Input chip (represents an input entity, removable). */
      "md-input-chip": MdElementProps<MdInputChip>;
      /** Suggestion chip (offered while typing). */
      "md-suggestion-chip": MdElementProps<MdSuggestionChip>;
      /** Floating action button. */
      "md-fab": MdElementProps<MdFab>;
      /** Branded FAB. */
      "md-branded-fab": MdElementProps<MdBrandedFab>;
      /** Divider (hairline separator). */
      "md-divider": MdElementProps<MdDivider>;
      /** Focus ring indicator. */
      "md-focus-ring": MdElementProps<MdFocusRing>;
      /** Elevation shadow box. */
      "md-elevation": MdElementProps<MdElevation>;
      /** Ripple effect behavior. */
      "md-ripple": MdElementProps<MdRipple>;
      /** Primary tab (inside md-tabs). */
      "md-primary-tab": MdElementProps<MdPrimaryTab>;
      /** Secondary tab (inside md-tabs). */
      "md-secondary-tab": MdElementProps<MdSecondaryTab>;
      /** Standard icon button. */
      "md-icon-button": MdElementProps<MdIconButton>;
      /** Filled icon button. */
      "md-filled-icon-button": MdElementProps<MdFilledIconButton>;
      /** Filled tonal icon button. */
      "md-filled-tonal-icon-button": MdElementProps<MdFilledTonalIconButton>;
      /** Outlined icon button. */
      "md-outlined-icon-button": MdElementProps<MdOutlinedIconButton>;
      /** Menu item (inside md-menu). */
      "md-menu-item": MdElementProps<MdMenuItem>;
      /** List item (inside md-list). */
      "md-list-item": MdElementProps<MdListItem>;
    }
  }
}
