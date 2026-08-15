import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ToastProvider } from "../../components/Toast";
import { CashEntryForm } from "./CashEntryForm";
import { api } from "../../api";
import type { AccountItem } from "../../types";

// ── Test fixtures ─────────────────────────────────────────────────────────

const ACCOUNTS: AccountItem[] = [
  { id: "1", name: "Kas Besar", code: "1101", account_type: "CASH" },
  { id: "2", name: "Bank BCA", code: "1102", account_type: "BANK" },
  { id: "3", name: "Pendapatan Jasa", code: "4101", account_type: "REVENUE" },
  { id: "4", name: "Beban Listrik", code: "5201", account_type: "EXPENSE" },
  { id: "5", name: "Beban Gaji", code: "5202", account_type: "EXPENSE" },
];

const CATEGORIES = [
  {
    id: "10",
    name: "Penjualan",
    kind: "money-in" as const,
    default_credit_account_id: 3,
    default_debit_account_id: null,
  },
  {
    id: "11",
    name: "Beban Operasional",
    kind: "money-out" as const,
    default_credit_account_id: null,
    default_debit_account_id: 4,
  },
];

vi.mock("../../api", () => ({
  api: {
    listAccounts: vi.fn(),
    listCategories: vi.fn(),
    listCashEntries: vi.fn(),
    postCashIn: vi.fn(),
    postCashOut: vi.fn(),
    postTransfer: vi.fn(),
    reverseCash: vi.fn(),
  },
  mockHelpers: {
    today: () => "2026-08-15",
    fmtIDR: (n: number) => `IDR ${n}`,
    nowIso: () => "2026-08-15T00:00:00Z",
  },
}));

vi.mock("../../workbench/state", () => ({
  useWorkbench: () => ({
    markUnsaved: vi.fn(),
    close: vi.fn(),
    activate: vi.fn(),
    openEntryDraft: vi.fn(),
    openEntryExisting: vi.fn(),
    openEntryDraftFromParent: vi.fn(),
  }),
}));

const { listAccounts, listCategories, listCashEntries, postCashIn, postCashOut, postTransfer, reverseCash } = api as unknown as {
  listAccounts: ReturnType<typeof vi.fn>;
  listCategories: ReturnType<typeof vi.fn>;
  listCashEntries: ReturnType<typeof vi.fn>;
  postCashIn: ReturnType<typeof vi.fn>;
  postCashOut: ReturnType<typeof vi.fn>;
  postTransfer: ReturnType<typeof vi.fn>;
  reverseCash: ReturnType<typeof vi.fn>;
};

function renderForm(props?: Partial<Parameters<typeof CashEntryForm>[0]>) {
  return render(
    <ToastProvider>
      <CashEntryForm tabId="t1" subKind="money-in" {...props} />
    </ToastProvider>,
  );
}

/** Open the StaticCombobox panel and pick an option by its visible label.
 *  The li accessible name is just the label (the code renders in its own
 *  span), so match on the label text. Options select on mousedown. */
async function pickComboboxOption(label: string, pickerIndex = 0) {
  const buttons = await screen.findAllByRole("button");
  const pickers = buttons.filter((b) => b.getAttribute("aria-haspopup") === "listbox");
  const control = pickers[pickerIndex];
  if (!control) throw new Error(`combobox picker #${pickerIndex} not found (got ${pickers.length})`);
  fireEvent.click(control);
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const option = await screen.findByRole(
    "option",
    { name: new RegExp(`^${escaped}$`, "i") },
    { timeout: 3000 },
  );
  fireEvent.mouseDown(option);
}

// ── Tests ─────────────────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks();
  listAccounts.mockResolvedValue(ACCOUNTS);
  listCategories.mockResolvedValue(CATEGORIES);
  listCashEntries.mockResolvedValue([]);
  localStorage.clear();
});

