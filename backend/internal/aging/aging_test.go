package aging

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests for pure functions in the aging package:
//   - classifyBucket
//   - buildAgingSummary
//   - parseAsOfDate
// ---------------------------------------------------------------------------

func TestClassifyBucket(t *testing.T) {
	tests := []struct {
		name        string
		daysOverdue int
		want        string
	}{
		{"negative far future", -45, "current"},
		{"zero exactly due", 0, "current"},
		{"one day overdue", 1, "1-30"},
		{"fifteen days overdue", 15, "1-30"},
		{"thirty days boundary", 30, "1-30"},
		{"thirty-one days boundary", 31, "31-60"},
		{"forty-five days overdue", 45, "31-60"},
		{"sixty days boundary", 60, "31-60"},
		{"sixty-one days boundary", 61, "61-90"},
		{"seventy-five days overdue", 75, "61-90"},
		{"ninety days boundary", 90, "61-90"},
		{"ninety-one days boundary", 91, "90+"},
		{"one hundred eighty days overdue", 180, "90+"},
		{"one year overdue", 365, "90+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyBucket(tt.daysOverdue)
			if got != tt.want {
				t.Errorf("classifyBucket(%d) = %q, want %q", tt.daysOverdue, got, tt.want)
			}
		})
	}
}

func TestBuildAgingSummary(t *testing.T) {
	asOf := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		rows           []agingRow
		wantTotal      int64
		wantCurrent    int64
		want130        int64
		want3160       int64
		want6190       int64
		want90Plus     int64
		wantRowCount   int
		wantRowsNonNil bool
	}{
		{
			name:             "nil rows becomes empty slice",
			rows:             nil,
			wantTotal:        0,
			wantCurrent:      0,
			want130:          0,
			want3160:         0,
			want6190:         0,
			want90Plus:       0,
			wantRowCount:     0,
			wantRowsNonNil:   true,
		},
		{
			name:           "empty rows",
			rows:           []agingRow{},
			wantTotal:      0,
			wantRowCount:   0,
			wantRowsNonNil: true,
		},
		{
			name: "single current row",
			rows: []agingRow{
				{PartyName: "Acme", OutstandingCents: 5000, Bucket: "current"},
			},
			wantTotal:    5000,
			wantCurrent:  5000,
			wantRowCount: 1,
		},
		{
			name: "one row per bucket",
			rows: []agingRow{
				{OutstandingCents: 1000, Bucket: "current"},
				{OutstandingCents: 2000, Bucket: "1-30"},
				{OutstandingCents: 3000, Bucket: "31-60"},
				{OutstandingCents: 4000, Bucket: "61-90"},
				{OutstandingCents: 5000, Bucket: "90+"},
			},
			wantTotal:    15000,
			wantCurrent:  1000,
			want130:      2000,
			want3160:     3000,
			want6190:     4000,
			want90Plus:   5000,
			wantRowCount: 5,
		},
		{
			name: "multiple rows in same bucket accumulate",
			rows: []agingRow{
				{OutstandingCents: 1000, Bucket: "1-30"},
				{OutstandingCents: 2500, Bucket: "1-30"},
				{OutstandingCents: 750, Bucket: "1-30"},
			},
			wantTotal:    4250,
			want130:      4250,
			wantRowCount: 3,
		},
		{
			name: "unknown bucket still counts toward total",
			rows: []agingRow{
				{OutstandingCents: 9999, Bucket: "unknown-bucket"},
			},
			wantTotal:    9999,
			wantRowCount: 1,
		},
		{
			name: "zero amount rows",
			rows: []agingRow{
				{OutstandingCents: 0, Bucket: "current"},
				{OutstandingCents: 0, Bucket: "90+"},
			},
			wantTotal:    0,
			wantRowCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := buildAgingSummary(tt.rows, asOf)

			if s.TotalCents != tt.wantTotal {
				t.Errorf("TotalCents = %d, want %d", s.TotalCents, tt.wantTotal)
			}
			if s.CurrentCents != tt.wantCurrent {
				t.Errorf("CurrentCents = %d, want %d", s.CurrentCents, tt.wantCurrent)
			}
			if s.Bucket130Cents != tt.want130 {
				t.Errorf("Bucket130Cents = %d, want %d", s.Bucket130Cents, tt.want130)
			}
			if s.Bucket3160Cents != tt.want3160 {
				t.Errorf("Bucket3160Cents = %d, want %d", s.Bucket3160Cents, tt.want3160)
			}
			if s.Bucket6190Cents != tt.want6190 {
				t.Errorf("Bucket6190Cents = %d, want %d", s.Bucket6190Cents, tt.want6190)
			}
			if s.Bucket90PlusCents != tt.want90Plus {
				t.Errorf("Bucket90PlusCents = %d, want %d", s.Bucket90PlusCents, tt.want90Plus)
			}
			if len(s.Rows) != tt.wantRowCount {
				t.Errorf("len(Rows) = %d, want %d", len(s.Rows), tt.wantRowCount)
			}
			if tt.wantRowsNonNil && s.Rows == nil {
				t.Error("Rows is nil, expected non-nil empty slice")
			}
			if s.AsOfDate != asOf.Format("2006-01-02") {
				t.Errorf("AsOfDate = %q, want %q", s.AsOfDate, asOf.Format("2006-01-02"))
			}
		})
	}
}

