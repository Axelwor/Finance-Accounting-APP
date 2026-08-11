package httperr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrIdempotencyKeyReuse is returned when an idempotency key is reused with a
// different request payload (M-023). Handlers should map it to 409 CONFLICT.
var ErrIdempotencyKeyReuse = errors.New("IDEMPOTENCY_KEY_REUSE")

// Classify maps an error to an HTTP status code and stable error code following
// the API_CONTRACT.md convention:
//
//   - pgx.ErrNoRows            → 404 NOT_FOUND
//   - unique violation (23505) → 409 CONFLICT
//   - foreign key violation    → 400 INVALID_REQUEST
//   - idempotency key reuse    → 409 IDEMPOTENCY_KEY_REUSE
//   - everything else          → 500 INTERNAL_ERROR
//
// Handlers should check their own domain-specific sentinel errors first, then
// fall back to Classify for the generic DB/error classification.
func Classify(err error) (int, string) {
	if errors.Is(err, ErrIdempotencyKeyReuse) {
		return http.StatusConflict, "IDEMPOTENCY_KEY_REUSE"
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, "NOT_FOUND"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return http.StatusConflict, "CONFLICT"
		case "23503": // foreign_key_violation
			return http.StatusBadRequest, "INVALID_REQUEST"
		}
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR"
}

// ComputeRequestHash reads the request body and returns a SHA-256 hex digest.
// The body is restored so downstream handlers (decodeJSON, etc.) can still read
// it. Returns "" when the body is nil or cannot be read.
func ComputeRequestHash(request *http.Request) string {
	if request.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return ""
	}
	request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	sum := sha256.Sum256(bodyBytes)
	return hex.EncodeToString(sum[:])
}

// CheckIdempotencyHash compares the stored request_hash on an existing journal
// entry against the current request hash. If both are non-empty and differ, it
// returns ErrIdempotencyKeyReuse (the caller should abort the transaction and
// return 409). On match (or when either hash is empty for backward
// compatibility) it returns nil.
func CheckIdempotencyHash(storedHash, requestHash string) error {
	if storedHash != "" && requestHash != "" && storedHash != requestHash {
		return ErrIdempotencyKeyReuse
	}
	return nil
}
