package tenant

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Create creates a new tenant for the authenticated user and links it via
// user_tenants with the owner role. If the user already owns a tenant the
// existing tenant is returned (idempotent), so onboarding retries and
// duplicate submissions never produce orphan tenants.
//
// NOTE: this handler is mounted INSIDE the auth middleware (not on the public
// route it once was) so the user identity is available from the JWT.
func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}

	// If the user already has a tenant, return it instead of creating another.
	var existingID int64
	var existingName, existingSlug string
	err := service.pool.QueryRow(request.Context(),
		`SELECT ut.tenant_id, t.name, t.slug
		 FROM user_tenants ut
		 JOIN tenants t ON t.id = ut.tenant_id
		 WHERE ut.user_id = $1
		 ORDER BY ut.id LIMIT 1`, userID,
	).Scan(&existingID, &existingName, &existingSlug)
	if err == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"id": existingID, "name": existingName, "slug": existingSlug})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, http.StatusInternalServerError, "TENANT_CREATE_FAILED", err.Error())
		return
	}

	var req CreateTenantRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name and slug are required")
		return
	}

	// Create the tenant and link it to the user in one transaction so a
	// failure between the two statements can never leave an orphan tenant.
	var tenantID int64
	txErr := pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(request.Context(),
			`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`, req.Name, req.Slug,
		).Scan(&tenantID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(request.Context(),
			`INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'owner')`,
			userID, tenantID,
		)
		return err
	})
	if txErr != nil {
		writeError(writer, http.StatusInternalServerError, "TENANT_CREATE_FAILED", txErr.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": tenantID, "name": req.Name, "slug": req.Slug})
}

// GetMyTenant returns the tenant linked to the authenticated user, so the
// frontend can skip onboarding on repeat logins. Returns 404 NO_TENANT when
// the user has not completed onboarding yet.
func (service *Service) GetMyTenant(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}

	var tenantID int64
	var tenantName, tenantSlug, role string
	err := service.pool.QueryRow(request.Context(),
		`SELECT ut.tenant_id, t.name, t.slug, ut.role
		 FROM user_tenants ut
		 JOIN tenants t ON t.id = ut.tenant_id
		 WHERE ut.user_id = $1
		 ORDER BY ut.id LIMIT 1`, userID,
	).Scan(&tenantID, &tenantName, &tenantSlug, &role)
	if err != nil {
		writeError(writer, http.StatusNotFound, "NO_TENANT", "user has no tenant yet")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"id":   strconv.FormatInt(tenantID, 10),
		"name": tenantName,
		"slug": tenantSlug,
		"role": role,
	})
}

func Health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}
