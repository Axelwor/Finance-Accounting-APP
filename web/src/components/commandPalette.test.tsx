import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CommandPalette } from "./CommandPalette";

const mockOpenList = vi.fn();
const mockOpenEntryDraft = vi.fn();

vi.mock("../workbench/state", () => ({
  useWorkbench: () => ({
    openList: mockOpenList,
    openEntryDraft: mockOpenEntryDraft,
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
});
