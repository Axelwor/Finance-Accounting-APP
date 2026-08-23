package coa

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateAccountInput(t *testing.T) {
	tests := []struct {
		name      string
		request   createAccountRequest
		wantError string // error code; empty means valid
	}{
		{
			name: "valid detail account",
			request: createAccountRequest{
				Code:        "1101",
				Name:        "Cash",
				ReportGroup: "asset",
				AccountType: "CASH",
			},
		},
		{
			name: "valid group account",
			request: createAccountRequest{
				Code:        "1100",
				Name:        "Cash and Bank",
				ReportGroup: "asset",
				IsGroup:     true,
			},
		},
		{
			name: "valid with parent and validity window",
			request: createAccountRequest{
				Code:        "1102",
				Name:        "BCA Bank",
				ReportGroup: "asset",
				AccountType: "BANK",
				ParentID:    int64Ptr(1100),
				ValidFrom:   timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				ValidTo:     timePtr(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "missing code",
			request: createAccountRequest{
				Name:        "Cash",
				ReportGroup: "asset",
				AccountType: "CASH",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "missing name",
			request: createAccountRequest{
				Code:        "1101",
				ReportGroup: "asset",
				AccountType: "CASH",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "code with surrounding whitespace",
			request: createAccountRequest{
				Code:        " 1101",
				Name:        "Cash",
				ReportGroup: "asset",
				AccountType: "CASH",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "code too long",
			request: createAccountRequest{
				Code:        strings.Repeat("1", 65),
				Name:        "Cash",
				ReportGroup: "asset",
				AccountType: "CASH",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "unknown report group",
			request: createAccountRequest{
				Code:        "1101",
				Name:        "Cash",
				ReportGroup: "contra_asset",
				AccountType: "CASH",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "missing account type on detail",
			request: createAccountRequest{
				Code:        "1101",
				Name:        "Cash",
				ReportGroup: "asset",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "group account must not carry account type",
			request: createAccountRequest{
				Code:        "1100",
				Name:        "Cash and Bank",
				ReportGroup: "asset",
				AccountType: "CASH",
				IsGroup:     true,
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "valid_to before valid_from",
			request: createAccountRequest{
				Code:        "1101",
				Name:        "Cash",
				ReportGroup: "asset",
				AccountType: "CASH",
				ValidFrom:   timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
				ValidTo:     timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := validateAccountInput(test.request)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

// TestValidateAccountTypeEnum covers the QA-10 fix: unknown account_type
// values must be rejected with a message listing the accepted enum.
func TestValidateAccountTypeEnum(t *testing.T) {
	rejectionMessage := func(accountType string) string {
		request := createAccountRequest{
			Code:        "9998",
			Name:        "Rejection Probe",
			ReportGroup: "asset",
			AccountType: accountType,
		}
		_, message := validateAccountInput(request)
		return message
	}

	tests := []struct {
		name        string
		accountType string
		wantValid   bool
	}{
		{"seeded cash type", "CASH", true},
		{"seeded bank type", "BANK", true},
		{"seeded ar type", "AR", true},
		{"seeded ap type", "AP", true},
		{"migration provisioned type", "VAT_PAYABLE", true},
		{"cheque module type", "CHEQUES_IN_TRANSIT", true},
		{"lease type", "LEASE_LIABILITY", true},
		{"oci type", "OCI", true},
		{"unknown type rejected", "NOT_A_TYPE", false},
		{"lowercase rejected", "cash", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := createAccountRequest{
				Code:        "9999",
				Name:        "Validation Probe",
				ReportGroup: "asset",
				AccountType: test.accountType,
			}
			code, message := validateAccountInput(request)
			if test.wantValid && code != "" {
				t.Fatalf("expected %q to be accepted, got %s: %s", test.accountType, code, message)
			}
			if !test.wantValid && code == "" {
				t.Fatalf("expected %q to be rejected", test.accountType)
			}
			if !test.wantValid && !strings.Contains(message, "account_type must be one of") {
				t.Fatalf("rejection message must list valid values, got %q", message)
			}
		})
	}

	// The rejection message must enumerate at least one value from each
	// source (seed and migrations) so clients can self-correct.
	message := rejectionMessage("NOT_A_TYPE")
	for _, want := range []string{"CASH", "REVENUE", "VAT_PAYABLE"} {
		if !strings.Contains(message, want) {
			t.Fatalf("valid list missing %q: %s", want, message)
		}
	}
	if !strings.Contains(message, `"NOT_A_TYPE"`) {
		t.Fatalf("valid list should echo offending value, got %q", message)
	}
}

// TestValidAccountTypesAreSortedAndDistinct guards the enum list quality.
func TestValidAccountTypesAreSortedAndDistinct(t *testing.T) {
	values := validAccountTypeList()
	if len(values) < 30 {
		t.Fatalf("expected the full system enum (>=30 values), got %d", len(values))
	}
	seen := make(map[string]bool, len(values))
	for index, value := range values {
		if seen[value] {
			t.Fatalf("duplicate account type %q in list", value)
		}
		seen[value] = true
		if index > 0 && values[index-1] >= value {
			t.Fatalf("list not sorted at %d: %q >= %q", index, values[index-1], value)
		}
		if !validAccountTypes[value] {
			t.Fatalf("list contains value missing from set: %q", value)
		}
	}
}

func TestValidateCategoryInput(t *testing.T) {
	tests := []struct {
		name      string
		request   createCategoryRequest
		wantError string
	}{
		{
			name: "valid IN category",
			request: createCategoryRequest{
				Name:                   "Cash Sales",
				Direction:              "IN",
				DefaultDebitAccountID:  1101,
				DefaultCreditAccountID: 4101,
			},
		},
		{
			name: "valid OUT category with single account",
			request: createCategoryRequest{
				Name:                  "Rent",
				Direction:             "OUT",
				DefaultDebitAccountID: 5101,
			},
		},
		{
			name: "missing name",
			request: createCategoryRequest{
				Direction:              "IN",
				DefaultDebitAccountID:  1101,
				DefaultCreditAccountID: 4101,
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "invalid direction",
			request: createCategoryRequest{
				Name:                   "Transfer",
				Direction:              "BOTH",
				DefaultDebitAccountID:  1101,
				DefaultCreditAccountID: 1102,
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "no default accounts",
			request: createCategoryRequest{
				Name:      "Empty",
				Direction: "IN",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "negative account id",
			request: createCategoryRequest{
				Name:                   "Negative",
				Direction:              "IN",
				DefaultDebitAccountID:  -1,
				DefaultCreditAccountID: 4101,
			},
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := validateCategoryInput(test.request)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

func TestValidateReportMappingInput(t *testing.T) {
	tests := []struct {
		name      string
		request   createReportMappingRequest
		wantError string
	}{
		{
			name: "valid mapping",
			request: createReportMappingRequest{
				AccountID:  1101,
				ReportType: "balance_sheet",
				ReportLine: "Cash and Cash Equivalents",
				Priority:   100,
			},
		},
		{
			name: "missing account",
			request: createReportMappingRequest{
				ReportType: "balance_sheet",
				ReportLine: "Cash and Cash Equivalents",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "missing report line",
			request: createReportMappingRequest{
				AccountID:  1101,
				ReportType: "balance_sheet",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "invalid report type",
			request: createReportMappingRequest{
				AccountID:  1101,
				ReportType: "tax_report",
				ReportLine: "Cash and Cash Equivalents",
			},
			wantError: "INVALID_REQUEST",
		},
		{
			name: "negative priority",
			request: createReportMappingRequest{
				AccountID:  1101,
				ReportType: "balance_sheet",
				ReportLine: "Cash and Cash Equivalents",
				Priority:   -1,
			},
			wantError: "INVALID_REQUEST",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := validateReportMappingInput(test.request)
			if test.wantError == "" && code != "" {
				t.Fatalf("expected valid request, got error code %q", code)
			}
			if test.wantError != "" && code != test.wantError {
				t.Fatalf("expected error code %q, got %q", test.wantError, code)
			}
		})
	}
}

func TestNormalizeAndValidAccountCode(t *testing.T) {
	if !validAccountCode("1101") {
		t.Fatal("expected 1101 to be valid")
	}
	if validAccountCode("") {
		t.Fatal("expected empty code to be invalid")
	}
	if validAccountCode(" 1101") {
		t.Fatal("expected leading whitespace to be invalid")
	}
	if got := normalizeCode("  1101  "); got != "1101" {
		t.Fatalf("expected normalized code 1101, got %q", got)
	}
}

func TestUniqueViolationDetection(t *testing.T) {
	var err error
	if isUniqueViolation(err) {
		t.Fatal("expected nil error to not be a unique violation")
	}
	if isUniqueViolation(errors.New("boom")) {
		t.Fatal("expected plain error to not be a unique violation")
	}
}

func int64Ptr(value int64) *int64 { return &value }

func timePtr(value time.Time) *time.Time { return &value }
