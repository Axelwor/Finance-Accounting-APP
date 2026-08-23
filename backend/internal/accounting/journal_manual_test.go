package accounting

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/approval"
)

// TestClassifyPostingError covers CreateManualJournal's error mapping
// (N5/NEW-1 + QA-08): a cross-tenant account_id returns zero rows under RLS
// and must surface as 404 ACCOUNT_NOT_FOUND, and an entry date outside every
// OPEN period must surface as 422 ENTRY_DATE_OUTSIDE_OPEN_PERIOD.
func TestClassifyPostingError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "approval gate stays conflict",
			err:        approval.ErrApprovalRequired,
			wantStatus: http.StatusConflict,
			wantCode:   "APPROVAL_REQUIRED",
		},
		{
			name:       "entry date outside open period",
			err:        fmt.Errorf("entry date is outside an open period: %w", ErrEntryDateOutsideOpenPeriod),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ENTRY_DATE_OUTSIDE_OPEN_PERIOD",
		},
		{
			name:       "cross-tenant account lookup under RLS is not found",
			err:        fmt.Errorf("load line 0: %w", pgx.ErrNoRows),
			wantStatus: http.StatusNotFound,
			wantCode:   "ACCOUNT_NOT_FOUND",
		},
		{
			name:       "unbalanced lines stay bad request",
			err:        ErrNotBalanced,
			wantStatus: http.StatusBadRequest,
			wantCode:   "NOT_BALANCED",
		},
		{
			name:       "non postable account stays bad request",
			err:        ErrAccountNotPostable,
			wantStatus: http.StatusBadRequest,
			wantCode:   "ACCOUNT_NOT_POSTABLE",
		},
		{
			name:       "unknown failure stays internal",
			err:        errors.New("connection reset"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "POST_FAILED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, code, message := classifyPostingError(test.err)
			if status != test.wantStatus || code != test.wantCode {
				t.Fatalf("classifyPostingError() = (%d, %s), want (%d, %s); message=%q",
					status, code, test.wantStatus, test.wantCode, message)
			}
		})
	}
}

// TestErrEntryDateOutsideOpenPeriodSentinel pins the sentinel code so the
// HTTP error code and the sentinel text stay aligned.
func TestErrEntryDateOutsideOpenPeriodSentinel(t *testing.T) {
	if ErrEntryDateOutsideOpenPeriod.Error() != "ENTRY_DATE_OUTSIDE_OPEN_PERIOD" {
		t.Fatalf("sentinel = %q, want ENTRY_DATE_OUTSIDE_OPEN_PERIOD", ErrEntryDateOutsideOpenPeriod.Error())
	}
}
