package tenant

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

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

// Create handles onboarding: it returns the user's existing tenant when they
// already have one (idempotent, so failed/retried onboarding never spawns
// orphan tenants), otherwise it creates the first tenant and links it with
// the owner role.
//
// To add a SECOND tenant to an existing account use CreateAdditional
// (POST /tenants/new), which always creates a new tenant.
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

	service.createTenant(writer, request, userID)
}

// CreateAdditional always creates a NEW tenant for the authenticated user
// (multi-tenant: one email can own several books). The caller becomes owner.
// Duplicate slugs are rejected with 409 SLUG_TAKEN.
func (service *Service) CreateAdditional(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}
	service.createTenant(writer, request, userID)
}

// createTenant is the shared tenant-creation core: validate, reject duplicate
// slugs, then insert the tenant + owner membership in one transaction.
func (service *Service) createTenant(writer http.ResponseWriter, request *http.Request, userID int64) {
	var req CreateTenantRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name and slug are required")
		return
	}

	// Reject duplicate slugs with a clear error instead of a raw SQL error.
	var slugExists bool
	err := service.pool.QueryRow(request.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenants WHERE slug = $1)`, req.Slug,
	).Scan(&slugExists)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TENANT_CREATE_FAILED", err.Error())
		return
	}
	if slugExists {
		writeError(writer, http.StatusConflict, "SLUG_TAKEN", "a tenant with this slug already exists")
		return
	}

	// Create the tenant, link it to the user, and provision the default chart
	// of accounts / categories / open period — all in one transaction so a
	// failure between the statements can never leave an orphan or unusable
	// tenant.
	var tenantID int64
	txErr := pgx.BeginFunc(request.Context(), service.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(request.Context(),
			`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`, req.Name, req.Slug,
		).Scan(&tenantID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(request.Context(),
			`INSERT INTO user_tenants (user_id, tenant_id, role) VALUES ($1, $2, 'owner')`,
			userID, tenantID,
		); err != nil {
			return err
		}
		// Provision the default COA so the new book can post immediately
		// (without this, every posting fails with ACCOUNT_NOT_FOUND).
		return auth.SeedDefaultCOA(request.Context(), tx, tenantID)
	})
	if txErr != nil {
		writeError(writer, http.StatusInternalServerError, "TENANT_CREATE_FAILED", txErr.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": tenantID, "name": req.Name, "slug": req.Slug})
}

// List returns every tenant the authenticated user belongs to, with the role
// they hold in each. This powers the tenant switcher in the UI.
func (service *Service) List(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}

	rows, err := service.pool.Query(request.Context(),
		`SELECT t.id, t.name, t.slug, ut.role
		 FROM user_tenants ut
		 JOIN tenants t ON t.id = ut.tenant_id
		 WHERE ut.user_id = $1
		 ORDER BY ut.id`, userID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TENANT_LIST_FAILED", err.Error())
		return
	}
	defer rows.Close()

	type tenantRow struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
		Role string `json:"role"`
	}
	tenants := []tenantRow{}
	for rows.Next() {
		var id int64
		var row tenantRow
		if err := rows.Scan(&id, &row.Name, &row.Slug, &row.Role); err != nil {
			writeError(writer, http.StatusInternalServerError, "TENANT_LIST_FAILED", err.Error())
			return
		}
		row.ID = strconv.FormatInt(id, 10)
		tenants = append(tenants, row)
	}
	writeJSON(writer, http.StatusOK, map[string]any{"tenants": tenants})
}

// GetMyTenant returns the ACTIVE tenant for the current session (taken from
// the JWT tenant claim), falling back to the user's first membership when the
// token predates tenant switching (tenant_id 0). Returns 404 NO_TENANT when
// the user has not completed onboarding yet.
func (service *Service) GetMyTenant(writer http.ResponseWriter, request *http.Request) {
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok || userID <= 0 {
		writeError(writer, http.StatusUnauthorized, "AUTH_REQUIRED", "user context is required")
		return
	}

	// Prefer the tenant bound to the current JWT (set at login/switch).
	activeTenant, hasActive := auth.TenantIDFromContext(request.Context())
	activeRole, _ := auth.RoleFromContext(request.Context())

	var tenantID int64
	var tenantName, tenantSlug, role string
	var err error
	if hasActive && activeTenant > 0 {
		// Still verify membership so a forged/stale token cannot read a
		// tenant the user no longer belongs to.
		err = service.pool.QueryRow(request.Context(),
			`SELECT t.id, t.name, t.slug, ut.role
			 FROM user_tenants ut
			 JOIN tenants t ON t.id = ut.tenant_id
			 WHERE ut.user_id = $1 AND ut.tenant_id = $2`, userID, activeTenant,
		).Scan(&tenantID, &tenantName, &tenantSlug, &role)
	}
	if !hasActive || activeTenant <= 0 || err != nil {
		err = service.pool.QueryRow(request.Context(),
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
	} else if role == "" {
		role = activeRole
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

// HealthDetailed returns component-level health (N-10): API always ok if it
// can respond, database via a ping, and the NextReport rendering sidecar via
// its /health endpoint (only checked when NEXTREPORT_URL is configured).
func (service *Service) HealthDetailed(writer http.ResponseWriter, request *http.Request) {
	result := map[string]any{"status": "ok"}

	// Database connectivity.
	if err := service.pool.Ping(request.Context()); err != nil {
		result["database"] = map[string]any{"status": "down", "error": err.Error()}
		result["status"] = "degraded"
	} else {
		result["database"] = map[string]any{"status": "up"}
	}

	// NextReport rendering sidecar (optional — only when configured).
	nextreportURL := os.Getenv("NEXTREPORT_URL")
	if nextreportURL != "" {
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(nextreportURL + "/health")
		if err != nil {
			result["nextreport"] = map[string]any{"status": "down", "error": err.Error()}
			result["status"] = "degraded"
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				result["nextreport"] = map[string]any{"status": "up"}
			} else {
				result["nextreport"] = map[string]any{"status": "down", "http_code": resp.StatusCode}
				result["status"] = "degraded"
			}
		}
	} else {
		result["nextreport"] = map[string]any{"status": "not_configured"}
	}

	writeJSON(writer, http.StatusOK, result)
}
