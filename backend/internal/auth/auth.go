package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	tenantIDKey contextKey = "tenant_id"
	userIDKey   contextKey = "user_id"
	roleKey     contextKey = "role"
)

// Role constants per ARCHITECTURE.md §8.2 RBAC matrix.
const (
	RoleOwner      = "owner"
	RoleAdmin      = "admin"
	RoleAccountant = "accountant"
	RoleManager    = "manager"
	RoleStaff      = "staff"
	RoleViewer     = "viewer"
)

type Service struct {
	pool      *pgxpool.Pool
	jwtSecret []byte
}

func NewService(pool *pgxpool.Pool, secret string) *Service {
	return &Service{pool: pool, jwtSecret: []byte(secret)}
}

type Claims struct {
	UserID   int64  `json:"user_id"`
	TenantID int64  `json:"tenant_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	// TenantName optionally creates a tenant during registration and makes the
	// new user its owner. When omitted the user is created without a tenant
	// and the issued token carries tenant_id 0.
	TenantName string `json:"tenant_name"`
}

func (service *Service) Register(writer http.ResponseWriter, request *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Email == "" || req.FullName == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "email and full_name are required")
		return
	}
	if err := validatePassword(req.Password); err != nil {
		writeError(writer, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not hash password")
		return
	}

	ctx := request.Context()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	var userID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		req.Email, string(hash), req.FullName,
	).Scan(&userID)
	if err != nil {
		writeError(writer, http.StatusConflict, "EMAIL_EXISTS", "email is already registered")
		return
	}

	tenantID := int64(0)
	if tenantName := strings.TrimSpace(req.TenantName); tenantName != "" {
		tenantID, err = insertTenant(ctx, tx, tenantName)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not create tenant")
			return
		}
		if _, err = tx.Exec(ctx,
			`INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'owner')`,
			userID, tenantID); err != nil {
			writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not assign tenant ownership")
			return
		}
		if err = seedDefaultCOA(ctx, tx, tenantID); err != nil {
			writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not provision chart of accounts")
			return
		}
	}

	if err = tx.Commit(ctx); err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not commit registration")
		return
	}

	// F-15: issue the session as RoleOwner — the membership row created above
	// says 'owner', and every RequireRole group in cmd/api pairs owner with
	// admin, so the claim must match the row.
	accessToken, err := service.issueToken(userID, tenantID, RoleOwner, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	// Issue a refresh token alongside the access token so the new session can
	// survive access-token expiry (and so /auth/switch-tenant, which requires
	// a refresh token, works right after onboarding creates the tenant).
	refreshToken, familyID, err := service.issueRefreshToken(request.Context(), userID, tenantID, RoleOwner, request.RemoteAddr, request.UserAgent())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue refresh token")
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{
		"id":            userID,
		"email":         req.Email,
		"tenant_id":     tenantID,
		"role":          RoleOwner,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"family_id":     familyID,
	})
}

// insertTenant inserts a tenant row inside tx, deriving a unique lowercase
// slug from name. If the derived slug already exists (unique_violation on
// tenants.slug), a random suffix is appended and the insert is retried.
func insertTenant(ctx context.Context, tx pgx.Tx, name string) (int64, error) {
	base := slugify(name)
	slug := base
	for attempt := 0; attempt < 5; attempt++ {
		var tenantID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
			name, slug).Scan(&tenantID)
		if err == nil {
			return tenantID, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			slug = base + "-" + randomSuffix()
			continue
		}
		return 0, err
	}
	return 0, fmt.Errorf("could not allocate a unique slug for tenant %q", name)
}

// slugify converts a tenant display name into a URL-safe lowercase slug.
// ASCII letters and digits are kept; everything else becomes a hyphen and
// runs of hyphens collapse. Falls back to "tenant" when nothing remains.
func slugify(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "tenant"
	}
	return slug
}

func randomSuffix() string {
	raw := make([]byte, 2)
	if _, err := rand.Read(raw); err != nil {
		return "0000"
	}
	return hex.EncodeToString(raw)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// TOTPC is the 6-digit authenticator code, required only when the user
	// has 2FA enabled (m-006).
	TOTPC string `json:"totp_code"`
}

func (service *Service) Login(writer http.ResponseWriter, request *http.Request) {
	var req LoginRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var userID int64
	var tenantID int64
	var passwordHash string
	var role string
	var totpEnabled bool
	var totpSecret pgtype.Text
	err := service.pool.QueryRow(request.Context(),
		`SELECT u.id, u.password_hash, COALESCE(ut.tenant_id, 0), COALESCE(ut.role, 'viewer'),
		        u.totp_enabled, u.totp_secret
		   FROM users u
		   LEFT JOIN user_tenants ut ON ut.user_id = u.id
		  WHERE u.email = $1 AND u.is_active = true
		  ORDER BY ut.id
		  LIMIT 1`,
		req.Email,
	).Scan(&userID, &passwordHash, &tenantID, &role, &totpEnabled, &totpSecret)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeError(writer, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
		return
	}
	// m-006: when 2FA is enabled the login must carry a valid TOTP code.
	if totpEnabled {
		if req.TOTPC == "" {
			writeError(writer, http.StatusUnauthorized, "TOTP_REQUIRED", "a 6-digit authenticator code is required")
			return
		}
		if !totpSecret.Valid || !ValidateTOTP(totpSecret.String, strings.TrimSpace(req.TOTPC)) {
			writeError(writer, http.StatusUnauthorized, "INVALID_TOTP", "authenticator code is incorrect")
			return
		}
	}
	accessToken, err := service.issueToken(userID, tenantID, role, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	refreshToken, familyID, err := service.issueRefreshToken(request.Context(), userID, tenantID, role, request.RemoteAddr, request.UserAgent())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue refresh token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"family_id":     familyID,
		"role":          role,
	})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (service *Service) Refresh(writer http.ResponseWriter, request *http.Request) {
	var req RefreshRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	hash := hashToken(req.RefreshToken)
	var userID int64
	var tenantID int64
	var tokenID int64
	var familyID string
	var expiresAt time.Time
	var role string
	var revokedAt pgtype.Timestamptz
	// The tenant + role are taken from the token row itself (set at login or
	// tenant switch) so a refreshed session stays on the ACTIVE tenant even
	// when the user has several tenants. Legacy rows without tenant_id fall
	// back to the user's first membership. Revoked rows are deliberately
	// INCLUDED in this lookup: presenting one is replay evidence (F-06) and
	// handled below.
	err := service.pool.QueryRow(request.Context(), `
		SELECT t.id, t.user_id, t.family_id, t.expires_at, t.revoked_at,
		       COALESCE(t.tenant_id, (
		           SELECT ut.tenant_id FROM user_tenants ut
		            WHERE ut.user_id = t.user_id
		            ORDER BY ut.id
		            LIMIT 1
		       ), 0),
		       COALESCE(t.role, (
		           SELECT ut.role FROM user_tenants ut
		            WHERE ut.user_id = t.user_id
		            ORDER BY ut.id
		            LIMIT 1
		       ), 'viewer')
		  FROM user_tokens t
		 WHERE t.token_hash = $1 AND t.token_type = 'refresh'
	`, hash).Scan(&tokenID, &userID, &familyID, &expiresAt, &revokedAt, &tenantID, &role)
	if err != nil || time.Now().After(expiresAt) {
		writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
		return
	}
	// F-06: presenting an already-revoked token means either a duplicated
	// in-flight request or a stolen token chain. Kill the entire family so
	// every descendant becomes unusable, then answer with the same generic
	// 401 as any other bad token — never disclose the reason.
	if revokedAt.Valid {
		_ = service.revokeTokenFamily(request.Context(), familyID)
		writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
		return
	}
	// Rotate: revoke the old token and store a new one in the same family.
	newRefresh, newFamily, err := service.rotateRefreshToken(request.Context(), tokenID, userID, tenantID, role, familyID, request.RemoteAddr, request.UserAgent())
	if err != nil {
		// F-06: a replay detected under lock (concurrent rotations) revokes
		// the whole family inside rotateRefreshToken; answer generically.
		if errors.Is(err, ErrRefreshReuse) {
			writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
			return
		}
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not rotate refresh token")
		return
	}
	// Preserve the tenant claim: re-issue the access token with the user's
	// default tenant (resolved above) instead of tenant_id 0.
	accessToken, err := service.issueToken(userID, tenantID, role, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": newRefresh,
		"family_id":     newFamily,
		"role":          role,
	})
}

// Logout revokes the presented refresh token.
func (service *Service) Logout(writer http.ResponseWriter, request *http.Request) {
	var req RefreshRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if _, err := service.pool.Exec(request.Context(),
		`UPDATE user_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashToken(req.RefreshToken)); err != nil {
		writeError(writer, http.StatusInternalServerError, "LOGOUT_FAILED", "could not revoke token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

// SwitchTenantRequest asks for a new token pair bound to another tenant the
// user belongs to.
type SwitchTenantRequest struct {
	TenantID     int64  `json:"tenant_id"`
	RefreshToken string `json:"refresh_token"`
}

// SwitchTenant issues a fresh access + refresh token pair for a different
// tenant of the authenticated user. It is the multi-tenant equivalent of
// login: the caller must present a valid (non-revoked) refresh token, which
// is rotated, and the user's membership in the requested tenant is verified
// against user_tenants (so a user can never switch into a tenant they do
// not belong to).
func (service *Service) SwitchTenant(writer http.ResponseWriter, request *http.Request) {
	var req SwitchTenantRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.TenantID <= 0 {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "tenant_id is required")
		return
	}
	if req.RefreshToken == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "refresh_token is required")
		return
	}

	hash := hashToken(req.RefreshToken)
	var tokenID, userID int64
	var familyID string
	var expiresAt time.Time
	var revokedAt pgtype.Timestamptz
	// Revoked rows are included so a replayed switch request is detected and
	// kills the family, mirroring the Refresh handler (F-06).
	err := service.pool.QueryRow(request.Context(), `
		SELECT id, user_id, family_id, expires_at, revoked_at
		FROM user_tokens
		WHERE token_hash = $1 AND token_type = 'refresh'
	`, hash).Scan(&tokenID, &userID, &familyID, &expiresAt, &revokedAt)
	if err != nil || time.Now().After(expiresAt) {
		writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
		return
	}
	if revokedAt.Valid {
		_ = service.revokeTokenFamily(request.Context(), familyID)
		writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
		return
	}

	// Verify membership and resolve the role in the requested tenant.
	var role string
	err = service.pool.QueryRow(request.Context(),
		`SELECT role FROM user_tenants WHERE user_id = $1 AND tenant_id = $2`, userID, req.TenantID,
	).Scan(&role)
	if err != nil {
		writeError(writer, http.StatusForbidden, "NOT_A_MEMBER", "you are not a member of that tenant")
		return
	}

	// Rotate the refresh token so the old one cannot be reused, storing the
	// newly-active tenant on the new row.
	newRefresh, newFamily, err := service.rotateRefreshToken(request.Context(), tokenID, userID, req.TenantID, role, familyID, request.RemoteAddr, request.UserAgent())
	if err != nil {
		if errors.Is(err, ErrRefreshReuse) {
			writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
			return
		}
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not rotate refresh token")
		return
	}
	accessToken, err := service.issueToken(userID, req.TenantID, role, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": newRefresh,
		"family_id":     newFamily,
		"tenant_id":     req.TenantID,
		"role":          role,
	})
}

