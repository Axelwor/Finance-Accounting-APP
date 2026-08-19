/**
 * M3 Dialog — Tier 2 wrapper via `@lit/react`.
 *
 * The element exposes imperative `show()`/`close()` methods; here we drive
 * it declaratively with the `open` property instead (React-friendly), and
 * map the close/cancel custom events.
 *
 * Content slots: headline, content (default), actions.
 */
import * as React from "react";
import type { MdDialog } from "@material/web/dialog/dialog.js";
import { MdDialog as MdDialogC } from "@material/web/dialog/dialog.js";
import { createM3Component } from "./createM3Component";
import "@material/web/dialog/dialog.js";

export interface DialogInternalProps {
  open?: boolean;
  quickDismiss?: boolean;
  onClose?: (e: Event) => void;
  onCancel?: (e: Event) => void;
  onOpen?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
}

const MdDialogComponent = createM3Component<DialogInternalProps>({
  tagName: "md-dialog",
  elementClass: MdDialogC as unknown as new () => HTMLElement,
  events: {
    onClose: "close",
    onCancel: "cancel",
    onOpen: "open",
  },
});

export interface DialogProps {
  /** Whether the dialog is open (declarative control). */
  open?: boolean;
  /** Hides the cancel affordance (scrim click / Esc) when false. */
  quickDismiss?: boolean;
  onClose?: (e: Event) => void;
  onCancel?: (e: Event) => void;
  className?: string;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  /** Rendered into the headline slot. */
  headline?: React.ReactNode;
  /** Rendered into the actions slot (buttons). */
  actions?: React.ReactNode;
}

/** Declaratively-controlled md-dialog. */
export function Dialog({
  open,
  quickDismiss,
  onClose,
  onCancel,
  className,
  style,
  children,
  headline,
  actions,
}: DialogProps) {
  return (
    <MdDialogComponent
      open={open}
      quickDismiss={quickDismiss}
      onClose={onClose}
      onCancel={onCancel}
      className={className}
      style={style}
    >
      {headline ? <div slot="headline">{headline}</div> : null}
      {children}
      {actions ? <div slot="actions">{actions}</div> : null}
    </MdDialogComponent>
  );
}

export type { MdDialog };
