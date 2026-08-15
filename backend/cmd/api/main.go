package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/aging"
	"finance-accounting-app/backend/internal/approval"
	"finance-accounting-app/backend/internal/assets"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/budget"
	"finance-accounting-app/backend/internal/cash"
	"finance-accounting-app/backend/internal/cheque"
	"finance-accounting-app/backend/internal/coa"
	"finance-accounting-app/backend/internal/config"
	"finance-accounting-app/backend/internal/costcenter"
	"finance-accounting-app/backend/internal/customer"
	"finance-accounting-app/backend/internal/dashboard"
	"finance-accounting-app/backend/internal/email"
	"finance-accounting-app/backend/internal/forecast"
	"finance-accounting-app/backend/internal/inventory"
	"finance-accounting-app/backend/internal/item"
	"finance-accounting-app/backend/internal/lease"
	"finance-accounting-app/backend/internal/middleware"
	"finance-accounting-app/backend/internal/notes"
	"finance-accounting-app/backend/internal/period"
	"finance-accounting-app/backend/internal/pettycash"
	"finance-accounting-app/backend/internal/pph"
	"finance-accounting-app/backend/internal/production"
	"finance-accounting-app/backend/internal/purchase"
	"finance-accounting-app/backend/internal/reconciliation"
	"finance-accounting-app/backend/internal/recurring"
	"finance-accounting-app/backend/internal/reports"
	"finance-accounting-app/backend/internal/reporting"
	"finance-accounting-app/backend/internal/sales"
	"finance-accounting-app/backend/internal/tax"
	"finance-accounting-app/backend/internal/tenant"
	"finance-accounting-app/backend/internal/warehouse"
)

