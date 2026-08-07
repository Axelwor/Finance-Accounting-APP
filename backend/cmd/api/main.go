package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/cash"
	"finance-accounting-app/backend/internal/coa"
	"finance-accounting-app/backend/internal/config"
	"finance-accounting-app/backend/internal/period"
	"finance-accounting-app/backend/internal/reporting"
	"finance-accounting-app/backend/internal/tenant"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database pool: %v", err)
	}
	defer pool.Close()

	tenantHandler := tenant.NewHandler(pool)
	authService := auth.NewService(pool, cfg.JWTSecret)
	reportingHandler := reporting.NewHandler(pool)
	coaHandler := coa.NewHandler(pool)
	cashHandler := cash.NewHandler(pool)
	periodHandler := period.NewHandler(pool)

	router := chi.NewRouter()
	router.Get("/healthz", tenant.Health)
	router.Route("/api/v1", func(router chi.Router) {
		router.Post("/auth/register", authService.Register)
		router.Post("/auth/login", authService.Login)
		router.Post("/auth/refresh", authService.Refresh)
		router.Post("/auth/logout", authService.Logout)
		router.Post("/tenants", tenantHandler.Create)

		router.Group(func(router chi.Router) {
			router.Use(authService.Middleware)

			router.Post("/cash-in", cashHandler.CashIn)
			router.Post("/cash-out", cashHandler.CashOut)
			router.Post("/transfers", cashHandler.Transfer)
			router.Post("/opening-balances", cashHandler.OpeningBalance)
			router.Post("/journal-entries/{id}/reverse", cashHandler.Reverse)

			router.Get("/accounts", coaHandler.List)
			router.Post("/accounts", coaHandler.Create)
			router.Post("/accounts/{id}/deactivate", coaHandler.Deactivate)
			router.Get("/categories", coaHandler.ListCategories)
			router.Post("/categories", coaHandler.CreateCategory)
			router.Post("/report-mappings", coaHandler.CreateReportMapping)

			router.Get("/reports/trial-balance", reportingHandler.TrialBalance)
			router.Get("/reports/profit-loss", reportingHandler.ProfitLoss)
			router.Get("/reports/balance-sheet", reportingHandler.BalanceSheet)
			router.Get("/reports/cash-flow", reportingHandler.CashFlow)

			router.Post("/periods/close", periodHandler.Close)
		})
	})

	log.Printf("api listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, router))
}
