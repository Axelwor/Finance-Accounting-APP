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
