/**
 * Shared helper for Tier 2 wrappers.
 *
 * `@lit/react`'s `createComponent()` produces component types derived from
 * the Lit element's class (all properties + omitted HTMLAttributes), which
 * frequently conflicts with the cleaner props we want to expose (e.g.
 * React-style `children`, our event signatures). Wrappers in this folder
 * define their own documented prop interfaces; this helper loosens the
 * internal component type so the public API is the single source of truth.
 */
import * as React from "react";
import { createComponent } from "@lit/react";

export interface CreateM3ComponentOptions {
  tagName: string;
  elementClass: new () => HTMLElement;
  /** React prop name -> custom event name (e.g. onChange -> "change"). */
  events?: Record<string, string>;
}

/** Create an @lit/react component with a loose internal type. */
export function createM3Component<P>(options: CreateM3ComponentOptions): React.ComponentType<P> {
  const Component = createComponent({
    react: React,
    tagName: options.tagName,
    elementClass: options.elementClass,
    events: options.events ?? {},
  });
  return Component as unknown as React.ComponentType<P>;
}
