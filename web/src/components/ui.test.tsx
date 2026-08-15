import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AmountField, EmptyState, FieldShell, FormError, TextField } from "../components/ui";

describe("TextField", () => {
  it("renders a label and input wired to onChange(value)", () => {
    const onChange = vi.fn();
    render(<TextField label="Code" value="WH-001" onChange={onChange} />);

    const input = screen.getByLabelText("Code");
    expect(input).toHaveValue("WH-001");

    fireEvent.change(input, { target: { value: "WH-002" } });
    expect(onChange).toHaveBeenCalledWith("WH-002");
  });

  it("shows hint text when provided", () => {
    render(<TextField label="Name" value="" onChange={() => {}} hint="Warehouse name" />);
    expect(screen.getByText("Warehouse name")).toBeInTheDocument();
  });

  it("marks the input invalid and describes the error when error is set", () => {
    render(<TextField label="City" value="" onChange={() => {}} error="City is required" />);
    const input = screen.getByLabelText("City");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toHaveTextContent("City is required");
  });
});

describe("AmountField", () => {
  it("displays the value with thousand separators while typing digits", () => {
    const onChange = vi.fn();
    render(<AmountField label="Total" value="1500000" onChange={onChange} />);

    const input = screen.getByLabelText("Total");
    expect(input).toHaveValue("1,500,000");
    expect(screen.getByText("Rp")).toBeInTheDocument();
  });

  it("passes only digits to onChange and strips everything else", () => {
    const onChange = vi.fn();
    render(<AmountField label="Amount" value="" onChange={onChange} />);

    fireEvent.change(screen.getByLabelText("Amount"), { target: { value: "Rp 25.000" } });
    expect(onChange).toHaveBeenCalledWith("25000");
  });

  it("caps input at 15 digits", () => {
    const onChange = vi.fn();
    render(<AmountField label="Big" value="" onChange={onChange} />);

    fireEvent.change(screen.getByLabelText("Big"), { target: { value: "1234567890123456789" } });
    expect(onChange).toHaveBeenCalledWith("123456789012345");
  });

  it("shows empty display for zero value", () => {
    render(<AmountField label="Zero" value="" onChange={() => {}} />);
    expect(screen.getByLabelText("Zero")).toHaveValue("");
  });
});

describe("FieldShell", () => {
  it("associates the label with the child control via htmlFor", () => {
    render(
      <FieldShell label="Account" htmlFor="acct-1">
        <input id="acct-1" className="input" />
      </FieldShell>,
    );
    expect(screen.getByLabelText("Account")).toHaveAttribute("id", "acct-1");
  });

  it("shows the error (and hides the hint) when error is set", () => {
    const { rerender } = render(
      <FieldShell label="Qty" htmlFor="qty" hint="Ordered quantity" error="Qty must be positive">
        <input id="qty" className="input" />
      </FieldShell>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Qty must be positive");
    expect(screen.queryByText("Ordered quantity")).not.toBeInTheDocument();

    rerender(
      <FieldShell label="Qty" htmlFor="qty" hint="Ordered quantity">
        <input id="qty" className="input" />
      </FieldShell>,
    );
    expect(screen.getByText("Ordered quantity")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("EmptyState", () => {
  it("renders title, message, and an optional action node", () => {
    render(
      <EmptyState
        title="No funds yet"
        message="Create a fund to start tracking petty cash."
        action={<button type="button">New Fund</button>}
      />,
    );
    expect(screen.getByText("No funds yet")).toBeInTheDocument();
    expect(screen.getByText("Create a fund to start tracking petty cash.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New Fund" })).toBeInTheDocument();
  });

  it("renders without an action", () => {
    render(<EmptyState title="Empty" message="Nothing here." />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("FormError", () => {
  it("renders the error message with role=alert", () => {
    render(<FormError message="All fields are required." />);
    expect(screen.getByRole("alert")).toHaveTextContent("All fields are required.");
  });

  it("renders nothing when message is null", () => {
    const { container } = render(<FormError message={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});
