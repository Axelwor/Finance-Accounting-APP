import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ApprovalRuleList } from "./ApprovalRuleList";
import { api } from "../../api";
import type { ApprovalWorkflow } from "../../types";

vi.mock("../../api", () => ({
  api: {
    listApprovalWorkflows: vi.fn(),
    createApprovalWorkflow: vi.fn(),
    deleteApprovalWorkflow: vi.fn(),
  },
}));

const { listApprovalWorkflows, createApprovalWorkflow } = api as unknown as {
  listApprovalWorkflows: ReturnType<typeof vi.fn>;
  createApprovalWorkflow: ReturnType<typeof vi.fn>;
  deleteApprovalWorkflow: ReturnType<typeof vi.fn>;
};

function makeWorkflow(overrides?: Partial<ApprovalWorkflow>): ApprovalWorkflow {
  return {
    id: 1,
    entity_type: "invoice",
    min_amount_cents: 500000000,
    approver_role: "accountant",
    is_active: true,
    ...overrides,
  };
}

describe("ApprovalRuleList (backend /approval-workflows contract)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders workflows with entity label and rupiah amount from cents", async () => {
    listApprovalWorkflows.mockResolvedValue([
      makeWorkflow({ entity_type: "purchase_order", approver_role: "manager" }),
    ]);

    render(<ApprovalRuleList />);

    await waitFor(() => expect(screen.getByText("Purchase Order")).toBeTruthy());
    // 5.000.000 rupiah stored as 500000000 cents.
    expect(screen.getByText(/5\.000\.000|5,000,000/)).toBeTruthy();
    expect(screen.getByText("manager")).toBeTruthy();
  });

  it("shows the empty state when no workflows exist", async () => {
    listApprovalWorkflows.mockResolvedValue([]);

    render(<ApprovalRuleList />);

    await waitFor(() =>
      expect(screen.getByText("No approval rules")).toBeTruthy(),
    );
  });

  it("creates a workflow via POST upsert converting rupiah input to cents", async () => {
    listApprovalWorkflows.mockResolvedValue([]);
    createApprovalWorkflow.mockResolvedValue(makeWorkflow({ id: 7 }));

    render(<ApprovalRuleList />);
    await waitFor(() => expect(screen.getByText("+ New Rule")).toBeTruthy());

    fireEvent.click(screen.getByText("+ New Rule"));
    fireEvent.change(screen.getByPlaceholderText(/0 = any amount/), {
      target: { value: "5.000.000" },
    });
    fireEvent.click(screen.getByText("Save"));

    await waitFor(() =>
      expect(createApprovalWorkflow).toHaveBeenCalledWith({
        entity_type: "invoice",
        min_amount_cents: 500000000,
        approver_role: "accountant",
      }),
    );
    // List refetched after save.
    await waitFor(() => expect(listApprovalWorkflows).toHaveBeenCalledTimes(2));
  });

  it("surfaces backend error messages via err.message in the form", async () => {
    listApprovalWorkflows.mockResolvedValue([]);
    createApprovalWorkflow.mockRejectedValue(
      new Error("approver_role must be one of: admin, accountant, manager"),
    );

    render(<ApprovalRuleList />);
    await waitFor(() => expect(screen.getByText("+ New Rule")).toBeTruthy());
    fireEvent.click(screen.getByText("+ New Rule"));
    fireEvent.change(screen.getByPlaceholderText(/0 = any amount/), {
      target: { value: "1000" },
    });
    fireEvent.click(screen.getByText("Save"));

    await waitFor(() =>
      expect(
        screen.getByText(/approver_role must be one of/),
      ).toBeTruthy(),
    );
  });
});
