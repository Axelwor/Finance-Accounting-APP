import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TabStrip } from "./TabStrip";
import { NestedTabStrip } from "./NestedTabStrip";
import type { Tab, NestedTab } from "./types";

const activate = vi.fn();
const close = vi.fn();

vi.mock("./state", () => ({
  useWorkbench: () => ({ tabs, activeId, activate, close }),
}));

let tabs: Tab[] = [];
let activeId: string | null = null;

function tab(id: string, title: string): Tab {
  return {
    id,
    moduleId: "cash",
    kind: "list",
    subKind: "journal",
    title,
    createdAt: 1,
  } as unknown as Tab;
}

describe("TabStrip keyboard support (F-15 #1)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    tabs = [tab("t1", "General Journal"), tab("t2", "Cash & Bank")];
    activeId = "t1";
  });

  it("gives the active pill roving tabindex and makes pills focusable", () => {
    render(<TabStrip />);
    const pills = screen.getAllByRole("tab");
    expect(pills[0]).toHaveAttribute("tabindex", "0");
    expect(pills[1]).toHaveAttribute("tabindex", "-1");
  });

  it("activates a focused pill with Enter and Space", () => {
    render(<TabStrip />);
    const pill = screen.getAllByRole("tab")[0];
    fireEvent.keyDown(pill, { key: "Enter" });
    expect(activate).toHaveBeenCalledWith("t1");
    fireEvent.keyDown(pill, { key: " " });
    expect(activate).toHaveBeenCalledTimes(2);
  });

  it("ArrowRight moves activation and focus to the next pill", () => {
    const { container } = render(<TabStrip />);
    const pill = screen.getAllByRole("tab")[0];
    fireEvent.keyDown(pill, { key: "ArrowRight" });
    expect(activate).toHaveBeenCalledWith("t2");
    const next = container.querySelector<HTMLElement>('[data-tab-index="1"]');
    expect(document.activeElement).toBe(next);
  });

  it("wraps at the edges (End → last, ArrowLeft on first → last)", () => {
    const { container } = render(<TabStrip />);
    fireEvent.keyDown(screen.getAllByRole("tab")[0], { key: "End" });
    expect(activate).toHaveBeenCalledWith("t2");
    expect(document.activeElement).toBe(container.querySelector<HTMLElement>('[data-tab-index="1"]'));

    fireEvent.keyDown(container.querySelector<HTMLElement>('[data-tab-index="1"]')!, {
      key: "ArrowRight",
    });
    expect(activate).toHaveBeenLastCalledWith("t1");
    expect(document.activeElement).toBe(container.querySelector<HTMLElement>('[data-tab-index="0"]'));
  });
});

describe("NestedTabStrip roving index attribute (F-15 #2)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  function nested(id: string, kind: "list" | "entry"): NestedTab {
    return {
      id,
      kind,
      moduleId: "sales",
      title: id,
      createdAt: 1,
    } as NestedTab;
  }

  it("stores the array index (not the roving tabindex) in data-nested-tab-index", () => {
    // Lists sort before entries; entry is index 0 in given order? No —
    // sortedChildren puts list first regardless of props order.
    render(
      <NestedTabStrip parentId="m" children={[nested("entry-1", "entry"), nested("list-1", "list")]} activeChildId="list-1" />,
    );
    const pills = screen.getAllByRole("tab");
    expect(pills[0].getAttribute("data-nested-tab-index")).toBe("0");
    expect(pills[1].getAttribute("data-nested-tab-index")).toBe("1");
    expect(pills[0]).toHaveAttribute("tabindex", "0");
    expect(pills[1]).toHaveAttribute("tabindex", "-1");
  });

  it("focus follows the arrow-key target via data-nested-tab-index", () => {
    const { container } = render(
      <NestedTabStrip parentId="m" children={[nested("list-1", "list"), nested("entry-1", "entry")]} activeChildId="list-1" />,
    );
    const first = screen.getAllByRole("tab")[0];
    fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(activate).toHaveBeenCalledWith("entry-1");
    expect(document.activeElement).toBe(container.querySelector('[data-nested-tab-index="1"]'));
  });
});
