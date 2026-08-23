package purchase

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// QA-04: only code+name are required; every optional column must be accepted
// as absent, empty string, or zero without becoming mandatory.
func TestMinimalSupplierRequestIsValid(t *testing.T) {
	req := CreateSupplierRequest{Code: "SUP-001", Name: "Supplier Satu"}
	if code, msg := validateSupplierRequest(req); code != "" {
		t.Errorf("minimal supplier rejected: code=%q msg=%q", code, msg)
	}
	req.NPWP, req.ContactPerson, req.Phone, req.Email = "", "", "", ""
	req.Address, req.City, req.Province, req.PostalCode = "", "", "", ""
	req.PaymentTermID, req.CreditLimitCents = 0, 0
	req.BankName, req.BankAccountNumber, req.BankAccountName = "", "", ""
	req.Website, req.Fax, req.ContactPerson2, req.Phone2 = "", "", "", ""
	req.OpeningBalanceCents, req.OpeningBalanceDate = 0, ""
	if code, msg := validateSupplierRequest(req); code != "" {
		t.Errorf("all-empty optional fields rejected: code=%q msg=%q", code, msg)
	}
}

// QA-04 companion rule: supplier_type is optional but must be one of
// GOODS/SERVICE/MIXED (case-insensitive) when provided.
func TestValidateSupplierTypeRule(t *testing.T) {
	tests := []struct {
		name         string
		supplierType string
		wantError    bool
	}{
		{name: "empty allowed", supplierType: ""},
		{name: "GOODS", supplierType: "GOODS"},
		{name: "lowercase goods", supplierType: "goods"},
		{name: "mixed case service", supplierType: "Service"},
		{name: "MIXED", supplierType: "MIXED"},
		{name: "invalid company", supplierType: "company", wantError: true},
		{name: "invalid retail", supplierType: "retail", wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateSupplierRequest{Code: "SUP-001", Name: "PT Uji Type", SupplierType: tc.supplierType}
			code, _ := validateSupplierRequest(req)
			if (code != "") != tc.wantError {
				t.Errorf("validateSupplierRequest(%q) code=%q, wantError=%v", tc.supplierType, code, tc.wantError)
			}
		})
	}
}

func TestNormalizeSupplierType(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "goods", want: "GOODS"},
		{raw: "GOODS", want: "GOODS"},
		{raw: " Service ", want: "SERVICE"},
		{raw: "MiXeD", want: "MIXED"},
		{raw: "", want: ""},
		{raw: "   ", want: ""},
	}
	for _, tc := range tests {
		if got := normalizeSupplierType(tc.raw); got != tc.want {
			t.Errorf("normalizeSupplierType(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// QA-04: NULL optional columns scan into supplierRow via pgtype and map to
// zero values in the response instead of failing the scan.
func TestSupplierRowResponseMapsNullOptionals(t *testing.T) {
	row := supplierRow{
		ID:       7,
		Code:     "SUP-MIN",
		Name:     "Minimal Supplier",
		IsActive: true,
	}
	got := row.response()
	if got.ID != 7 || got.Code != "SUP-MIN" || got.Name != "Minimal Supplier" || !got.IsActive {
		t.Fatalf("response core fields mismatch: %+v", got)
	}
	for name, value := range map[string]string{
		"NPWP": got.NPWP, "ContactPerson": got.ContactPerson, "Phone": got.Phone,
		"Email": got.Email, "Address": got.Address, "City": got.City,
		"Province": got.Province, "PostalCode": got.PostalCode, "SupplierType": got.SupplierType,
		"CurrencyCode": got.CurrencyCode, "BankName": got.BankName,
		"BankAccountNumber": got.BankAccountNumber, "BankAccountName": got.BankAccountName,
		"Website": got.Website, "Fax": got.Fax, "ContactPerson2": got.ContactPerson2,
		"Phone2": got.Phone2, "OpeningBalanceDate": got.OpeningBalanceDate,
	} {
		if value != "" {
			t.Errorf("%s = %q, want empty for NULL column", name, value)
		}
	}
	if got.PaymentTermID != 0 || got.CreditLimitCents != 0 || got.OpeningBalanceCents != 0 {
		t.Errorf("int64 optional fields = %d/%d/%d, want 0 for NULL columns",
			got.PaymentTermID, got.CreditLimitCents, got.OpeningBalanceCents)
	}
}

func TestSupplierRowResponseMapsFilledOptionals(t *testing.T) {
	row := supplierRow{
		ID:               1,
		Code:             "SUP-FULL",
		Name:             "Supplier Lengkap",
		NPWP:             pgtype.Text{String: "01.234.567.8-901.000", Valid: true},
		PaymentTermID:    pgtype.Int8{Int64: 3, Valid: true},
		CreditLimitCents: pgtype.Int8{Int64: 5000000, Valid: true},
		SupplierType:     pgtype.Text{String: "GOODS", Valid: true},
		CurrencyCode:     pgtype.Text{String: "IDR", Valid: true},
		OpeningBalanceDate: pgtype.Date{
			Time: dateMust("2026-01-31"), Valid: true,
		},
	}
	got := row.response()
	if got.NPWP != "01.234.567.8-901.000" {
		t.Errorf("NPWP = %q", got.NPWP)
	}
	if got.PaymentTermID != 3 || got.CreditLimitCents != 5000000 {
		t.Errorf("PaymentTermID/CreditLimitCents = %d/%d", got.PaymentTermID, got.CreditLimitCents)
	}
	if got.SupplierType != "GOODS" || got.CurrencyCode != "IDR" {
		t.Errorf("SupplierType/CurrencyCode = %q/%q", got.SupplierType, got.CurrencyCode)
	}
	if got.OpeningBalanceDate != "2026-01-31" {
		t.Errorf("OpeningBalanceDate = %q", got.OpeningBalanceDate)
	}
}

func TestSupplierScanDestMatchesColumnOrder(t *testing.T) {
	var row supplierRow
	dest := supplierScanDest(&row)
	const columnCount = 26
	if len(dest) != columnCount {
		t.Fatalf("supplierScanDest len = %d, want %d", len(dest), columnCount)
	}
	if got := strings.Count(supplierColumns, ",") + 1; got != columnCount {
		t.Fatalf("supplierColumns has %d columns, want %d", got, columnCount)
	}
}

func dateMust(raw string) time.Time {
	parsed, err := parseDate(raw)
	if err != nil {
		panic(err)
	}
	return parsed.Time
}
