import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CommandPalette } from "./CommandPalette";

const mockOpenList = vi.fn();
const mockOpenEntryDraft = vi.fn();
let mockRole = "owner";

vi.mock("../workbench/state", () => ({
  useWorkbench: () => ({
    openList: mockOpenList,
    openEntryDraft: mockOpenEntryDraft,
  }),
}));

vi.mock("../state", () => ({
  useAppState: () => ({
    business: { id: "t1", name: "T", businessType: mockRole, currency: "IDR", fiscalYearStart: 1 },
  }),
}));

describe("CommandPalette Component", () => {
  it("does not render when isOpen is false", () => {
    const { container } = render(<CommandPalette isOpen={false} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders search input and commands when isOpen is true", () => {
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    expect(screen.getByPlaceholderText(/Cari modul, transaksi/i)).toBeInTheDocument();
    expect(screen.getByText("+ Kas Masuk (Other Receipt)")).toBeInTheDocument();
  });

  it("filters command list based on search query", () => {
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);

    fireEvent.change(input, { target: { value: "Jurnal" } });
    expect(screen.getByText("+ Jurnal Umum Manual")).toBeInTheDocument();
  });

  it("executes command action and calls onClose when item is clicked", () => {
    const onClose = vi.fn();
    render(<CommandPalette isOpen={true} onClose={onClose} />);

    const item = screen.getByText("+ Kas Masuk (Other Receipt)");
    fireEvent.click(item);

    expect(mockOpenEntryDraft).toHaveBeenCalledWith("money-in");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("moves initial focus into the palette search input", () => {
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);
    expect(document.activeElement).toBe(input);
  });

  it("keeps Tab focus inside the dialog (only focusable element cycles)", () => {
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);
    for (let i = 0; i < 3; i++) {
      fireEvent.keyDown(document, { key: "Tab" });
      expect(document.activeElement).toBe(input);
    }
  });

  it("closes on Escape from anywhere inside the dialog", () => {
    const onClose = vi.fn();
    render(<CommandPalette isOpen={true} onClose={onClose} />);
    const listbox = screen.getByRole("listbox");
    fireEvent.keyDown(listbox, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("restores focus to the trigger element after closing", () => {
    const trigger = document.createElement("button");
    document.body.appendChild(trigger);
    trigger.focus();

    const { rerender } = render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    rerender(<CommandPalette isOpen={false} onClose={vi.fn()} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
  });

  it("wires combobox semantics: activedescendant follows the selected option", () => {
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);
    const options = screen.getAllByRole("option");
    expect(options.length).toBeGreaterThan(0);

    const activeId = input.getAttribute("aria-activedescendant");
    expect(activeId).toBeTruthy();
    expect(document.getElementById(activeId!)).toBe(options[0]);

    fireEvent.keyDown(input, { key: "ArrowDown" });
    const nextId = input.getAttribute("aria-activedescendant");
    expect(document.getElementById(nextId!)).toBe(options[1]);
  });

  it("shows email module commands for owner/admin roles", () => {
    mockRole = "owner";
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);
    fireEvent.change(input, { target: { value: "Email" } });
    expect(screen.getByText(/Buka Email Templates/i)).toBeInTheDocument();
  });

  it("hides email module commands for non-admin roles", () => {
    mockRole = "staff";
    render(<CommandPalette isOpen={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText(/Cari modul, transaksi/i);
    fireEvent.change(input, { target: { value: "Email" } });
    expect(screen.queryByText(/Buka Email/i)).not.toBeInTheDocument();
    // Other module commands still reachable
    fireEvent.change(input, { target: { value: "Invoice" } });
    const options = screen.getAllByRole("option");
    expect(options.length).toBeGreaterThan(0);
  });
});
