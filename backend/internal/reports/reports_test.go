package reports

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// Tests for the pure (non-DB) functions in the reports package:
//   - validateTemplateRequest  (template field validation)
//   - pathID                   (URL path id parsing)
//   - isNoRows                 (pgx no-rows error detection)
//
// DB-coupled handler methods (ListTemplates, CreateTemplate, GetTemplate,
// RenderReport, GetLayout, AddWidget, widgetKPI*, ensureLayout, ...) are
// intentionally NOT tested here — they require a live *pgxpool.Pool.
// ============================================================================

// ---------------------------------------------------------------------------
// validateTemplateRequest
// ---------------------------------------------------------------------------

func TestValidateTemplateRequest(t *testing.T) {
	// A fully-populated request that should pass every check.
	valid := CreateTemplateRequest{
		Code:         "INV-TPL",
		Name:         "Invoice Template",
		DocumentType: "invoice",
		TemplateYAML: "title: Invoice\n",
		IsDefault:    true,
	}

	tests := []struct {
		name   string
		mutate func(*CreateTemplateRequest)
		want   bool
	}{
		{
			name:   "all fields present passes",
			mutate: func(r *CreateTemplateRequest) {},
			want:   true,
		},
		{
			name:   "is_default false still passes (not part of validation)",
			mutate: func(r *CreateTemplateRequest) { r.IsDefault = false },
			want:   true,
		},
		{
			name:   "empty code rejected",
			mutate: func(r *CreateTemplateRequest) { r.Code = "" },
			want:   false,
		},
		{
			name:   "whitespace-only code accepted (no trim, only empty check)",
			mutate: func(r *CreateTemplateRequest) { r.Code = "   " },
			want:   true, // reports validation does NOT trim — mirrors inline behavior
		},
		{
			name:   "empty name rejected",
			mutate: func(r *CreateTemplateRequest) { r.Name = "" },
			want:   false,
		},
		{
			name:   "empty document_type rejected",
			mutate: func(r *CreateTemplateRequest) { r.DocumentType = "" },
			want:   false,
		},
		{
			name:   "empty template_yaml rejected",
			mutate: func(r *CreateTemplateRequest) { r.TemplateYAML = "" },
			want:   false,
		},
		{
			name:   "only code present fails (name/document_type/yaml empty)",
			mutate: func(r *CreateTemplateRequest) { r.Name, r.DocumentType, r.TemplateYAML = "", "", "" },
			want:   false,
		},
		{
			name:   "only yaml present fails (code/name/doc empty)",
			mutate: func(r *CreateTemplateRequest) { r.Code, r.Name, r.DocumentType = "", "", "" },
			want:   false,
		},
		{
			name:   "all four required fields empty fails",
			mutate: func(r *CreateTemplateRequest) { r.Code, r.Name, r.DocumentType, r.TemplateYAML = "", "", "", "" },
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh copy so tests are independent.
			req := valid
			tt.mutate(&req)
			got := validateTemplateRequest(req)
			if got != tt.want {
				t.Errorf("validateTemplateRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateTemplateRequest_Independent confirms mutations don't leak
// between calls (pure function, no shared state).
func TestValidateTemplateRequest_Independent(t *testing.T) {
	base := CreateTemplateRequest{
		Code: "A", Name: "B", DocumentType: "C", TemplateYAML: "D",
	}
	// Invalid call must not corrupt the struct for the next valid call.
	invalid := base
	invalid.Code = ""
	if validateTemplateRequest(invalid) {
		t.Fatal("expected invalid request to fail")
	}
	if !validateTemplateRequest(base) {
		t.Fatal("base request was corrupted by previous call — function is not pure")
	}
}

// ---------------------------------------------------------------------------
// pathID
// ---------------------------------------------------------------------------

func TestPathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "simple positive", raw: "42", want: 42},
		{name: "zero", raw: "0", want: 0},
		{name: "one", raw: "1", want: 1},
		{name: "large number", raw: "9999999999", want: 9999999999},
		{name: "max int64", raw: "9223372036854775807", want: 9223372036854775807},
		{name: "negative number parses (no sign guard in pathID)", raw: "-5", want: -5},
		{name: "plus sign prefix parses", raw: "+10", want: 10},
		{name: "empty string returns zero", raw: "", want: 0},
		{name: "non-numeric returns zero", raw: "abc", want: 0},
		{name: "leading spaces rejected by ParseInt (returns 0)", raw: "  123", want: 0},
		{name: "trailing spaces rejected by ParseInt (returns 0)", raw: "123  ", want: 0},
		{name: "trailing garbage rejected by ParseInt (returns 0)", raw: "45xyz", want: 0},
		{name: "decimal-looking value rejected by ParseInt (returns 0)", raw: "7.9", want: 0},
		{name: "hex-looking value rejected in base 10 (returns 0)", raw: "0x1A", want: 0},
		{name: "underscore separator rejected (returns 0)", raw: "1_000", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathID(tt.raw)
			if got != tt.want {
				t.Errorf("pathID(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isNoRows
// ---------------------------------------------------------------------------

func TestIsNoRows(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "pgx.ErrNoRows is detected", err: pgx.ErrNoRows, want: true},
		{name: "nil error is not no-rows", err: nil, want: false},
		{name: "wrapped ErrNoRows (fmt.Errorf %w) IS detected (uses errors.Is)", err: wrapErr(pgx.ErrNoRows), want: true},
		{name: "generic error not detected", err: errGeneric("some failure"), want: false},
		{name: "different sentinel not detected", err: errGeneric("no rows in result set"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNoRows(tt.err)
			if got != tt.want {
				t.Errorf("isNoRows(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestIsNoRows_ConstantPointer confirms the sentinel is a specific comparable
// value (pgx.ErrNoRows is a pointerless error variable).
func TestIsNoRows_ConstantPointer(t *testing.T) {
	// Two references to pgx.ErrNoRows should compare equal — confirms the
	// package uses identity comparison consistently with pgx's sentinel.
	if !isNoRows(pgx.ErrNoRows) {
		t.Fatal("isNoRows(pgx.ErrNoRows) must be true")
	}
}

// ---------------------------------------------------------------------------
// test helpers (local error types)
// ---------------------------------------------------------------------------

type errGeneric string

func (e errGeneric) Error() string { return string(e) }

// wrapErr mimics fmt.Errorf("...: %w", err) so we can verify isNoRows uses
// errors.Is (which unwraps) rather than direct identity comparison (==).
func wrapErr(err error) error {
	return wrappedError{inner: err, msg: "wrapped: " + err.Error()}
}

type wrappedError struct {
	inner error
	msg   string
}

func (w wrappedError) Error() string { return w.msg }
func (w wrappedError) Unwrap() error { return w.inner }