func TestBuildAgingSummary_PreservesRowData(t *testing.T) {
	asOf := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	input := []agingRow{
		{
			PartyID:          1,
			PartyName:        "Globex Corp",
			InvoiceNumber:    "INV-001",
			InvoiceDate:      "2026-01-01",
			DueDate:          "2026-02-01",
			OutstandingCents: 12500,
			Bucket:           "90+",
			DaysOverdue:      134,
		},
	}

	s := buildAgingSummary(input, asOf)

	if len(s.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(s.Rows))
	}
	r := s.Rows[0]
	if r.PartyID != input[0].PartyID {
		t.Errorf("PartyID = %d, want %d", r.PartyID, input[0].PartyID)
	}
	if r.PartyName != input[0].PartyName {
		t.Errorf("PartyName = %q, want %q", r.PartyName, input[0].PartyName)
	}
	if r.InvoiceNumber != input[0].InvoiceNumber {
		t.Errorf("InvoiceNumber = %q, want %q", r.InvoiceNumber, input[0].InvoiceNumber)
	}
	if r.OutstandingCents != input[0].OutstandingCents {
		t.Errorf("OutstandingCents = %d, want %d", r.OutstandingCents, input[0].OutstandingCents)
	}
	if r.Bucket != input[0].Bucket {
		t.Errorf("Bucket = %q, want %q", r.Bucket, input[0].Bucket)
	}
}

func TestParseAsOfDate(t *testing.T) {
	tests := []struct {
		name       string
		asOfParam  string
		wantExact  string // non-empty: assert exact date; empty: assert fallback to ~now
		wantLayout string
	}{
		{
			name:       "valid date",
			asOfParam:  "2026-03-15",
			wantExact:  "2026-03-15",
			wantLayout: "2006-01-02",
		},
		{
			name:       "empty param defaults to now",
			asOfParam:  "",
			wantExact:  "",
			wantLayout: "2006-01-02",
		},
		{
			name:       "invalid format defaults to now",
			asOfParam:  "15/03/2026",
			wantExact:  "",
			wantLayout: "2006-01-02",
		},
		{
			name:       "garbage defaults to now",
			asOfParam:  "not-a-date",
			wantExact:  "",
			wantLayout: "2006-01-02",
		},
		{
			name:       "ISO with time portion rejected defaults to now",
			asOfParam:  "2026-03-15T12:00:00Z",
			wantExact:  "",
			wantLayout: "2006-01-02",
		},
		{
			name:       "year only rejected defaults to now",
			asOfParam:  "2026",
			wantExact:  "",
			wantLayout: "2006-01-02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: "",
				},
			}
			if tt.asOfParam != "" {
				req.URL.RawQuery = "as_of=" + tt.asOfParam
			}

			got := parseAsOfDate(req)

			// Verify the result is a valid date in the expected layout.
			formatted := got.Format(tt.wantLayout)
			if _, err := time.Parse(tt.wantLayout, formatted); err != nil {
				t.Fatalf("parseAsOfDate returned unparseable date: %v", err)
			}

			if tt.wantExact != "" {
				if formatted != tt.wantExact {
					t.Errorf("parseAsOfDate(%q) = %q, want %q", tt.asOfParam, formatted, tt.wantExact)
				}
			} else {
				// Fallback: should be within 1 minute of now.
				now := time.Now()
				diff := got.Sub(now)
				if diff < -time.Minute || diff > time.Minute {
					t.Errorf("parseAsOfDate(%q) = %v, expected ~now (diff %v)", tt.asOfParam, got, diff)
				}
			}
		})
	}
}

func TestParseAsOfDate_OnlyUsesQueryParam(t *testing.T) {
	// Verify the function only reads the "as_of" query param, ignoring others.
	req := &http.Request{
		URL: &url.URL{
			RawQuery: "foo=bar&as_of=2025-12-25&baz=qux",
		},
	}
	got := parseAsOfDate(req)
	want := "2025-12-25"
	if got.Format("2006-01-02") != want {
		t.Errorf("parseAsOfDate = %q, want %q", got.Format("2006-01-02"), want)
	}
}
