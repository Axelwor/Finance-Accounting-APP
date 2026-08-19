/**
 * M3 Menu — Tier 2 wrapper via `@lit/react`.
 *
 * The element exposes imperative `show()`/`close()` methods; here we drive
 * it with the `open` property (declarative) and map close events. Anchor
 * positioning requires a reference to the anchor element — pass its id via
 * `anchor`.
 */
import * as React from "react";
import type { MdMenu } from "@material/web/menu/menu.js";
import { MdMenu as MdMenuC } from "@material/web/menu/menu.js";
import { createM3Component } from "./createM3Component";
import "@material/web/menu/menu.js";
import "@material/web/menu/menu-item.js";

export interface MenuInternalProps {
  open?: boolean;
  anchor?: string;
  positioning?: "absolute" | "fixed" | "document" | "popover";
  "menu-corner"?: "START" | "END";
  corner?: "START_START" | "START_END" | "END_START" | "END_END";
  xOffset?: number;
  yOffset?: number;
  typeaheadDelay?: number;
  onClose?: (e: Event) => void;
  onOpened?: (e: Event) => void;
  onCanceled?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

const MdMenuComponent = createM3Component<MenuInternalProps>({
  tagName: "md-menu",
  elementClass: MdMenuC as unknown as new () => HTMLElement,
  events: {
    onClose: "close",
    onOpened: "opened",
    onCanceled: "canceled",
  },
});

export interface MenuProps {
  open?: boolean;
  /** id of the anchor element. */
  anchor?: string;
  positioning?: "absolute" | "fixed" | "document" | "popover";
  corner?: "START_START" | "START_END" | "END_START" | "END_END";
  menuCorner?: "START" | "END";
  xOffset?: number;
  yOffset?: number;
  typeaheadDelay?: number;
  onClose?: (e: Event) => void;
  onOpened?: (e: Event) => void;
  onCanceled?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

/** Declaratively-controlled md-menu. */
export function Menu({
  open,
  anchor,
  positioning,
  corner,
  menuCorner,
  xOffset,
  yOffset,
  typeaheadDelay,
  onClose,
  onOpened,
  onCanceled,
  className,
  style,
  children,
}: MenuProps) {
  return (
    <MdMenuComponent
      open={open}
      anchor={anchor}
      positioning={positioning}
      corner={corner}
      menu-corner={menuCorner}
      xOffset={xOffset}
      yOffset={yOffset}
      typeaheadDelay={typeaheadDelay}
      onClose={onClose}
      onOpened={onOpened}
      onCanceled={onCanceled}
      className={className}
      style={style}
    >
      {children}
    </MdMenuComponent>
  );
}

export interface MenuItemProps {
  /** Keyboard shortcut hint shown at the end. */
  shortcut?: React.ReactNode;
  /** Trailing content (submenu caret, checkbox, etc). */
  trailing?: React.ReactNode;
  /** Leading icon content. */
  leading?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  onclick?: React.MouseEventHandler;
  disabled?: boolean;
}

/** md-menu-item with slot helpers. */
export function MenuItem({
  shortcut,
  trailing,
  leading,
  className,
  style,
  children,
  onclick,
  disabled,
}: MenuItemProps) {
  return (
    <md-menu-item className={className} style={style} onclick={onclick} disabled={disabled}>
      {leading ? <div slot="headline">{leading}</div> : null}
      {children}
      {shortcut ? <span slot="shortcut">{shortcut}</span> : null}
      {trailing ? <span slot="item-end">{trailing}</span> : null}
    </md-menu-item>
  );
}

export type { MdMenu };
