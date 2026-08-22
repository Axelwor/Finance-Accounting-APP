import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ToastProvider } from "../../components/Toast";
import { CashEntryForm } from "./CashEntryForm";
import { api } from "../../api";
import type { BackendAccount } from "../../types";

const ACCOUNTS: BackendAccount[] = [
  { id: 1, name: "Kas Besar", code: "1101", account_type: "CASH", report_group: "ASSET", parent_id: null, is_group: false, is_active: true, valid_from: null, valid_to: null },
  { id: 2, name: "Bank BCA", code: "1102", account_type: "BANK", report_group: "ASSET", parent_id: null, is_group: false, is_active: true, valid_from: null, valid_to: null },
  { id: 3, name: "Pendapatan Jasa", code: "4101", account_type: "REVENUE", report_group: "REVENUE", parent_id: null, is_group: false, is_active: true, valid_from: null, valid_to: null },
  { id: 4, name: "Beban Listrik", code: "5201", account_type: "EXPENSE", report_group: "EXPENSE", parent_id: null, is_group: false, is_active: true, valid_from: null, valid_to: null },
];

vi.mock("../../api", () => ({
  api: {
    listBackendAccounts: vi.fn(),
    listRecentJournalEntries: vi.fn().mockResolvedValue([]),
    postCashIn: vi.fn(),
    postCashOut: vi.fn(),
  },
  mockHelpers: {
    today: () => "2026-08-22",
  },
}));

vi.mock("../../workbench/state", () => ({
  useWorkbench: () => ({
    close: vi.fn(),
  }),
}));

const { listBackendAccounts, postCashIn, postCashOut } = api as unknown as {
  listBackendAccounts: ReturnType<typeof vi.fn>;
  postCashIn: ReturnType<typeof vi.fn>;
  postCashOut: ReturnType<typeof vi.fn>;
};

function renderForm(props?: Partial<Parameters<typeof CashEntryForm>[0]>) {
  return render(
    <ToastProvider>
      <CashEntryForm tabId="t1" subKind="money-in" {...props} />
    </ToastProvider>
  );
}

describe("CashEntryForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listBackendAccounts.mockResolvedValue(ACCOUNTS);
  });

  it("renders the 3-Zone Enterprise Header & Form fields", async () => {
    renderForm();
    await waitFor(() => {
      expect(screen.getByText(/Kas Masuk Operasional/i)).toBeTruthy();
    });
    expect(screen.getByText(/BKM-2026\/DRAFT/i)).toBeTruthy();
    expect(screen.getByText(/Alokasi Akun Penerimaan/i)).toBeTruthy();
  });

  it("posts cash-in when valid data is entered", async () => {
    postCashIn.mockResolvedValue({
      id: 101,
      number: "BKM-2026/08/0001",
      status: "POSTED",
    });

    renderForm();
    await waitFor(() => {
      expect(screen.getByText(/Kas Masuk Operasional/i)).toBeTruthy();
    });

    // Fill counter line amount
    const amountInputs = screen.getAllByPlaceholderText("0");
    fireEvent.change(amountInputs[0], { target: { value: "500000" } });

    // Pick counter account
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[1], { target: { value: "3" } });

    // Submit form
    const submitBtn = screen.getByRole("button", { name: /POSTING TRANSAKSI KAS/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(postCashIn).toHaveBeenCalledTimes(1);
    });
  });
});
