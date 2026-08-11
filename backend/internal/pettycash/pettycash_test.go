package pettycash

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Tests for pure validation/math helpers in the petty cash (imprest) package.
// These functions contain no DB or http coupling, so they are unit-testable.
// ---------------------------------------------------------------------------

// --- validateFundRequest ---------------------------------------------------

func TestValidateFundRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  CreateFundRequest
		want bool
	}{
		{
			name: "all fields valid",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: 5, ImprestAmountCents: 500000},
			want: true,
		},
		{
			name: "empty code rejected",
			req:  CreateFundRequest{Code: "", Name: "Main", CashAccountID: 5, ImprestAmountCents: 500000},
			want: false,
		},
		{
			name: "whitespace-only code rejected (TrimSpace)",
			req:  CreateFundRequest{Code: "   ", Name: "Main", CashAccountID: 5, ImprestAmountCents: 500000},
			want: false,
		},
		{
			name: "empty name rejected",
			req:  CreateFundRequest{Code: "PC-001", Name: "", CashAccountID: 5, ImprestAmountCents: 500000},
			want: false,
		},
		{
			name: "whitespace-only name still passes (only empty check on name)",
			req:  CreateFundRequest{Code: "PC-001", Name: "   ", CashAccountID: 5, ImprestAmountCents: 500000},
			want: true, // mirrors inline behavior: name only checked for == "", not trimmed
		},
		{
			name: "zero cash account id rejected",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: 0, ImprestAmountCents: 500000},
			want: false,
		},
		{
			name: "negative cash account id rejected",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: -1, ImprestAmountCents: 500000},
			want: false,
		},
		{
			name: "zero imprest amount rejected",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: 5, ImprestAmountCents: 0},
			want: false,
		},
		{
			name: "negative imprest amount rejected",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: 5, ImprestAmountCents: -100},
			want: false,
		},
		{
			name: "one cent imprest is valid",
			req:  CreateFundRequest{Code: "PC-001", Name: "Main", CashAccountID: 5, ImprestAmountCents: 1},
			want: true,
		},
		{
			name: "code with surrounding spaces passes (only trimmed for emptiness)",
			req:  CreateFundRequest{Code: "  PC-001  ", Name: "Main", CashAccountID: 5, ImprestAmountCents: 1},
			want: true,
		},
		{
			name: "all fields missing",
			req:  CreateFundRequest{},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := validateFundRequest(tc.req)
			if got != tc.want {
				t.Errorf("validateFundRequest(%+v) = %v, want %v", tc.req, got, tc.want)
			}
		})
	}
}

// --- validateVoucherRequest -------------------------------------------------

func TestValidateVoucherRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  CreateVoucherRequest
		want bool
	}{
		{
			name: "all fields valid",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1500, ExpenseAccountID: 9,
				Description: "Taxi fare", Recipient: "John",
			},
			want: true,
		},
		{
			name: "empty recipient still valid (optional field)",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1500, ExpenseAccountID: 9,
				Description: "Taxi fare", Recipient: "",
			},
			want: true,
		},
		{
			name: "zero fund id rejected",
			req: CreateVoucherRequest{
				FundID: 0, AmountCents: 1500, ExpenseAccountID: 9, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "negative fund id rejected",
			req: CreateVoucherRequest{
				FundID: -3, AmountCents: 1500, ExpenseAccountID: 9, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "zero amount rejected",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 0, ExpenseAccountID: 9, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "negative amount rejected",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: -50, ExpenseAccountID: 9, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "zero expense account rejected",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1500, ExpenseAccountID: 0, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "negative expense account rejected",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1500, ExpenseAccountID: -1, Description: "Taxi fare",
			},
			want: false,
		},
		{
			name: "empty description rejected",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1500, ExpenseAccountID: 9, Description: "",
			},
			want: false,
		},
		{
			name: "one cent amount is valid",
			req: CreateVoucherRequest{
				FundID: 1, AmountCents: 1, ExpenseAccountID: 9, Description: "x",
			},
			want: true,
		},
		{
			name: "all fields missing",
			req:  CreateVoucherRequest{},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := validateVoucherRequest(tc.req)
			if got != tc.want {
				t.Errorf("validateVoucherRequest(%+v) = %v, want %v", tc.req, got, tc.want)
			}
		})
	}
}

// --- computeReplenishAmount -------------------------------------------------
// Imprest system math: replenishment restores the fund to its imprest amount,
// so the replenish amount equals the sum of posted vouchers (spent amount).

func TestComputeReplenishAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		imprest    int64
		spent      int64
		wantAmount int64
		wantReplen bool
	}{
		{name: "no vouchers spent returns zero and no replenish", imprest: 500000, spent: 0, wantAmount: 0, wantReplen: false},
		{name: "single voucher spent", imprest: 500000, spent: 1500, wantAmount: 1500, wantReplen: true},
		{name: "many vouchers summing to large amount", imprest: 500000, spent: 487650, wantAmount: 487650, wantReplen: true},
		{name: "fully spent fund still replenishes full imprest", imprest: 500000, spent: 500000, wantAmount: 500000, wantReplen: true},
		{name: "overspent beyond imprest replenishes the spent total", imprest: 500000, spent: 501000, wantAmount: 501000, wantReplen: true},
		{name: "negative spent is treated as no replenish (zero guard)", imprest: 500000, spent: -100, wantAmount: 0, wantReplen: false},
		{name: "one cent spent triggers replenish of one cent", imprest: 500000, spent: 1, wantAmount: 1, wantReplen: true},
		{name: "zero imprest with spent still replenishes spent amount", imprest: 0, spent: 750, wantAmount: 750, wantReplen: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotAmount, gotReplen := computeReplenishAmount(tc.imprest, tc.spent)
			if gotAmount != tc.wantAmount {
				t.Errorf("computeReplenishAmount(imprest=%d, spent=%d) amount = %d, want %d",
					tc.imprest, tc.spent, gotAmount, tc.wantAmount)
			}
			if gotReplen != tc.wantReplen {
				t.Errorf("computeReplenishAmount(imprest=%d, spent=%d) shouldReplenish = %v, want %v",
					tc.imprest, tc.spent, gotReplen, tc.wantReplen)
			}
		})
	}
}

// --- CreateFundRequest / CreateVoucherRequest struct sanity ------------------

func TestCreateFundRequest_ZeroValue(t *testing.T) {
	t.Parallel()
	// The zero-value CreateFundRequest must fail validation — this guards
	// against accidental default-valid behavior if the struct evolves.
	var zero CreateFundRequest
	if validateFundRequest(zero) {
		t.Error("zero-value CreateFundRequest should not be valid")
	}
}

func TestCreateVoucherRequest_ZeroValue(t *testing.T) {
	t.Parallel()
	var zero CreateVoucherRequest
	if validateVoucherRequest(zero) {
		t.Error("zero-value CreateVoucherRequest should not be valid")
	}
}
