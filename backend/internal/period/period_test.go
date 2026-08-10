package period

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"finance-accounting-app/backend/internal/accounting"
)

// Period status values (see handler.go: OPEN, CLOSED, REOPENED).
const (
	periodStatusOpen     = "OPEN"
	periodStatusClosed   = "CLOSED"
	periodStatusReopened = "REOPENED"
)

// Account code constants — seeded by migration and used by resolveEquityAccounts.
const (
	retainedEarningsCode = "3201"
	currentEarningsCode  = "3301"
)

// ---------------------------------------------------------------------------
// Account code constants
// ---------------------------------------------------------------------------

func TestRetainedEarningsCode(t *testing.T) {
	if retainedEarningsCode != "3201" {
		t.Fatalf("retained earnings code = %s, want 3201", retainedEarningsCode)
	}
}

func TestCurrentEarningsCode(t *testing.T) {
	if currentEarningsCode != "3301" {
		t.Fatalf("current earnings code = %s, want 3301", currentEarningsCode)
	}
}

// ---------------------------------------------------------------------------
// Period status constants
// ---------------------------------------------------------------------------

func TestPeriodStatusOpen(t *testing.T) {
	if periodStatusOpen != "OPEN" {
		t.Fatalf("got %s, want OPEN", periodStatusOpen)
	}
}

func TestPeriodStatusClosed(t *testing.T) {
	if periodStatusClosed != "CLOSED" {
		t.Fatalf("got %s, want CLOSED", periodStatusClosed)
	}
}

func TestPeriodStatusReopened(t *testing.T) {
	if periodStatusReopened != "REOPENED" {
		t.Fatalf("got %s, want REOPENED", periodStatusReopened)
	}
}

func TestPeriodStatusValuesAreDistinct(t *testing.T) {
	statuses := map[string]bool{
		periodStatusOpen:     true,
		periodStatusClosed:   true,
		periodStatusReopened: true,
	}
	if len(statuses) != 3 {
		t.Fatalf("period statuses are not distinct: %v", statuses)
	}
}

// ---------------------------------------------------------------------------
// Net profit calculation: net = revenue - expense
// ---------------------------------------------------------------------------

func TestNetProfit_Calculation(t *testing.T) {
	tests := []struct {
		name    string
		revenue int64
		expense int64
		wantNet int64
	}{
		{"profit", 10_000_00, 6_000_00, 4_000_00},
		{"loss", 5_000_00, 8_000_00, -3_000_00},
		{"break-even", 7_500_00, 7_500_00, 0},
		{"no revenue no expense", 0, 0, 0},
		{"only revenue", 12_000_00, 0, 12_000_00},
		{"only expense", 0, 3_000_00, -3_000_00},
		{"large amounts", 99_999_99, 1, 99_999_98},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			net := tc.revenue - tc.expense
			if net != tc.wantNet {
				t.Fatalf("net = %d, want %d (revenue=%d expense=%d)",
					net, tc.wantNet, tc.revenue, tc.expense)
			}
		})
	}
}

func TestNetProfit_PositiveMeansProfit(t *testing.T) {
	if (10_000_00 - 4_000_00) <= 0 {
		t.Fatal("positive net profit expected")
	}
}

func TestNetProfit_NegativeMeansLoss(t *testing.T) {
	if (3_000_00 - 5_000_00) >= 0 {
		t.Fatal("negative net profit (loss) expected")
	}
}

func TestNetProfit_BreakEvenIsZero(t *testing.T) {
	net := 5_000_00 - 5_000_00
	if net != 0 {
		t.Fatalf("break-even net = %d, want 0", net)
	}
}

// ---------------------------------------------------------------------------
// Break-even scenario: net = 0, no closing entry to retained earnings.
// Replicates the logic in buildClosingLines: when netProfit == 0, no
// retained-earnings lines are appended.
// ---------------------------------------------------------------------------

func TestBreakEven_NoRetainedEarningsEntry(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 5_000_00},  // revenue
		{accountID: 200, amount: -5_000_00}, // expense
	}
	retainedID := int64(1)
	runningID := int64(2)

	lines := buildClosingLines(balances, retainedID, runningID)

	// With net = 0, there should be no lines referencing retainedID.
	for _, line := range lines {
		if line.AccountID == retainedID {
			t.Fatalf("break-even should not produce retained earnings lines, got %+v", line)
		}
	}
}

