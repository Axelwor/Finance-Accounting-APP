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
    postTransfer: vi.fn(),
  },
  mockHelpers: {
    today: () => "2026-08-22",
  },
}));

vi.mock("../../workbench/state", () => ({
  useWorkbench: () => ({
    close: vi.fn(),
    activeNested: null,
    markUnsaved: vi.fn(),
  }),
}));

const { listBackendAccounts, postCashIn, postCashOut, postTransfer } = api as unknown as {
  listBackendAccounts: ReturnType<typeof vi.fn>;
  postCashIn: ReturnType<typeof vi.fn>;
  postCashOut: ReturnType<typeof vi.fn>;
  postTransfer: ReturnType<typeof vi.fn>;
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
    localStorage.clear();
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

  it("converts rupiah input to integer cents on submit (×100)", async () => {
    postCashIn.mockResolvedValue({
      id: 101,
      number: "BKM-2026/08/0001",
      status: "POSTED",
    });

    renderForm();
    await waitFor(() => {
      expect(screen.getByText(/Kas Masuk Operasional/i)).toBeTruthy();
    });

    // Fill counter line amount: user types rupiah (Rp 500.000).
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
    // Backend stores cents: Rp 500.000 -> 50_000_000.
    expect(postCashIn).toHaveBeenCalledWith(
      expect.objectContaining({ amount_cents: 50000000 })
    );
    expect(postCashIn.mock.calls[0][0].counter_lines[0].amount_cents).toBe(50000000);
  });

  it("rejects a negative nominal with a clear error (F-14)", async () => {
    renderForm();
    await waitFor(() => {
      expect(screen.getByText(/Kas Masuk Operasional/i)).toBeTruthy();
    });

    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[1], { target: { value: "3" } });
    const amountInputs = screen.getAllByPlaceholderText("0");
    fireEvent.change(amountInputs[0], { target: { value: "-5000" } });

    const submitBtn = screen.getByRole("button", { name: /POSTING TRANSAKSI KAS/i });
    fireEvent.click(submitBtn);

    expect(await screen.findByText(/Nominal tidak boleh negatif\./i)).toBeTruthy();
    expect(postCashIn).not.toHaveBeenCalled();
  });

  it("posts a bank transfer Sumber→Tujuan via /transfers (F-04)", async () => {
    postTransfer.mockResolvedValue({ id: 9, number: "BTR-2026/08/0001", status: "POSTED" });

    renderForm({ subKind: "cash-transfer" });
    await waitFor(() => {
      expect(screen.getByText(/Transfer Kas \/ Bank/i)).toBeTruthy();
    });

    // Source stays the default cash account; pick the destination.
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[1], { target: { value: "2" } });

    const amountInput = screen.getByPlaceholderText("0");
    fireEvent.change(amountInput, { target: { value: "250000" } });

    fireEvent.click(screen.getByRole("button", { name: /POSTING TRANSFER KAS/i }));

    await waitFor(() => {
      expect(postTransfer).toHaveBeenCalledTimes(1);
    });
    expect(postTransfer).toHaveBeenCalledWith(
      expect.objectContaining({
        from_account_id: 1,
        to_account_id: 2,
        amount_cents: 25000000,
      })
    );
    expect(postCashIn).not.toHaveBeenCalled();
    expect(postCashOut).not.toHaveBeenCalled();
  });
});