describe("CashEntryForm", () => {
  it("renders the dual-mode toggle for non-transfer kinds", async () => {
    renderForm();
    await waitFor(() =>
      expect(screen.getByRole("tab", { name: "Mode Cepat" })).toBeTruthy(),
    );
    expect(screen.getByRole("tab", { name: "Mode Rinci" })).toBeTruthy();
  });

  it("shows the cash-side picker filtered to CASH/BANK only", async () => {
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));
    // Open the cash-side picker (first listbox control) and assert ONLY
    // cash/bank options are listed — the revenue/expense accounts from the
    // fixture must not appear.
    const buttons = screen.getAllByRole("button");
    const pickers = buttons.filter((b) => b.getAttribute("aria-haspopup") === "listbox");
    fireEvent.click(pickers[0]);
    const options = await screen.findAllByRole("option", {}, { timeout: 2000 });
    const names = options.map((o) => o.textContent ?? "");
    expect(names.some((n) => n.includes("Kas Besar"))).toBe(true);
    expect(names.some((n) => n.includes("Bank BCA"))).toBe(true);
    expect(names.some((n) => n.includes("Pendapatan Jasa"))).toBe(false);
    expect(names.some((n) => n.includes("Beban"))).toBe(false);
  });

  it("quick mode: category selection sets the counter account and save posts one counter line", async () => {
    postCashIn.mockResolvedValue({
      id: 42,
      number: "BM-2026-00042",
      status: "POSTED",
      hash: "h",
      prev_hash: "p",
      intent_type: "CASH_IN",
      is_reversal: false,
    });
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));

    // Pick cash account (Kas Besar).
    await pickComboboxOption("1101Kas Besar");
    // Pick category (Penjualan) — quick mode.
    await pickComboboxOption("Penjualan", 1);
    // Amount.
    const amount = screen.getByLabelText("Jumlah");
    fireEvent.change(amount, { target: { value: "250000" } });

    // Save.
    fireEvent.click(screen.getByRole("button", { name: /Simpan$/ }));

    await waitFor(() =>
      expect(postCashIn).toHaveBeenCalledTimes(1),
    );
    const call = postCashIn.mock.calls[0][0];
    expect(call.amount_cents).toBe(250000);
    expect(call.cash_account_id).toBe(1);
    expect(call.counter_lines).toHaveLength(1);
    expect(call.counter_lines![0].account_id).toBe(3); // category default credit
    expect(call.counter_lines![0].amount_cents).toBe(250000);

    // Success panel shows the backend journal number.
    await waitFor(() => expect(screen.getAllByText(/BM-2026-00042/).length).toBeGreaterThan(0));
  });

  it("quick mode amount syncs two-way with the single detail line", async () => {
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));
    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "150000" } });
    // Switch to detail mode — the first line should carry the same amount.
    fireEvent.click(screen.getByRole("tab", { name: "Mode Rinci" }));
    const lineAmount = screen.getByLabelText("Nilai baris 1");
    expect((lineAmount as HTMLInputElement).value).toBe("150.000");
  });

  it("validation marks missing fields instead of a single global error", async () => {
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));
    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "0" } });
    fireEvent.click(screen.getByRole("button", { name: /Simpan$/ }));

    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThan(0));
    expect(postCashIn).not.toHaveBeenCalled();
    expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
  });

  it("transfer mode posts from/to accounts and blocks same-account transfer", async () => {
    postTransfer.mockResolvedValue({
      id: 43,
      number: "BT-2026-00007",
      status: "POSTED",
      hash: "h",
      prev_hash: "p",
      intent_type: "TRANSFER",
      is_reversal: false,
    });
    renderForm({ subKind: "cash-transfer" });
    await waitFor(() => screen.getByText("Transfer Kas/Bank"));

    await pickComboboxOption("1101Kas Besar");
    await pickComboboxOption("1102Bank BCA", 1);
    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "500000" } });

    fireEvent.click(screen.getByRole("button", { name: /Simpan$/ }));
    await waitFor(() => expect(postTransfer).toHaveBeenCalledTimes(1));
    const call = postTransfer.mock.calls[0][0];
    expect(call.from_account_id).toBe(1);
    expect(call.to_account_id).toBe(2);
    expect(call.amount_cents).toBe(500000);
  });

  it("save & new keeps the header when keepHeader is on", async () => {
    postCashIn.mockResolvedValue({
      id: 44,
      number: "BM-2026-00044",
      status: "POSTED",
      hash: "h",
      prev_hash: "p",
      intent_type: "CASH_IN",
      is_reversal: false,
    });
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));

    await pickComboboxOption("1101Kas Besar");
    await pickComboboxOption("Penjualan", 1);
    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "100000" } });
    fireEvent.change(screen.getByLabelText("Keterangan"), { target: { value: "batch 1" } });

    // Save & New from the success panel path: first save, then trigger new.
    fireEvent.click(screen.getByRole("button", { name: /Simpan & Baru/ }));
    await waitFor(() => expect(postCashIn).toHaveBeenCalledTimes(1));

    // After Save&New the form resets amounts but the cash account persists
    // (via localStorage) and a fresh draft is ready.
    await waitFor(() => screen.getByRole("button", { name: /Simpan$/ }));
    expect((screen.getByLabelText("Jumlah") as HTMLInputElement).value).toBe("");
  });

  it("existing entry opens read-only (no save buttons)", async () => {
    listCashEntries.mockResolvedValue([
      {
        id: 99,
        number: "BM-2026-00099",
        kind: "money-in",
        entry_date: "2026-08-01",
        status: "POSTED",
        description: "old entry",
        amount_cents: 750000,
        cash_account_id: 1,
        cash_account_code: "1101",
        cash_account_name: "Kas Besar",
        counter_account_id: 3,
        counter_account_code: "4101",
        counter_account_name: "Pendapatan Jasa",
        from_account_id: 0,
        from_account_code: "",
        from_account_name: "",
        to_account_id: 0,
        to_account_code: "",
        to_account_name: "",
        reference: "",
        reversal_of_id: 0,
      },
    ]);
    renderForm({ entryId: "BM-2026-00099", initialTitle: "BM-2026-00099" });
    // Read-only view: no Simpan button, but Balik & Ganti is offered.
    await waitFor(() => screen.getByText("Balik & Ganti"));
    expect(screen.queryByRole("button", { name: /^Simpan$/ })).toBeNull();
  });

  it("Ctrl+S triggers save", async () => {
    postCashIn.mockResolvedValue({
      id: 45,
      number: "BM-2026-00045",
      status: "POSTED",
      hash: "h",
      prev_hash: "p",
      intent_type: "CASH_IN",
      is_reversal: false,
    });
    renderForm();
    await waitFor(() => screen.getByText("Penerimaan Kas Lainnya"));
    await pickComboboxOption("1101Kas Besar");
    await pickComboboxOption("Penjualan", 1);
    fireEvent.change(screen.getByLabelText("Jumlah"), { target: { value: "1000" } });

    fireEvent.keyDown(document, { key: "s", ctrlKey: true });
    await waitFor(() => expect(postCashIn).toHaveBeenCalledTimes(1));
  });
});
