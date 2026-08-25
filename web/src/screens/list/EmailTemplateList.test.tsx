import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { EmailTemplateList } from "./EmailTemplateList";
import { api } from "../../api";
import type { EmailTemplate } from "../../types";

vi.mock("../../api", () => ({
  api: {
    listEmailTemplates: vi.fn(),
    createEmailTemplate: vi.fn(),
    updateEmailTemplate: vi.fn(),
    deleteEmailTemplate: vi.fn(),
  },
}));

// useTabRefresh (list refetch on tab activation) requires WorkbenchProvider,
// which this isolated component test does not mount.
vi.mock("../../workbench/useTabRefresh", () => ({
  useTabRefresh: () => {},
}));

const { listEmailTemplates } = api as unknown as {
  listEmailTemplates: ReturnType<typeof vi.fn>;
};

function makeTemplate(overrides?: Partial<EmailTemplate>): EmailTemplate {
  return {
    id: 1,
    tenant_id: 1,
    subject: "Invoice Notification",
    body_html: "<p>Thank you</p>",
    body_text: "Thank you",
    trigger_event: "INVOICE_SENT",
    is_active: true,
    created_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

describe("EmailTemplateList preview sanitization (F-03)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("strips <script> tags when previewing a stored template", async () => {
    listEmailTemplates.mockResolvedValue([
      makeTemplate({
        body_html: '<p>Hello</p><script>window.alert("xss")</script>',
      }),
    ]);

    render(<EmailTemplateList />);
    await waitFor(() => expect(screen.getByText("Invoice Notification")).toBeTruthy());

    fireEvent.click(screen.getByText("Edit"));

    const preview = document.querySelector(".email-preview");
    expect(preview).not.toBeNull();
    expect(preview!.querySelector("script")).toBeNull();
    expect(preview!.innerHTML).toContain("Hello");
  });

  it("removes inline event handlers and javascript: URLs from the preview", async () => {
    listEmailTemplates.mockResolvedValue([
      makeTemplate({
        body_html:
          '<img src=x onerror="alert(1)"><a href="javascript:alert(2)">link</a>',
      }),
    ]);

    render(<EmailTemplateList />);
    await waitFor(() => expect(screen.getByText("Invoice Notification")).toBeTruthy());

    fireEvent.click(screen.getByText("Edit"));

    const preview = document.querySelector(".email-preview")!;
    expect(preview.querySelector("[onerror]")).toBeNull();
    expect(preview.querySelector('a[href^="javascript:"]')).toBeNull();
  });
});