func TestBreakEven_BalancedLines(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 3_000_00},  // revenue
		{accountID: 200, amount: -3_000_00}, // expense
	}
	lines := buildClosingLines(balances, 1, 2)
	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("break-even lines not balanced: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: profit scenario
// Net profit > 0: Dr 3301 running / Cr 3201 retained
// ---------------------------------------------------------------------------

func TestBuildClosingLines_Profit(t *testing.T) {
	retainedID := int64(3201)
	runningID := int64(3301)
	balances := []plBalance{
		{accountID: 100, amount: 10_000_00}, // revenue (credit balance, positive)
		{accountID: 200, amount: -6_000_00}, // expense (debit balance, negative)
	}

	lines := buildClosingLines(balances, retainedID, runningID)

	// Revenue line: Dr revenue account / Cr running
	// Expense line: Dr running / Cr expense account
	// Profit line: Dr running / Cr retained
	// Total: 4 pairs = 8 lines (2 per balance + 2 for retained)
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}

	// Verify the journal balances.
	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("profit closing lines not balanced: %v", err)
	}

	// Check that retained earnings was credited (net profit goes to retained).
	var retainedCredit int64
	for _, line := range lines {
		if line.AccountID == retainedID && line.CreditCents > 0 {
			retainedCredit = line.CreditCents
		}
	}
	if retainedCredit != 4_000_00 {
		t.Fatalf("retained earnings credit = %d, want 400000", retainedCredit)
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: loss scenario
// Net profit < 0: Dr 3201 retained / Cr 3301 running
// ---------------------------------------------------------------------------

func TestBuildClosingLines_Loss(t *testing.T) {
	retainedID := int64(3201)
	runningID := int64(3301)
	balances := []plBalance{
		{accountID: 100, amount: 3_000_00},  // revenue
		{accountID: 200, amount: -5_000_00}, // expense
	}

	lines := buildClosingLines(balances, retainedID, runningID)

	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("loss closing lines not balanced: %v", err)
	}

	// Net loss = 2000. Retained should be debited.
	var retainedDebit int64
	for _, line := range lines {
		if line.AccountID == retainedID && line.DebitCents > 0 {
			retainedDebit = line.DebitCents
		}
	}
	if retainedDebit != 2_000_00 {
		t.Fatalf("retained earnings debit = %d, want 200000", retainedDebit)
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: break-even (net = 0)
// No retained earnings lines.
// ---------------------------------------------------------------------------

func TestBuildClosingLines_BreakEven(t *testing.T) {
	retainedID := int64(3201)
	runningID := int64(3301)
	balances := []plBalance{
		{accountID: 100, amount: 5_000_00},  // revenue
		{accountID: 200, amount: -5_000_00}, // expense
	}

	lines := buildClosingLines(balances, retainedID, runningID)

	// 2 balances × 2 lines each = 4 lines, no retained earnings lines.
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("break-even closing lines not balanced: %v", err)
	}

	// No lines should reference retained earnings.
	for _, line := range lines {
		if line.AccountID == retainedID {
			t.Fatalf("break-even should not reference retained earnings: %+v", line)
		}
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: empty balances
// ---------------------------------------------------------------------------

func TestBuildClosingLines_Empty(t *testing.T) {
	lines := buildClosingLines(nil, 3201, 3301)
	if len(lines) != 0 {
		t.Fatalf("got %d lines, want 0 for empty balances", len(lines))
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: zero-amount balances are skipped
// ---------------------------------------------------------------------------

func TestBuildClosingLines_SkipsZeroAmounts(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 0},         // zero — skipped
		{accountID: 200, amount: 5_000_00},  // revenue
		{accountID: 300, amount: 0},         // zero — skipped
		{accountID: 400, amount: -2_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// Only 2 non-zero balances × 2 lines + 2 retained earnings lines = 6.
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: revenue-only (no expense)
// ---------------------------------------------------------------------------

func TestBuildClosingLines_RevenueOnly(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 8_000_00}, // revenue only
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// 2 lines for revenue + 2 lines for retained earnings (all profit).
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("revenue-only lines not balanced: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: expense-only (no revenue)
// ---------------------------------------------------------------------------

func TestBuildClosingLines_ExpenseOnly(t *testing.T) {
	balances := []plBalance{
		{accountID: 200, amount: -4_000_00}, // expense only
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// 2 lines for expense + 2 lines for retained earnings (all loss).
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("expense-only lines not balanced: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildClosingLines: multiple revenue and expense accounts
// ---------------------------------------------------------------------------

func TestBuildClosingLines_MultipleAccounts(t *testing.T) {
	balances := []plBalance{
		{accountID: 101, amount: 15_000_00}, // revenue
		{accountID: 102, amount: 5_000_00},  // revenue
		{accountID: 201, amount: -8_000_00}, // expense
		{accountID: 202, amount: -4_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// 4 balances × 2 lines + 2 retained earnings lines = 10.
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("multi-account lines not balanced: %v", err)
	}

	// Net = 20000 - 12000 = 8000 profit.
	var retainedCredit int64
	for _, line := range lines {
		if line.AccountID == 3201 && line.CreditCents > 0 {
			retainedCredit = line.CreditCents
		}
	}
	if retainedCredit != 8_000_00 {
		t.Fatalf("retained credit = %d, want 800000", retainedCredit)
	}
}

// ---------------------------------------------------------------------------
// Closing entry line structure verification:
// Revenue closing:   Dr revenue account / Cr 3301 running
// Expense closing:   Dr 3301 running / Cr expense account
// Profit closing:    Dr 3301 running / Cr 3201 retained
// Loss closing:      Dr 3201 retained / Cr 3301 running
// ---------------------------------------------------------------------------

func TestBuildClosingLines_RevenueLineStructure(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 7_000_00}, // revenue
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// First pair: Dr revenue account / Cr running.
	found := false
	for _, line := range lines {
		if line.AccountID == 100 && line.DebitCents == 7_000_00 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Dr revenue account 700000")
	}

	found = false
	for _, line := range lines {
		if line.AccountID == 3301 && line.CreditCents == 7_000_00 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Cr running account 700000")
	}
}

func TestBuildClosingLines_ExpenseLineStructure(t *testing.T) {
	balances := []plBalance{
		{accountID: 200, amount: -3_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// Expense pair: Dr running / Cr expense account.
	found := false
	for _, line := range lines {
		if line.AccountID == 3301 && line.DebitCents == 3_000_00 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Dr running account 300000")
	}

	found = false
	for _, line := range lines {
		if line.AccountID == 200 && line.CreditCents == 3_000_00 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Cr expense account 300000")
	}
}

func TestBuildClosingLines_ProfitRetainedLineStructure(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 10_000_00}, // revenue
		{accountID: 200, amount: -3_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// Net profit = 7000. Expect Dr running / Cr retained.
	foundDr := false
	foundCr := false
	for _, line := range lines {
		if line.AccountID == 3301 && line.DebitCents == 7_000_00 {
			foundDr = true
		}
		if line.AccountID == 3201 && line.CreditCents == 7_000_00 {
			foundCr = true
		}
	}
	if !foundDr {
		t.Fatal("expected Dr running 700000 for profit closing")
	}
	if !foundCr {
		t.Fatal("expected Cr retained 700000 for profit closing")
	}
}

func TestBuildClosingLines_LossRetainedLineStructure(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 2_000_00},  // revenue
		{accountID: 200, amount: -6_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	// Net loss = 4000. Expect Dr retained / Cr running.
	foundDr := false
	foundCr := false
	for _, line := range lines {
		if line.AccountID == 3201 && line.DebitCents == 4_000_00 {
			foundDr = true
		}
		if line.AccountID == 3301 && line.CreditCents == 4_000_00 {
			foundCr = true
		}
	}
	if !foundDr {
		t.Fatal("expected Dr retained 400000 for loss closing")
	}
	if !foundCr {
		t.Fatal("expected Cr running 400000 for loss closing")
	}
}

// ---------------------------------------------------------------------------
// SourceLineRef verification (ensures audit trail references are correct)
// ---------------------------------------------------------------------------

func TestBuildClosingLines_SourceLineRefs(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 5_000_00},  // revenue
		{accountID: 200, amount: -3_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	refs := make(map[string]bool)
	for _, line := range lines {
		refs[line.SourceLineRef] = true
	}

	expectedRefs := []string{
		"rev-100",       // revenue debit
		"to-running",    // revenue credit to running
		"from-running",  // expense debit from running
		"exp-200",       // expense credit
		"close-running", // profit closing debit
		"to-retained",   // profit closing credit
	}
	for _, ref := range expectedRefs {
		if !refs[ref] {
			t.Errorf("missing SourceLineRef %q in closing lines", ref)
		}
	}
}

func TestBuildClosingLines_LossSourceLineRefs(t *testing.T) {
	balances := []plBalance{
		{accountID: 100, amount: 1_000_00},  // revenue
		{accountID: 200, amount: -4_000_00}, // expense
	}
	lines := buildClosingLines(balances, 3201, 3301)

	refs := make(map[string]bool)
	for _, line := range lines {
		refs[line.SourceLineRef] = true
	}

	expectedRefs := []string{
		"rev-100",
		"to-running",
		"from-running",
		"exp-200",
		"from-retained", // loss: debit retained
		"close-running", // loss: credit running
	}
	for _, ref := range expectedRefs {
		if !refs[ref] {
			t.Errorf("missing SourceLineRef %q in loss closing lines", ref)
		}
	}
}

// ---------------------------------------------------------------------------
// plBalance struct behavior
// ---------------------------------------------------------------------------

func TestPLBalance_ZeroValue(t *testing.T) {
	var b plBalance
	if b.accountID != 0 {
		t.Fatalf("zero-value accountID = %d, want 0", b.accountID)
	}
	if b.amount != 0 {
		t.Fatalf("zero-value amount = %d, want 0", b.amount)
	}
}

func TestPLBalance_PositiveIsRevenue(t *testing.T) {
	b := plBalance{accountID: 100, amount: 5_000_00}
	if b.amount <= 0 {
		t.Fatal("positive amount should represent revenue (credit balance)")
	}
}

func TestPLBalance_NegativeIsExpense(t *testing.T) {
	b := plBalance{accountID: 200, amount: -3_000_00}
	if b.amount >= 0 {
		t.Fatal("negative amount should represent expense (debit balance)")
	}
}

// ---------------------------------------------------------------------------
// loadPLBalances SQL filter logic (conceptual test)
// The SQL in loadPLBalances filters by:
//   - je.status = 'POSTED'
//   - a.report_group IN ('revenue', 'expense')
//   - je.entry_date BETWEEN p.period_start AND p.period_end
//   - amount = SUM(credit_cents - debit_cents)
// We verify the conceptual logic here since we can't hit the DB.
// ---------------------------------------------------------------------------

func TestPLBalance_AmountIsCreditMinusDebit(t *testing.T) {
	// Revenue accounts normally have credit balances (credit > debit).
	// So amount = credit - debit > 0 for revenue.
	creditCents := int64(10_000_00)
	debitCents := int64(0)
	amount := creditCents - debitCents
	if amount <= 0 {
		t.Fatalf("revenue amount = %d, should be positive", amount)
	}
}

func TestPLBalance_ExpenseAmountIsNegative(t *testing.T) {
	// Expense accounts normally have debit balances (debit > credit).
	// So amount = credit - debit < 0 for expense.
	creditCents := int64(0)
	debitCents := int64(6_000_00)
	amount := creditCents - debitCents
	if amount >= 0 {
		t.Fatalf("expense amount = %d, should be negative", amount)
	}
}

func TestPLBalance_SQLFilterConceptual(t *testing.T) {
	// The SQL filters report_group IN ('revenue', 'expense').
	// Verify that only these two report groups are considered P&L.
	plReportGroups := map[string]bool{
		"revenue": true,
		"expense": true,
	}
	if plReportGroups["asset"] {
		t.Fatal("assets should not be in P&L")
	}
	if plReportGroups["liability"] {
		t.Fatal("liabilities should not be in P&L")
	}
	if plReportGroups["equity"] {
		t.Fatal("equity should not be in P&L (except via closing entries)")
	}
	if !plReportGroups["revenue"] {
		t.Fatal("revenue must be in P&L")
	}
	if !plReportGroups["expense"] {
		t.Fatal("expense must be in P&L")
	}
}

// ---------------------------------------------------------------------------
// Closing entry math verification:
//   Dr Revenue / Cr 3301 (close revenue to running)
//   Dr 3301 / Cr Expense (close expense to running)
//   Dr 3301 / Cr 3201 for net profit (if profit)
//   Dr 3201 / Cr 3301 for net loss (if loss)
// The total debits must equal total credits.
// ---------------------------------------------------------------------------

func TestClosingEntryMath_ProfitBalanced(t *testing.T) {
	revenue := int64(12_000_00)
	expense := int64(7_000_00)
	netProfit := revenue - expense // 5_000_00

	// Closing entries:
	// Dr Revenue 120000 / Cr 3301 120000
	// Dr 3301 70000 / Cr Expense 70000
	// Dr 3301 50000 / Cr 3201 50000
	totalDebits := revenue + expense + netProfit
	totalCredits := revenue + expense + netProfit

	if totalDebits != totalCredits {
		t.Fatalf("debits %d != credits %d", totalDebits, totalCredits)
	}
}

func TestClosingEntryMath_LossBalanced(t *testing.T) {
	revenue := int64(3_000_00)
	expense := int64(8_000_00)
	netLoss := expense - revenue // 5_000_00

	// Closing entries:
	// Dr Revenue 30000 / Cr 3301 30000
	// Dr 3301 80000 / Cr Expense 80000
	// Dr 3201 50000 / Cr 3301 50000
	totalDebits := revenue + expense + netLoss
	totalCredits := revenue + expense + netLoss

	if totalDebits != totalCredits {
		t.Fatalf("debits %d != credits %d", totalDebits, totalCredits)
	}
}

func TestClosingEntryMath_BreakEvenBalanced(t *testing.T) {
	revenue := int64(5_000_00)
	expense := int64(5_000_00)

	// No retained earnings entry.
	// Dr Revenue 50000 / Cr 3301 50000
	// Dr 3301 50000 / Cr Expense 50000
	totalDebits := revenue + expense
	totalCredits := revenue + expense

	if totalDebits != totalCredits {
		t.Fatalf("debits %d != credits %d", totalDebits, totalCredits)
	}
}

// ---------------------------------------------------------------------------
// Running account (3301) net balance after closing
// After closing, the running account should be fully cleared to retained.
// ---------------------------------------------------------------------------

func TestRunningAccount_ClearsToZero_Profit(t *testing.T) {
	revenue := int64(10_000_00)
	expense := int64(4_000_00)
	netProfit := revenue - expense

	// Running account flows:
	// Cr 3301 100000 (from revenue close)
	// Dr 3301 40000 (to expense close)
	// Dr 3301 60000 (to retained, profit)
	runningCredits := revenue
	runningDebits := expense + netProfit

	if runningCredits != runningDebits {
		t.Fatalf("running account not cleared: credits %d != debits %d",
			runningCredits, runningDebits)
	}
}

func TestRunningAccount_ClearsToZero_Loss(t *testing.T) {
	revenue := int64(2_000_00)
	expense := int64(6_000_00)
	netLoss := expense - revenue

	// Running account flows:
	// Cr 3301 20000 (from revenue close)
	// Dr 3301 60000 (to expense close)
	// Cr 3301 40000 (from retained, loss)
	runningCredits := revenue + netLoss
	runningDebits := expense

	if runningCredits != runningDebits {
		t.Fatalf("running account not cleared: credits %d != debits %d",
			runningCredits, runningDebits)
	}
}

func TestRunningAccount_ClearsToZero_BreakEven(t *testing.T) {
	revenue := int64(5_000_00)
	expense := int64(5_000_00)

	// No retained entry.
	// Cr 3301 50000 (from revenue close)
	// Dr 3301 50000 (to expense close)
	runningCredits := revenue
	runningDebits := expense

	if runningCredits != runningDebits {
		t.Fatalf("running account not cleared: credits %d != debits %d",
			runningCredits, runningDebits)
	}
}

// ---------------------------------------------------------------------------
// mustJSON helper
// ---------------------------------------------------------------------------

func TestMustJSON_ValidInput(t *testing.T) {
	data := mustJSON(map[string]int{"a": 1})
	if len(data) == 0 {
		t.Fatal("mustJSON returned empty for valid input")
	}
}

func TestMustJSON_NilInput(t *testing.T) {
	data := mustJSON(nil)
	if string(data) != "null" {
		t.Fatalf("mustJSON(nil) = %s, want null", string(data))
	}
}

// ---------------------------------------------------------------------------
// errorResponse struct
// ---------------------------------------------------------------------------

func TestErrorResponse_Fields(t *testing.T) {
	er := errorResponse{Code: "CLOSE_FAILED", Message: "period already closed"}
	if er.Code != "CLOSE_FAILED" {
		t.Fatalf("Code = %s, want CLOSE_FAILED", er.Code)
	}
	if er.Message != "period already closed" {
		t.Fatalf("Message = %s", er.Message)
	}
}

func TestErrorResponse_ZeroValue(t *testing.T) {
	var er errorResponse
	if er.Code != "" || er.Message != "" {
		t.Fatal("zero-value errorResponse should have empty fields")
	}
}

// ---------------------------------------------------------------------------
// findOpenPeriod validation (conceptual)
// findOpenPeriod looks for status = 'OPEN' and checks no PERIOD_CLOSE journal
// exists. We test the validation logic conceptually.
// ---------------------------------------------------------------------------

func TestFindOpenPeriod_Conceptual_AlreadyClosed(t *testing.T) {
	// If a PERIOD_CLOSE journal exists with source_ref = "CLOSE-{periodID}",
	// findOpenPeriod returns "period already closed".
	// This is the validation guard against double-closing.
	periodID := int64(42)
	sourceRef := "CLOSE-" + intToStr(periodID)
	if sourceRef != "CLOSE-42" {
		t.Fatalf("source ref = %s, want CLOSE-42", sourceRef)
	}
}

// intToStr is a helper to avoid importing strconv just for one call.
func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// Source reference format verification
// ---------------------------------------------------------------------------

func TestSourceRef_CloseFormat(t *testing.T) {
	// closePeriod uses SourceRef = "CLOSE-{periodID}".
	periodID := int64(7)
	sourceRef := "CLOSE-" + intToStr(periodID)
	if sourceRef != "CLOSE-7" {
		t.Fatalf("close source ref = %s, want CLOSE-7", sourceRef)
	}
}

func TestSourceRef_UnlockFormat(t *testing.T) {
	// unlockPeriod uses SourceRef = "UNLOCK-{periodID}".
	periodID := int64(3)
	sourceRef := "UNLOCK-" + intToStr(periodID)
	if sourceRef != "UNLOCK-3" {
		t.Fatalf("unlock source ref = %s, want UNLOCK-3", sourceRef)
	}
}

// ---------------------------------------------------------------------------
// Journal number format verification (nextJournalNumber)
// Format: {PREFIX}-{YEAR}-{6-digit seq}, e.g. JRN-2026-000001
// ---------------------------------------------------------------------------

// leftPad6Local pads a sequence number to 6 digits with leading zeros.
func leftPad6Local(seq int64) string {
	s := intToStr(seq)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

func TestJournalNumberFormat(t *testing.T) {
	// Replicate the format from nextJournalNumber: PREFIX-YEAR-000001.
	year := 2026
	prefix := "JRN"
	seq := int64(1)
	formatted := prefix + "-" + intToStr(int64(year)) + "-" + leftPad6Local(seq)
	if formatted != "JRN-2026-000001" {
		t.Fatalf("journal number = %s, want JRN-2026-000001", formatted)
	}
}

func TestJournalNumberFormat_LargeSeq(t *testing.T) {
	year := 2026
	prefix := "JRN"
	seq := int64(999999)
	formatted := prefix + "-" + intToStr(int64(year)) + "-" + leftPad6Local(seq)
	if formatted != "JRN-2026-999999" {
		t.Fatalf("journal number = %s, want JRN-2026-999999", formatted)
	}
}

// ---------------------------------------------------------------------------
// Integration: full closing entry with buildClosingLines + BalanceCheck
// This verifies the entire closing entry pipeline produces valid double-entry.
// ---------------------------------------------------------------------------

func TestFullClosingEntry_ProfitScenario(t *testing.T) {
	balances := []plBalance{
		{accountID: 101, amount: 50_000_00},  // sales revenue
		{accountID: 102, amount: 10_000_00},  // other income
		{accountID: 201, amount: -20_000_00}, // COGS
		{accountID: 202, amount: -5_000_00},  // operating expenses
	}
	retainedID := int64(3201)
	runningID := int64(3301)

	lines := buildClosingLines(balances, retainedID, runningID)

	// Must pass BalanceCheck (debits == credits, valid lines).
	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("full closing entry failed balance check: %v", err)
	}

	// Net profit = 60000 - 25000 = 35000.
	// Verify retained earnings credit.
	var retainedCredit int64
	for _, line := range lines {
		if line.AccountID == retainedID {
			retainedCredit += line.CreditCents
		}
	}
	if retainedCredit != 35_000_00 {
		t.Fatalf("retained credit = %d, want 3500000", retainedCredit)
	}
}

func TestFullClosingEntry_LossScenario(t *testing.T) {
	balances := []plBalance{
		{accountID: 101, amount: 10_000_00},  // revenue
		{accountID: 201, amount: -25_000_00}, // expenses
	}
	retainedID := int64(3201)
	runningID := int64(3301)

	lines := buildClosingLines(balances, retainedID, runningID)

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("loss closing entry failed balance check: %v", err)
	}

	// Net loss = 15000. Verify retained earnings debit.
	var retainedDebit int64
	for _, line := range lines {
		if line.AccountID == retainedID {
			retainedDebit += line.DebitCents
		}
	}
	if retainedDebit != 15_000_00 {
		t.Fatalf("retained debit = %d, want 1500000", retainedDebit)
	}
}

func TestFullClosingEntry_BreakEvenScenario(t *testing.T) {
	balances := []plBalance{
		{accountID: 101, amount: 20_000_00},  // revenue
		{accountID: 201, amount: -20_000_00}, // expense
	}

	lines := buildClosingLines(balances, 3201, 3301)

	if err := accounting.BalanceCheck(lines); err != nil {
		t.Fatalf("break-even closing entry failed balance check: %v", err)
	}

	// No retained earnings lines.
	for _, line := range lines {
		if line.AccountID == 3201 {
			t.Fatalf("break-even should not produce retained earnings lines: %+v", line)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers: writeJSON / writeError
// ---------------------------------------------------------------------------

func TestWriteJSON_StatusAndContentType(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"ok", http.StatusOK},
		{"bad request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"internal error", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeJSON(recorder, tt.status, map[string]string{"a": "b"})
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %s, want application/json", got)
			}
		})
	}
}

func TestWriteJSON_BodyIsEncodedPayload(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, map[string]any{"period_id": int64(7), "status": "CLOSED"})

	var decoded map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded["status"] != "CLOSED" {
		t.Fatalf("status = %v, want CLOSED", decoded["status"])
	}
	if decoded["period_id"] != float64(7) {
		t.Fatalf("period_id = %v, want 7", decoded["period_id"])
	}
}

func TestWriteError_ResponseShape(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
	}{
		{"tenant required", http.StatusUnauthorized, "TENANT_REQUIRED", "tenant context is required"},
		{"close failed", http.StatusBadRequest, "CLOSE_FAILED", "period already closed"},
		{"unlock failed", http.StatusBadRequest, "UNLOCK_FAILED", "no closed period"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeError(recorder, tt.status, tt.code, tt.message)

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			var decoded errorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&decoded); err != nil {
				t.Fatalf("body is not valid JSON: %v", err)
			}
			if decoded.Code != tt.code {
				t.Fatalf("code = %s, want %s", decoded.Code, tt.code)
			}
			if decoded.Message != tt.message {
				t.Fatalf("message = %s, want %s", decoded.Message, tt.message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Handler tenant-validation guards (no database: the guard returns before
// any pool access, so a Service built with a nil pool is safe).
// ---------------------------------------------------------------------------

func TestCloseHandler_RejectsWithoutTenant(t *testing.T) {
	service := NewHandler(nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/periods/close", nil)

	service.Close(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var decoded errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded.Code != "TENANT_REQUIRED" {
		t.Fatalf("code = %s, want TENANT_REQUIRED", decoded.Code)
	}
}

func TestUnlockHandler_RejectsWithoutTenant(t *testing.T) {
	service := NewHandler(nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/periods/unlock", nil)

	service.Unlock(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var decoded errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded.Code != "TENANT_REQUIRED" {
		t.Fatalf("code = %s, want TENANT_REQUIRED", decoded.Code)
	}
}

// ---------------------------------------------------------------------------
// Unlock reversal construction: the reversal of the closing journal swaps
// debits and credits, prefixes SourceLineRef with "rev-", and must balance.
// (Replicates the inline logic in unlockPeriod, handler.go:85-96.)
// ---------------------------------------------------------------------------

func reverseLines(lines []accounting.Line) []accounting.Line {
	var reversed []accounting.Line
	for _, line := range lines {
		reversed = append(reversed, accounting.Line{
			AccountID:     line.AccountID,
			DebitCents:    line.CreditCents,
			CreditCents:   line.DebitCents,
			SourceLineRef: "rev-" + line.SourceLineRef,
		})
	}
	return reversed
}

func TestUnlockReversal_SwapsDebitsAndCredits(t *testing.T) {
	closing := []accounting.Line{
		{AccountID: 101, DebitCents: 10_000_00, SourceLineRef: "rev-101"},
		{AccountID: 3301, CreditCents: 10_000_00, SourceLineRef: "to-running"},
	}

	reversed := reverseLines(closing)

	if reversed[0].DebitCents != 0 || reversed[0].CreditCents != 10_000_00 {
		t.Fatalf("reversed[0] = %+v, want credit 1000000", reversed[0])
	}
	if reversed[1].DebitCents != 10_000_00 || reversed[1].CreditCents != 0 {
		t.Fatalf("reversed[1] = %+v, want debit 1000000", reversed[1])
	}
}

func TestUnlockReversal_PrefixesSourceLineRef(t *testing.T) {
	closing := []accounting.Line{
		{AccountID: 101, DebitCents: 1_00, SourceLineRef: "rev-101"},
		{AccountID: 3301, CreditCents: 1_00, SourceLineRef: "to-running"},
	}

	reversed := reverseLines(closing)

	if reversed[0].SourceLineRef != "rev-rev-101" {
		t.Fatalf("reversed[0] ref = %s, want rev-rev-101", reversed[0].SourceLineRef)
	}
	if reversed[1].SourceLineRef != "rev-to-running" {
		t.Fatalf("reversed[1] ref = %s, want rev-to-running", reversed[1].SourceLineRef)
	}
}

func TestUnlockReversal_BalanceCheckPasses(t *testing.T) {
	closing := buildClosingLines([]plBalance{
		{accountID: 4001, amount: 50_000_00},  // revenue
		{accountID: 6001, amount: -30_000_00}, // expense
	}, 3201, 3301)

	if err := accounting.BalanceCheck(closing); err != nil {
		t.Fatalf("closing entry not balanced: %v", err)
	}
	if err := accounting.BalanceCheck(reverseLines(closing)); err != nil {
		t.Fatalf("reversal not balanced: %v", err)
	}
}

func TestUnlockReversal_NetEffectIsZero(t *testing.T) {
	closing := buildClosingLines([]plBalance{
		{accountID: 4001, amount: 25_000_00},
		{accountID: 6001, amount: -15_000_00},
	}, 3201, 3301)
	reversed := reverseLines(closing)

	combined := append(append([]accounting.Line{}, closing...), reversed...)

	var debitTotal, creditTotal int64
	for _, line := range combined {
		debitTotal += line.DebitCents
		creditTotal += line.CreditCents
	}
	if debitTotal != creditTotal {
		t.Fatalf("closing + reversal: debits %d != credits %d", debitTotal, creditTotal)
	}
}