func main() {
	// Structured logging (log/slog) — JSON in production, text on local dev.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	// DB pool tuning: bounded connections, lifetime, idle timeout, and a
	// startup Ping so we fail fast when the database is unreachable.
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		slog.Error("database config parse", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		slog.Error("database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connectivity at startup.
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		cancel()
		slog.Error("database ping", "error", err)
		os.Exit(1)
	}
	cancel()

	tenantHandler := tenant.NewHandler(pool)
	authService := auth.NewService(pool, cfg.JWTSecret)
	reportingHandler := reporting.NewHandler(pool)
	coaHandler := coa.NewHandler(pool)
	cashHandler := cash.NewHandler(pool)
	accountingHandler := accounting.NewHandler(pool)
	periodHandler := period.NewHandler(pool)
	customerHandler := customer.NewHandler(pool)
	itemHandler := item.NewHandler(pool)
	salesHandler := sales.NewHandler(pool)
	purchaseHandler := purchase.NewHandler(pool)
	notesHandler := notes.NewHandler(pool)
	agingHandler := aging.NewHandler(pool)
	recurringHandler := recurring.NewHandler(pool)
	pettyCashHandler := pettycash.NewHandler(pool)
	dashboardHandler := dashboard.NewHandler(pool)
	reportsHandler := reports.NewHandler(pool)
	warehouseHandler := warehouse.NewHandler(pool)
	approvalHandler := approval.NewHandler(pool)
	pphHandler := pph.NewHandler(pool)
	forecastHandler := forecast.NewHandler(pool)
	chequeHandler := cheque.NewHandler(pool)
	costCenterHandler := costcenter.NewHandler(pool)
	emailHandler := email.NewHandler(pool)

	router := chi.NewRouter()

	// Global middleware (applied to all routes).
	router.Use(middleware.RequestID)             // inject X-Request-ID for traceability
	router.Use(middleware.Recover)              // i-008: catch panics
	router.Use(middleware.RequestLogger)        // i-009: log every request
	router.Use(middleware.CORS(middleware.DefaultCORSConfig())) // i-010: CORS
	router.Use(middleware.Timeout(60 * time.Second)) // i-011: per-request timeout

	// Rate limiter for auth endpoints (M-027: prevent brute-force).
	loginLimiter := middleware.NewRateLimiter(5, time.Minute)

	router.Get("/healthz", tenant.Health)
	router.Get("/healthz/detail", tenantHandler.HealthDetailed)
	router.Route("/api/v1", func(router chi.Router) {
		router.With(loginLimiter.Middleware).Post("/auth/login", authService.Login)
		router.Post("/auth/register", authService.Register)
		router.With(loginLimiter.Middleware).Post("/auth/refresh", authService.Refresh)
		router.Post("/auth/logout", authService.Logout)
		router.Post("/auth/switch-tenant", authService.SwitchTenant)

		router.Group(func(router chi.Router) {
			router.Use(authService.Middleware)

			// m-006: two-factor authentication setup (authenticated).
			router.With(loginLimiter.Middleware).Post("/auth/2fa/setup", authService.Setup2FA)
			router.With(loginLimiter.Middleware).Post("/auth/2fa/verify", authService.Setup2FAVerify)
			router.With(loginLimiter.Middleware).Post("/auth/2fa/disable", authService.Disable2FA)

			// Tenant lifecycle: create (onboarding, idempotent), create-new
			// (additional tenant for multi-book accounts), list (tenant
			// switcher), and get current tenant. Mounted under auth so the
			// user identity is available.
			router.Post("/tenants", tenantHandler.Create)
			router.Post("/tenants/new", tenantHandler.CreateAdditional)
			router.Get("/tenants", tenantHandler.List)
			router.Get("/tenants/me", tenantHandler.GetMyTenant)

			// --- Write operations: admin, accountant, manager, staff ---
			router.Group(func(router chi.Router) {
				router.Use(auth.RequireRole(auth.RoleOwner, auth.RoleAdmin, auth.RoleAccountant, auth.RoleManager, auth.RoleStaff))

				router.Post("/cash-in", cashHandler.CashIn)
				router.Post("/cash-out", cashHandler.CashOut)
				router.Post("/transfers", cashHandler.Transfer)
				router.Post("/opening-balances", cashHandler.OpeningBalance)
				router.Post("/journal-entries/{id}/reverse", cashHandler.Reverse)

				router.Post("/journal-entries", accountingHandler.CreateManualJournal)

				router.Post("/accounts", coaHandler.Create)
				router.Post("/categories", coaHandler.CreateCategory)
				router.Post("/report-mappings", coaHandler.CreateReportMapping)

				router.Post("/customers", customerHandler.CreateCustomer)
				router.Put("/customers/{id}", customerHandler.UpdateCustomer)
				router.Post("/customers/{id}/deactivate", customerHandler.DeactivateCustomer)
				router.Post("/payment-terms", customerHandler.CreatePaymentTerm)

				router.Post("/items", itemHandler.Create)
				router.Post("/items/{id}/deactivate", itemHandler.Deactivate)
				router.Post("/items/{id}/prices", itemHandler.CreatePrice)

				router.Post("/quotations", salesHandler.Create)
				router.Post("/quotations/{id}/send", salesHandler.Send)
				router.Post("/quotations/{id}/cancel", salesHandler.Cancel)
				router.Post("/quotations/{id}/mark-expired", salesHandler.MarkExpired)

				router.Post("/sales-orders", salesHandler.CreateOrder)
				router.Post("/sales-orders/{id}/cancel", salesHandler.CancelOrder)
				router.Post("/sales-orders/{id}/down-payments", salesHandler.CreateDP)
				router.Post("/down-payments/{id}/refund", salesHandler.RefundDP)

				router.Post("/delivery-orders", salesHandler.CreateDelivery)
				router.Post("/invoices", salesHandler.CreateInvoice)
				router.Post("/invoices/{id}/payments", salesHandler.CreatePayment)
				router.Post("/credit-notes", salesHandler.CreateCreditNote)

				router.Post("/supplier-invoices", purchaseHandler.CreateSupplierInvoice)
				router.Post("/supplier-invoices/{id}/payments", purchaseHandler.CreateSupplierPayment)

				purchaseHandler.Routes(router)

				inventoryHandler := inventory.NewHandler(pool)
				inventoryHandler.Routes(router)

				assetsHandler := assets.NewHandler(pool)
				assetsHandler.Routes(router)

				productionHandler := production.NewHandler(pool)
				productionHandler.Routes(router)

				taxHandler := tax.NewHandler(pool)
				taxHandler.Routes(router)

				budgetHandler := budget.NewHandler(pool)
				budgetHandler.Routes(router)

				leaseHandler := lease.NewHandler(pool)
			leaseHandler.Routes(router)

			reconciliationHandler := reconciliation.NewHandler(pool)
			reconciliationHandler.Routes(router)

			notesHandler := notes.NewHandler(pool)
			notesHandler.Routes(router)

			auditHandler := audit.NewHandler(pool, cfg.StorageRoot)
			auditHandler.Routes(router)

			// F-07: Recurring Transactions
			recurringHandler.Routes(router)

			// F-08: Petty Cash (Imprest)
			pettyCashHandler.Routes(router)

			// F-02: Multi-Warehouse
			warehouseHandler.Routes(router)

			// F-03: Approval Workflow
			approvalHandler.Routes(router)

			// F-12: PPh (Withholding Tax)
			pphHandler.Routes(router)

			// F-14: Giro & Cheque Management
			chequeHandler.Routes(router)

			// F-09: Cost/Profit Center
			costCenterHandler.Routes(router)

			// F-15: Email Notification (write — templates + queue)
			emailHandler.Routes(router)

			// N-01..N-10: Report Templates (CRUD)
			router.Get("/reports/templates", reportsHandler.ListTemplates)
			router.Post("/reports/templates", reportsHandler.CreateTemplate)
			router.Get("/reports/templates/{id}", reportsHandler.GetTemplate)
			router.Put("/reports/templates/{id}", reportsHandler.UpdateTemplate)
			router.Delete("/reports/templates/{id}", reportsHandler.DeleteTemplate)
			router.Post("/reports/templates/{id}/render", reportsHandler.RenderReport)

			// D-01..D-02: Dashboard widget CRUD (write)
			router.Put("/dashboard/layout", dashboardHandler.SaveLayout)
			router.Post("/dashboard/widgets", dashboardHandler.AddWidget)
			router.Put("/dashboard/widgets/{id}", dashboardHandler.UpdateWidget)
			router.Delete("/dashboard/widgets/{id}", dashboardHandler.DeleteWidget)
		})

			// --- Admin only: period close/unlock, account deactivation ---
			router.Group(func(router chi.Router) {
				router.Use(auth.RequireRole(auth.RoleOwner, auth.RoleAdmin))

				router.Post("/periods/close", periodHandler.Close)
				router.Post("/periods/unlock", periodHandler.Unlock)
				router.Post("/accounts/{id}/deactivate", coaHandler.Deactivate)
			})

			// --- Read-only: all authenticated users (including viewer) ---
			router.Get("/journal-entries", accountingHandler.ListJournalEntries)
			router.Get("/journal-entries/{id}", accountingHandler.GetJournalEntry)
			router.Get("/general-ledger", accountingHandler.GetGeneralLedger)
			router.Get("/journal-register", accountingHandler.GetJournalRegister)

			router.Get("/accounts", coaHandler.List)
			router.Get("/accounts/export", coaHandler.Export)
			router.Get("/categories", coaHandler.ListCategories)

			router.Get("/reports/trial-balance", reportingHandler.TrialBalance)
			router.Get("/reports/profit-loss", reportingHandler.ProfitLoss)
			router.Get("/reports/balance-sheet", reportingHandler.BalanceSheet)
			router.Get("/reports/cash-flow", reportingHandler.CashFlow)

			// Export endpoints: ?format=pdf|xlsx plus optional from_date/to_date.
			router.Get("/reports/trial-balance/export", reportingHandler.Export)
			router.Get("/reports/profit-loss/export", reportingHandler.Export)
			router.Get("/reports/balance-sheet/export", reportingHandler.Export)
			router.Get("/reports/cash-flow/export", reportingHandler.Export)

			router.Get("/customers", customerHandler.ListCustomers)
			router.Get("/customers/ar-balances", customerHandler.ARBalances)
			router.Get("/customers/{id}", customerHandler.GetCustomer)
			router.Get("/customers/{id}/ar-balance", customerHandler.ARBalance)
			router.Get("/payment-terms", customerHandler.ListPaymentTerms)

			router.Get("/items", itemHandler.List)
			router.Get("/items/{id}/prices", itemHandler.ListPrices)

			router.Get("/quotations", salesHandler.List)
			router.Get("/reports/quotation-stats", salesHandler.ConversionStats)
			router.Get("/quotations/{id}", salesHandler.Get)

			router.Get("/sales-orders", salesHandler.ListOrders)
			router.Get("/sales-orders/{id}", salesHandler.GetOrder)
			router.Get("/sales-orders/{id}/down-payments", salesHandler.ListDPs)

			router.Get("/delivery-orders", salesHandler.ListDeliveries)
			router.Get("/delivery-orders/{id}", salesHandler.GetDelivery)

			router.Get("/invoices", salesHandler.ListInvoices)
			router.Get("/invoices/{id}", salesHandler.GetInvoice)
			router.Get("/invoices/{id}/payments", salesHandler.ListPayments)

			router.Get("/credit-notes", salesHandler.ListCreditNotes)
			router.Get("/credit-notes/{id}", salesHandler.GetCreditNote)

			router.Get("/supplier-invoices", purchaseHandler.ListSupplierInvoices)
			router.Get("/supplier-invoices/{id}", purchaseHandler.GetSupplierInvoice)
			router.Get("/supplier-invoices/{id}/payments", purchaseHandler.ListSupplierPayments)

			router.Get("/reminders/due-dates", notesHandler.DueDateReminders)

			// F-04/F-05: AR/AP Aging (read-only)
			agingHandler.Routes(router)

			// F-02: Warehouses (read-only)
			warehouseHandler.Routes(router)

			// F-06: Cash Flow Forecast (read-only)
			router.Get("/forecast/cash-flow", forecastHandler.GetCashFlowForecast)

			// D-01..D-02: Dashboard (read-only data fetch)
			router.Get("/dashboard/layout", dashboardHandler.GetLayout)
			router.Get("/dashboard/widgets", dashboardHandler.ListWidgets)
			router.Get("/dashboard/widgets/{id}/data", dashboardHandler.GetWidgetData)
		})
	})

	slog.Info("api listening", "addr", cfg.HTTPAddr)

	// F-07: recurring-transaction scheduler. Auto-posts due recurring rows
	// every 15 minutes; cancelled together with the server on shutdown.
	schedCtx, cancelScheduler := context.WithCancel(context.Background())
	defer cancelScheduler()
	recurringHandler.StartScheduler(schedCtx, 15*time.Minute)

	// SMTP delivery worker for the email queue. Disabled (no-op, one log
	// line) unless SMTP_HOST is configured in the environment.
	emailHandler.StartSMTPWorker(schedCtx, email.SMTPConfigFromEnv())

	// Graceful shutdown: SIGINT/SIGTERM triggers a 15-second drain period
	// so in-flight requests finish before the process exits.
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}

	shutdownErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			shutdownErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-shutdownErr:
		slog.Error("server error", "error", err)
		cancelScheduler()
		os.Exit(1)
	case sig := <-sigCh:
		slog.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
	cancelScheduler()
	slog.Info("server stopped")
}
