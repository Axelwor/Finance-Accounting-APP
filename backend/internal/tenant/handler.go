package tenant

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
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

func (service *Service) Create(writer http.ResponseWriter, request *http.Request) {
	var req CreateTenantRequest
	if err := decodeJSON(request, &req); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", "name and slug are required")
		return
	}
	var tenantID int64
	err := service.pool.QueryRow(request.Context(),
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`, req.Name, req.Slug,
	).Scan(&tenantID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "TENANT_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{"id": tenantID, "name": req.Name, "slug": req.Slug})
}

func Health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}
