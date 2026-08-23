package cash

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/accounting"
)

// TestErrorForClassification is a table-driven check of the posting error
// classification (QA-07, QA-08, N6): business guards must map to 4xx codes,
// and the ErrNoRows fallback must stay last so the resolvePeriod sentinel
// wins over it.
func TestErrorForClassification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "double reverse of VOID journal",
			err:        fmt.Errorf("journal 5 is not posted: %w", errJournalNotPosted),
			wantStatus: http.StatusConflict,
			wantCode:   "JOURNAL_NOT_POSTED",
		},
		{
			name:       "entry date outside open period beats no-rows fallback",
			err:        fmt.Errorf("entry date is outside an open period: %w", accounting.ErrEntryDateOutsideOpenPeriod),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ENTRY_DATE_OUTSIDE_OPEN_PERIOD",
		},
		{
			name:       "entry date outside open period wraps pgx.ErrNoRows like resolvePeriod",
			err:        fmt.Errorf("entry date is outside an open period (%w)", errors.Join(pgx.ErrNoRows, accounting.ErrEntryDateOutsideOpenPeriod)),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ENTRY_DATE_OUTSIDE_OPEN_PERIOD",
		},
		{
			name:       "transfer with non cash account rejected by engine",
			err:        accounting.ErrAccountTypeMismatch,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ACCOUNT_TYPE_MISMATCH",
		},
		{
			name:       "same transfer account",
			err:        accounting.ErrSameTransferAccount,
			wantStatus: http.StatusBadRequest,
			wantCode:   "SAME_TRANSFER_ACCOUNT",
		},
		{
			name:       "plain missing account stays not found",
			err:        pgx.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCode:   "ACCOUNT_NOT_FOUND",
		},
		{
			name:       "idempotency key reuse stays conflict",
			err:        fmt.Errorf("wrapped: %w", errIdempotencyKeyReuse),
			wantStatus: http.StatusConflict,
			wantCode:   "IDEMPOTENCY_KEY_REUSE",
		},
		{
			name:       "unknown error stays internal failure",
			err:        errors.New("connection reset"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "JOURNAL_POST_FAILED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, code, message := errorFor(test.err)
			if status != test.wantStatus || code != test.wantCode {
				t.Fatalf("errorFor() = (%d, %s), want (%d, %s); message=%q",
					status, code, test.wantStatus, test.wantCode, message)
			}
			if code == "JOURNAL_POST_FAILED" && status >= 400 && status < 500 {
				t.Fatal("4xx responses must use business codes, not JOURNAL_POST_FAILED")
			}
		})
	}
}

// TestErrJournalNotPostedSentinel guards the sentinel text used by logs.
func TestErrJournalNotPostedSentinel(t *testing.T) {
	if errJournalNotPosted.Error() != "JOURNAL_NOT_POSTED" {
		t.Fatalf("sentinel = %q, want JOURNAL_NOT_POSTED", errJournalNotPosted.Error())
	}
	if !errors.Is(fmt.Errorf("journal 5 is not posted: %w", errJournalNotPosted), errJournalNotPosted) {
		t.Fatal("wrapped reversal guard must remain detectable via errors.Is")
	}
}
