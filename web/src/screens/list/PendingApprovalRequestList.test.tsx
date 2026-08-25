import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PendingApprovalRequestList } from "./PendingApprovalRequestList";
import { api } from "../../api";
import type { ApprovalRequest } from "../../types";

vi.mock("../../api", () => ({
  api: {
    listApprovalRequests: vi.fn(),
    approveApprovalRequest: vi.fn(),
    rejectApprovalRequest: vi.fn(),
  },
}));

// useTabRefresh requires WorkbenchProvider which this isolated test does not mount.
vi.mock("../../workbench/useTabRefresh", () => ({
  useTabRefresh: () => {},
}));

const { listApprovalRequests, approveApprovalRequest, rejectApprovalRequest } =
  api as unknown as {
    listApprovalRequests: ReturnType<typeof vi.fn>;
    approveApprovalRequest: ReturnType<typeof vi.fn>;
    rejectApprovalRequest: ReturnType<typeof vi.fn>;
  };

function makeRequest(overrides?: Partial<ApprovalRequest>): ApprovalRequest {
  return {
    id: 11,
    entity_type: "invoice",
    entity_id: 42,
    entity_number: "INV-2026-000042",
    requested_by: 3,
    requested_at: "2026-08-25T10:00:00Z",
    status: "PENDING",
    amount_cents: 250000000,
    ...overrides,
  };
}

describe("PendingApprovalRequestList (backend /approval-requests contract)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads PENDING requests with the status filter and renders entity_number", async () => {
    listApprovalRequests.mockResolvedValue([makeRequest()]);

    render(<PendingApprovalRequestList />);

    await waitFor(() =>
      expect(listApprovalRequests).toHaveBeenCalledWith({ status: "PENDING" }),
    );
    await waitFor(() =>
      expect(screen.getByText("INV-2026-000042")).toBeTruthy(),
    );
    expect(screen.getByText("Sales Invoice")).toBeTruthy();
  });

  it("shows the empty state when nothing is pending", async () => {
    listApprovalRequests.mockResolvedValue([]);

    render(<PendingApprovalRequestList />);

    await waitFor(() =>
      expect(screen.getByText("No pending approvals")).toBeTruthy(),
    );
  });

  it("approves after confirmation and reloads the list", async () => {
    listApprovalRequests.mockResolvedValue([makeRequest()]);
    approveApprovalRequest.mockResolvedValue(undefined);

    render(<PendingApprovalRequestList />);
    await waitFor(() => expect(screen.getByText("Approve")).toBeTruthy());

    fireEvent.click(screen.getByText("Approve"));
    fireEvent.click(screen.getByText("Confirm Approve"));

    await waitFor(() =>
      expect(approveApprovalRequest).toHaveBeenCalledWith(11, { reason: "" }),
    );
    // Reload after success.
    await waitFor(() => expect(listApprovalRequests).toHaveBeenCalledTimes(2));
  });

  it("blocks rejection without a reason and sends the reason once provided", async () => {
    listApprovalRequests.mockResolvedValue([makeRequest()]);
    rejectApprovalRequest.mockResolvedValue(undefined);

    render(<PendingApprovalRequestList />);
    await waitFor(() => expect(screen.getByText("Reject")).toBeTruthy());

    fireEvent.click(screen.getByText("Reject"));
    fireEvent.click(screen.getByText("Confirm Reject"));

    await waitFor(() =>
      expect(screen.getByText("Reason is required for rejection.")).toBeTruthy(),
    );
    expect(rejectApprovalRequest).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/Reason/), {
      target: { value: "Harga tidak sesuai PO" },
    });
    fireEvent.click(screen.getByText("Confirm Reject"));

    await waitFor(() =>
      expect(rejectApprovalRequest).toHaveBeenCalledWith(11, {
        reason: "Harga tidak sesuai PO",
      }),
    );
  });
});
