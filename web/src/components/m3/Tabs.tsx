/**
 * M3 Tabs — Tier 2 wrapper via `@lit/react`.
 *
 * `md-tabs` is a controlled container: React props map to the element's
 * `activeTabIndex` property, and the `change` event fires on user selection
 * (React 19 does not map custom events, hence the wrapper).
 *
 * Tabs themselves (`md-primary-tab`) are Tier 1 display elements used as
 * direct children — content goes in the default slot, icons in
 * `slot="icon"`.
 */
import * as React from "react";
import type { MdTabs } from "@material/web/tabs/tabs.js";
import { MdTabs as MdTabsC } from "@material/web/tabs/tabs.js";
import { createM3Component } from "./createM3Component";
import "@material/web/tabs/tabs.js";
import "@material/web/tabs/primary-tab.js";

export interface TabsInternalProps {
  activeTabIndex?: number;
  onChange?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

const MdTabsComponent = createM3Component<TabsInternalProps>({
  tagName: "md-tabs",
  elementClass: MdTabsC as unknown as new () => HTMLElement,
  events: {
    onChange: "change",
  },
});

export interface TabsProps {
  /** Index of the active tab (controlled). */
  activeTabIndex: number;
  onChange?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

/** Controlled md-tabs container. Children must be md-primary-tab elements. */
export function Tabs({ activeTabIndex, onChange, className, style, children }: TabsProps) {
  return (
    <MdTabsComponent activeTabIndex={activeTabIndex} onChange={onChange} className={className} style={style}>
      {children}
    </MdTabsComponent>
  );
}

export interface PrimaryTabProps {
  /** Whether this tab is the active one (mirrors the container's index). */
  active?: boolean;
  /** Click handler (native click on the tab element). */
  onclick?: (e: React.MouseEvent) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  /** Optional icon content, rendered in the icon slot. */
  icon?: React.ReactNode;
}

/**
 * md-primary-tab — content in the default slot, icon in the icon slot.
 * The container (md-tabs) owns the active state; `active` is set for
 * CSS/ARIA correctness.
 */
export function PrimaryTab({ active, onclick, className, style, children, icon }: PrimaryTabProps) {
  return (
    <md-primary-tab active={active} onclick={onclick} className={className} style={style}>
      {icon ? <span slot="icon">{icon}</span> : null}
      {children}
    </md-primary-tab>
  );
}

export type { MdTabs };
