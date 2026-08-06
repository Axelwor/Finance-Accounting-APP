package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/config"
	"finance-accounting-app/backend/internal/tenant"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(nil, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database pool: %v", err)
	}
	defer pool.Close()

	tenantHandler := tenant.NewHandler(pool)

	router := chi.NewRouter()
	router.Get("/healthz", tenant.Health)
	router.Route("/api/v1", func(router chi.Router) {
		router.Post("/tenants", tenantHandler.Create)
	})

	log.Printf("api listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, router))
}
