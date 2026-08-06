package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	tenantIDKey contextKey = "tenant_id"
	userIDKey   contextKey = "user_id"
)

type Service struct {
	pool      *pgxpool.Pool
	jwtSecret []byte
}

func NewService(pool *pgxpool.Pool, secret string) *Service {
	return &Service{pool: pool, jwtSecret: []byte(secret)}
}

type Claims struct {
	UserID   int64 `json:"user_id"`
	TenantID int64 `json:"tenant_id"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (service *Service) Register(writer http.ResponseWriter, request *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Email == "" || len(req.Password) < 8 || req.FullName == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "email, password (min 8), and full_name are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REGISTER_FAILED", "could not hash password")
		return
	}
	var userID int64
	err = service.pool.QueryRow(request.Context(),
		`INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		req.Email, string(hash), req.FullName,
	).Scan(&userID)
	if err != nil {
		writeError(writer, http.StatusConflict, "EMAIL_EXISTS", "email is already registered")
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": userID, "email": req.Email})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (service *Service) Login(writer http.ResponseWriter, request *http.Request) {
	var req LoginRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var userID int64
	var passwordHash string
	err := service.pool.QueryRow(request.Context(),
		`SELECT id, password_hash FROM users WHERE email = $1 AND is_active = true`, req.Email,
	).Scan(&userID, &passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeError(writer, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
		return
	}
	accessToken, err := service.issueToken(userID, 0, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	refreshToken, familyID, err := service.issueRefreshToken(request.Context(), userID, request.RemoteAddr, request.UserAgent())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue refresh token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"family_id":     familyID,
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
	var tokenID int64
	var familyID string
	var expiresAt time.Time
	err := service.pool.QueryRow(request.Context(), `
		SELECT id, user_id, family_id, expires_at
		FROM user_tokens
		WHERE token_hash = $1 AND token_type = 'refresh' AND revoked_at IS NULL
	`, hash).Scan(&tokenID, &userID, &familyID, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		writeError(writer, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token is invalid or expired")
		return
	}
	// Rotate: revoke the old token and store a new one in the same family.
	newRefresh, newFamily, err := service.rotateRefreshToken(request.Context(), tokenID, userID, familyID, request.RemoteAddr, request.UserAgent())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not rotate refresh token")
		return
	}
	accessToken, err := service.issueToken(userID, 0, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": newRefresh,
		"family_id":     newFamily,
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

func (service *Service) issueRefreshToken(ctx context.Context, userID int64, ip, agent string) (token, family string, err error) {
	token = randomToken()
	family = randomUUID()
	if _, err = service.pool.Exec(ctx, `
		INSERT INTO user_tokens (user_id, token_type, token_hash, family_id, expires_at, ip_address, user_agent)
		VALUES ($1, 'refresh', $2, $3, now() + interval '30 days', $4, $5)
	`, userID, hashToken(token), family, ip, agent); err != nil {
		return "", "", err
	}
	return token, family, nil
}

func (service *Service) rotateRefreshToken(ctx context.Context, oldID, userID int64, family, ip, agent string) (token, newFamily string, err error) {
	token = randomToken()
	newFamily = family
	if _, err = service.pool.Exec(ctx, `
		INSERT INTO user_tokens (user_id, token_type, token_hash, family_id, expires_at, replaced_by, ip_address, user_agent)
		VALUES ($1, 'refresh', $2, $3, now() + interval '30 days', $4, $5, $6)
	`, userID, hashToken(token), family, oldID, ip, agent); err != nil {
		return "", "", err
	}
	if _, err = service.pool.Exec(ctx,
		`UPDATE user_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, oldID); err != nil {
		return "", "", err
	}
	return token, newFamily, nil
}

func (service *Service) issueToken(userID, tenantID int64, duration time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(service.jwtSecret)
}

func (service *Service) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return service.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, pgx.ErrNoRows
	}
	return claims, nil
}

// Middleware validates the Bearer token and injects tenant_id/user_id into context.
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
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func bearerToken(request *http.Request) string {
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
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

// ContextKeyTenantID exposes the tenant context key for tests and other
// packages that need to inject the same value the middleware sets.
func ContextKeyTenantID() contextKey {
	return tenantIDKey
}
