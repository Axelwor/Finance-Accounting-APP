/**
 * M3 component wrappers — barrel export.
 *
 * Two tiers (see .kilo/plans/material-design-3-full-redesign.md §3):
 *  - Tier 1 (display): direct JSX via React 19 native custom element
 *    support + light wrappers (Button, Icon).
 *  - Tier 2 (form/event): @lit/react createComponent() wrappers that map
 *    React event props to Lit custom events (TextField, Select, Checkbox,
 *    Switch, Radio, Tabs, Dialog, Menu).
 *  - Custom: Autocomplete composes TextField + Menu for async searchable
 *    selects (replaces legacy Combobox/AccountPicker).
 */
export { Button, type ButtonVariant, type ButtonSize, type M3ButtonProps } from "./Button";
export { IconButton, type IconButtonSize, type IconButtonProps } from "./IconButton";
export { Icon, type IconProps } from "./Icon";
export {
  TextField,
  FilledTextField,
  OutlinedTextField,
  type M3TextFieldProps,
  type TextFieldVariant,
  type TextFieldEvents,
} from "./TextField";
export {
  Select,
  SelectOption,
  type SelectProps,
  type SelectOptionData,
} from "./Select";
export { Checkbox, type CheckboxProps } from "./Checkbox";
export { Switch, type SwitchProps } from "./Switch";
export { Radio, type RadioProps } from "./Radio";
export { Tabs, PrimaryTab, type TabsProps, type PrimaryTabProps } from "./Tabs";
export { Dialog, type DialogProps } from "./Dialog";
export { Menu, MenuItem, type MenuProps, type MenuItemProps } from "./Menu";
export {
  Autocomplete,
  type AutocompleteOption,
  type AutocompleteProps,
} from "./Autocomplete";