// ---------------------------------------------------------------------------
// m-006: Two-factor authentication (TOTP) endpoints.
// ---------------------------------------------------------------------------

// Setup2FARequest carries an optional TOTP code. When 2FA is already
// enabled, a valid current code is required to authorize changing the secret.
type Setup2FARequest struct {
	TOTPCode string `json:"totp_code"`
}

// Setup2FA generates a TOTP secret for the authenticated user and returns the
// secret + provisioning URI. The secret is stored unverified; 2FA only
// activates after Setup2FAVerify succeeds. If 2FA is already enabled, the
// caller must supply a valid current TOTP code to authorize changing the
// secret — this prevents a stolen JWT from silently replacing the secret
// and downgrading 2FA.
func (service *Service) Setup2FA(writer http.ResponseWriter, request *http.Request) {
	userID, ok := UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	// Best-effort decode: the body may be empty for first-time setup.
	var req Setup2FARequest
	_ = decodeJSON(request, &req)

	var email string
	var totpEnabled bool
	var currentSecret pgtype.Text
	if err := service.pool.QueryRow(request.Context(),
		`SELECT email, totp_enabled, totp_secret FROM users WHERE id = $1`, userID,
	).Scan(&email, &totpEnabled, &currentSecret); err != nil {
		writeError(writer, http.StatusInternalServerError, "USER_NOT_FOUND", err.Error())
		return
	}
	// If 2FA is already enabled, require a valid current TOTP code to
	// authorize changing the secret.
	if totpEnabled {
		if strings.TrimSpace(req.TOTPCode) == "" {
			writeError(writer, http.StatusUnauthorized, "TOTP_REQUIRED", "a current authenticator code is required to re-setup 2FA")
			return
		}
		if !currentSecret.Valid || !ValidateTOTP(currentSecret.String, strings.TrimSpace(req.TOTPCode)) {
			writeError(writer, http.StatusUnauthorized, "INVALID_TOTP", "authenticator code is incorrect")
			return
		}
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOTP_SETUP_FAILED", err.Error())
		return
	}
	// Store the new secret in totp_pending_secret; keep totp_secret and
	// totp_enabled at their current values. The old secret stays valid
	// until Setup2FAVerify copies the pending secret to totp_secret, so a
	// lost setup response never locks the user out.
	if _, err := service.pool.Exec(request.Context(),
		`UPDATE users SET totp_pending_secret = $2 WHERE id = $1`,
		userID, secret); err != nil {
		writeError(writer, http.StatusInternalServerError, "TOTP_SETUP_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"secret": secret,
		"uri":    TOTPUri(secret, email, "FinanceAccounting"),
	})
}

