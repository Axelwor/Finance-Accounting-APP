package auth

import (
	"context"
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
	token, err := service.issueToken(userID, 0, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"access_token": token})
}

type RefreshRequest struct {
	AccessToken string `json:"access_token"`
}

func (service *Service) Refresh(writer http.ResponseWriter, request *http.Request) {
	var req RefreshRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	claims, err := service.parseToken(req.AccessToken)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "INVALID_TOKEN", "token is invalid or expired")
		return
	}
	token, err := service.issueToken(claims.UserID, claims.TenantID, 15*time.Minute)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TOKEN_FAILED", "could not issue token")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"access_token": token})
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
