import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { DepositModal } from "./DepositModal";
import type { ChequeListItem } from "../../types";

const cheque: ChequeListItem = {
  id: "1",
  cheque_number: "CHQ-001",
  counterparty_name: "PT Amanah",
  amount_cents: 150000,
} as unknown as ChequeListItem;

function setup(open = true) {
  const onClose = vi.fn();
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const view = render(
    <DepositModal open={open} onClose={onClose} onSubmit={onSubmit} cheque={cheque} />,
  );
  return { onClose, onSubmit, ...view };
}

describe("DepositModal dialog semantics (F-15 #4)", () => {
  it("exposes role=dialog with aria-modal and an accessible name", () => {
    setup();
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveAccessibleName("Deposit Cheque");
  });

  it("moves initial focus into the dialog", () => {
    setup();
    const dialog = screen.getByRole("dialog");
    expect(dialog.contains(document.activeElement)).toBe(true);
  });

  it("closes on Escape and restores focus to the pre-dialog element", () => {
    const trigger = document.createElement("button");
    document.body.appendChild(trigger);
    trigger.focus();

    const { rerender, unmount } = setup(true);
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    rerender(<DepositModal open={false} onClose={() => {}} onSubmit={() => Promise.resolve()} cheque={cheque} />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
    unmount();
  });

  it("keeps Tab cycling inside the dialog", () => {
    setup();
    // The m3 buttons are custom elements (no button role in jsdom); just
    // press Tab repeatedly and assert focus never escapes the dialog.
    for (let i = 0; i < 4; i++) {
      fireEvent.keyDown(document.activeElement!, { key: "Tab" });
      expect(screen.getByRole("dialog").contains(document.activeElement)).toBe(true);
    }
  });
});
