package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-accounting-app/backend/internal/accounting"
	"finance-accounting-app/backend/internal/assets"
	"finance-accounting-app/backend/internal/audit"
	"finance-accounting-app/backend/internal/auth"
	"finance-accounting-app/backend/internal/budget"
	"finance-accounting-app/backend/internal/cash"
	"finance-accounting-app/backend/internal/coa"
	"finance-accounting-app/backend/internal/config"
	"finance-accounting-app/backend/internal/customer"
	"finance-accounting-app/backend/internal/inventory"
	"finance-accounting-app/backend/internal/item"
	"finance-accounting-app/backend/internal/notes"
	"finance-accounting-app/backend/internal/period"
	"finance-accounting-app/backend/internal/production"
	"finance-accounting-app/backend/internal/purchase"
	"finance-accounting-app/backend/internal/reconciliation"
	"finance-accounting-app/backend/internal/reporting"
	"finance-accounting-app/backend/internal/sales"
	"finance-accounting-app/backend/internal/tax"
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
	accountingHandler := accounting.NewHandler(pool)
	periodHandler := period.NewHandler(pool)
	customerHandler := customer.NewHandler(pool)
	itemHandler := item.NewHandler(pool)
	salesHandler := sales.NewHandler(pool)

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

			router.Post("/journal-entries", accountingHandler.CreateManualJournal)
			router.Get("/journal-entries", accountingHandler.ListJournalEntries)
			router.Get("/journal-entries/{id}", accountingHandler.GetJournalEntry)
			router.Get("/general-ledger", accountingHandler.GetGeneralLedger)
			router.Get("/journal-register", accountingHandler.GetJournalRegister)

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

			// Export endpoints: ?format=pdf|xlsx plus optional from_date/to_date.
			router.Get("/reports/trial-balance/export", reportingHandler.Export)
			router.Get("/reports/profit-loss/export", reportingHandler.Export)
			router.Get("/reports/balance-sheet/export", reportingHandler.Export)
			router.Get("/reports/cash-flow/export", reportingHandler.Export)

			router.Post("/periods/close", periodHandler.Close)
			router.Post("/periods/unlock", periodHandler.Unlock)

			router.Get("/customers", customerHandler.ListCustomers)
			router.Post("/customers", customerHandler.CreateCustomer)
			router.Get("/customers/{id}", customerHandler.GetCustomer)
			router.Post("/customers/{id}/deactivate", customerHandler.DeactivateCustomer)
			router.Get("/payment-terms", customerHandler.ListPaymentTerms)
			router.Post("/payment-terms", customerHandler.CreatePaymentTerm)

			router.Get("/items", itemHandler.List)
			router.Post("/items", itemHandler.Create)
			router.Post("/items/{id}/deactivate", itemHandler.Deactivate)
			router.Get("/items/{id}/prices", itemHandler.ListPrices)
			router.Post("/items/{id}/prices", itemHandler.CreatePrice)

			router.Get("/quotations", salesHandler.List)
			router.Post("/quotations", salesHandler.Create)
			router.Get("/quotations/{id}", salesHandler.Get)
			router.Post("/quotations/{id}/send", salesHandler.Send)
			router.Post("/quotations/{id}/cancel", salesHandler.Cancel)
			router.Post("/quotations/{id}/mark-expired", salesHandler.MarkExpired)

			router.Post("/sales-orders", salesHandler.CreateOrder)
			router.Get("/sales-orders", salesHandler.ListOrders)
			router.Get("/sales-orders/{id}", salesHandler.GetOrder)
			router.Post("/sales-orders/{id}/cancel", salesHandler.CancelOrder)

			router.Post("/sales-orders/{id}/down-payments", salesHandler.CreateDP)
			router.Get("/sales-orders/{id}/down-payments", salesHandler.ListDPs)
			router.Post("/down-payments/{id}/refund", salesHandler.RefundDP)

			router.Post("/delivery-orders", salesHandler.CreateDelivery)
			router.Get("/delivery-orders", salesHandler.ListDeliveries)
			router.Get("/delivery-orders/{id}", salesHandler.GetDelivery)

			router.Post("/invoices", salesHandler.CreateInvoice)
			router.Get("/invoices", salesHandler.ListInvoices)
			router.Get("/invoices/{id}", salesHandler.GetInvoice)
			router.Post("/invoices/{id}/payments", salesHandler.CreatePayment)
			router.Get("/invoices/{id}/payments", salesHandler.ListPayments)

			router.Post("/credit-notes", salesHandler.CreateCreditNote)
			router.Get("/credit-notes", salesHandler.ListCreditNotes)
			router.Get("/credit-notes/{id}", salesHandler.GetCreditNote)

			purchaseHandler := purchase.NewHandler(pool)
			purchaseHandler.Routes(router)

			inventoryHandler := inventory.NewHandler(pool)
			inventoryHandler.Routes(router)

			router.Post("/supplier-invoices", purchaseHandler.CreateSupplierInvoice)
			router.Get("/supplier-invoices", purchaseHandler.ListSupplierInvoices)
			router.Get("/supplier-invoices/{id}", purchaseHandler.GetSupplierInvoice)
			router.Post("/supplier-invoices/{id}/payments", purchaseHandler.CreateSupplierPayment)
			router.Get("/supplier-invoices/{id}/payments", purchaseHandler.ListSupplierPayments)

			// US-050: Bank Reconciliation (Rekonsiliasi Kas & Bank).
			reconciliationHandler := reconciliation.NewHandler(pool)
			reconciliationHandler.Routes(router)

			// P2-016: Catatan atas Laporan Keuangan + Pengingat Jatuh Tempo.
			notesHandler := notes.NewHandler(pool)
			notesHandler.Routes(router)
			router.Get("/reminders/due-dates", notesHandler.DueDateReminders)

			assetsHandler := assets.NewHandler(pool)
			assetsHandler.Routes(router)

			productionHandler := production.NewHandler(pool)
			productionHandler.Routes(router)
			// US-080..083: Tax (PPN, PPh Final UMKM, ECL, Deferred Tax).
			taxHandler := tax.NewHandler(pool)
			taxHandler.Routes(router)

			// US-090A + US-093: Report frameworks, dimensions, and budget vs actual.
			budgetHandler := budget.NewHandler(pool)
			budgetHandler.Routes(router)
			// US-100/US-101: Attachments (Lampirkan Bukti) + Audit Trail.
			auditHandler := audit.NewHandler(pool, cfg.StorageRoot)
			auditHandler.Routes(router)
		})
	})

	log.Printf("api listening on %s", cfg.HTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, router))
}