type Verify2FARequest struct {
	Code string `json:"code"`
}

// Setup2FAVerify activates 2FA once the user proves possession of the secret
// by submitting a valid current TOTP code. It reads the pending secret (set
// by Setup2FA), verifies the code against it, and then atomically copies it
// to totp_secret, sets totp_enabled = true, and clears the pending value.
func (service *Service) Setup2FAVerify(writer http.ResponseWriter, request *http.Request) {
	userID, ok := UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	var req Verify2FARequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var pendingSecret pgtype.Text
	if err := service.pool.QueryRow(request.Context(),
		`SELECT totp_pending_secret FROM users WHERE id = $1`, userID,
	).Scan(&pendingSecret); err != nil {
		writeError(writer, http.StatusInternalServerError, "USER_NOT_FOUND", err.Error())
		return
	}
	if !pendingSecret.Valid {
		writeError(writer, http.StatusBadRequest, "TOTP_NOT_SETUP", "run 2FA setup first")
		return
	}
	if !ValidateTOTP(pendingSecret.String, strings.TrimSpace(req.Code)) {
		writeError(writer, http.StatusUnauthorized, "INVALID_TOTP", "authenticator code is incorrect")
		return
	}
	// Promote the pending secret to the active secret and enable 2FA.
	if _, err := service.pool.Exec(request.Context(),
		`UPDATE users SET totp_secret = $2, totp_enabled = true, totp_pending_secret = NULL WHERE id = $1`,
		userID, pendingSecret.String); err != nil {
		writeError(writer, http.StatusInternalServerError, "TOTP_ENABLE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"enabled": true})
}

// Disable2FA turns off 2FA for the authenticated user after verifying a code.
func (service *Service) Disable2FA(writer http.ResponseWriter, request *http.Request) {
	userID, ok := UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	var req Verify2FARequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var secret pgtype.Text
	if err := service.pool.QueryRow(request.Context(),
		`SELECT totp_secret FROM users WHERE id = $1`, userID,
	).Scan(&secret); err != nil {
		writeError(writer, http.StatusInternalServerError, "USER_NOT_FOUND", err.Error())
		return
	}
	if !secret.Valid || !ValidateTOTP(secret.String, strings.TrimSpace(req.Code)) {
		writeError(writer, http.StatusUnauthorized, "INVALID_TOTP", "authenticator code is incorrect")
		return
	}
	if _, err := service.pool.Exec(request.Context(),
		`UPDATE users SET totp_secret = NULL, totp_pending_secret = NULL, totp_enabled = false WHERE id = $1`, userID); err != nil {
		writeError(writer, http.StatusInternalServerError, "TOTP_DISABLE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"enabled": false})
}

func (service *Service) issueRefreshToken(ctx context.Context, userID, tenantID int64, role, ip, agent string) (token, family string, err error) {
	token = randomToken()
	family = randomUUID()
	if _, err = service.pool.Exec(ctx, `
		INSERT INTO user_tokens (user_id, token_type, token_hash, family_id, expires_at, ip_address, user_agent, tenant_id, role)
		VALUES ($1, 'refresh', $2, $3, now() + interval '30 days', $4, $5, $6, $7)
	`, userID, hashToken(token), family, ip, agent, tenantID, role); err != nil {
		return "", "", err
	}
	return token, family, nil
}

// ErrRefreshReuse is returned by rotateRefreshToken when the presented
// refresh token has already been revoked — evidence of a replayed (stolen or
// retried) token. Before returning, the entire token family is revoked so
// every descendant of the compromised chain becomes unusable. Handlers must
// answer this with 401 INVALID_REFRESH without disclosing the reason.
var ErrRefreshReuse = errors.New("refresh token reuse detected")

func (service *Service) rotateRefreshToken(ctx context.Context, oldID, userID, tenantID int64, role, family, ip, agent string) (token, newFamily string, err error) {
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	// Lock the old token row FOR UPDATE so concurrent rotations of the same
	// token serialize: exactly one wins, the rest observe revoked_at set.
	var revokedAt pgtype.Timestamptz
	if err = tx.QueryRow(ctx,
		`SELECT revoked_at FROM user_tokens WHERE id = $1 FOR UPDATE`, oldID,
	).Scan(&revokedAt); err != nil {
		return "", "", err
	}
	if revokedAt.Valid {
		// Replay detected: kill the whole family, then reject. The caller
		// must not learn more than "invalid refresh" from the response.
		if _, err = tx.Exec(ctx,
			`UPDATE user_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, family,
		); err != nil {
			return "", "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", "", err
		}
		return "", "", ErrRefreshReuse
	}

	token = randomToken()
	newFamily = family
	if _, err = tx.Exec(ctx, `
		INSERT INTO user_tokens (user_id, token_type, token_hash, family_id, expires_at, replaced_by, ip_address, user_agent, tenant_id, role)
		VALUES ($1, 'refresh', $2, $3, now() + interval '30 days', $4, $5, $6, $7, $8)
	`, userID, hashToken(token), family, oldID, ip, agent, tenantID, role); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE user_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, oldID); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return token, newFamily, nil
}

// revokeTokenFamily revokes every still-active token in a family. Used by
// the Refresh/SwitchTenant handlers when a presented token turns out to be
// already revoked (replay evidence, F-06). rotateRefreshToken performs the
// same revocation inside its own transaction when it detects reuse under
// lock.
func (service *Service) revokeTokenFamily(ctx context.Context, family string) error {
	_, err := service.pool.Exec(ctx,
		`UPDATE user_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, family)
	return err
}

func (service *Service) issueToken(userID, tenantID int64, role string, duration time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(service.jwtSecret)
}

// validatePassword enforces a minimum password policy (i-003):
// - min 8 characters
// - at least one uppercase letter
// - at least one digit
// - at least one special character
func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errors.New("password must include an uppercase letter")
	}
	if !hasDigit {
		return errors.New("password must include a digit")
	}
	if !hasSpecial {
		return errors.New("password must include a special character (!@#$%^&*())")
	}
	// Lowercase is not enforced because we assume all input contains lowercase unless specified otherwise.
	// If the check below fails due to no lowercase, that's extremely rare in real data and would require
	// explicit uppercase+digit+special without any lowercase — practically non-existent in practice.
	if !hasLower {
		return errors.New("password must include a lowercase letter")
	}
	return nil
}

func (service *Service) parseToken(tokenString string) (*Claims, error) {
	// F-13: pin the algorithm so a token signed with a different alg (e.g.
	// RS256/none confusion attacks) is rejected before the keyfunc runs.
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return service.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, pgx.ErrNoRows
	}
	return claims, nil
}

// Middleware validates the Bearer token and injects tenant_id/user_id/role into context.
func (service *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenString := bearerToken(request)
		if tokenString == "" {
			writeError(writer, http.StatusUnauthorized, "TOKEN_REQUIRED", "authorization header is required")
			return
		}
		claims, err := service.parseToken(tokenString)
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired")
			return
		}
		ctx := context.WithValue(request.Context(), tenantIDKey, claims.TenantID)
		ctx = context.WithValue(ctx, userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// RequireRole returns middleware that rejects requests whose role is not in
// the allowed set. Must be used after Middleware (which sets the role).
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			role, _ := RoleFromContext(request.Context())
			if role == "" || !allowed[role] {
				writeError(writer, http.StatusForbidden, "FORBIDDEN", "you do not have permission to perform this action")
				return
			}
			next.ServeHTTP(writer, request.WithContext(request.Context()))
		})
	}
}

func bearerToken(request *http.Request) string {
	header := request.Header.Get("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return header[7:]
	}
	return ""
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	return hex.EncodeToString(raw)
}

func randomUUID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

// TenantIDFromContext returns the tenant id set by Middleware.
func TenantIDFromContext(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(tenantIDKey).(int64)
	return value, ok
}

// UserIDFromContext returns the user id set by Middleware.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey).(int64)
	return value, ok
}

// RoleFromContext returns the role set by Middleware.
func RoleFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(roleKey).(string)
	return value, ok
}

// ContextKeyTenantID exposes the tenant context key for tests and other
// packages that need to inject the same value the middleware sets.
func ContextKeyTenantID() contextKey {
	return tenantIDKey
}

// ContextKeyRole exposes the role context key for tests and other packages
// that need to inject the same value the middleware sets.
func ContextKeyRole() contextKey {
	return roleKey
}
