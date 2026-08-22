package notes

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"finance-accounting-app/backend/internal/db"
)

// dueDateReminder is one row in the due-dates reminder result. It unions
// customer invoices (receivable, direction=customer) with supplier invoices
// (payable, direction=supplier), filtering out PAID/VOID rows whose due_date
// is within `days_ahead` days from today (or already overdue).
type dueDateReminder struct {
	ID          int64     `json:"id"`
	Number      string    `json:"number"`
	PartyName   string    `json:"party_name"`
	Direction   string    `json:"direction"` // "customer" | "supplier"
	InvoiceDate time.Time `json:"invoice_date"`
	DueDate     time.Time `json:"due_date"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
	DaysOverdue int       `json:"days_overdue"`
}

// DueDateReminders returns customer + supplier invoices whose due_date falls
// within `days_ahead` days from today (or is already overdue) and whose status
// is not PAID or VOID. The result is ordered by due_date ascending.
//
// GET /api/v1/reminders/due-dates?days_ahead=7
func (service *Service) DueDateReminders(writer http.ResponseWriter, request *http.Request) {
	tenant, err := tenantID(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	daysAhead := 7
	if raw := request.URL.Query().Get("days_ahead"); raw != "" {
		if parsed, perr := strconv.Atoi(raw); perr == nil && parsed >= 0 {
			daysAhead = parsed
		}
	}

	// Use a CTE so the same cutoff + today are applied to both halves of the
	// union and the days_overdue computation is consistent.
	reminders := make([]dueDateReminder, 0)
	err = db.WithTenantData(request.Context(), service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(request.Context(), `
		WITH cutoff AS (
			SELECT CURRENT_DATE AS today,
			       CURRENT_DATE + ($2 || ' days')::INTERVAL AS horizon
		)
		SELECT id, number, party_name, direction, invoice_date, due_date,
		       amount_cents, status,
		       (CURRENT_DATE - due_date)::INT AS days_overdue
		FROM (
			SELECT i.id, i.number, c.name AS party_name, 'customer' AS direction,
			       i.invoice_date, i.due_date, i.receivable_cents AS amount_cents, i.status
			FROM invoices i
			JOIN customers c ON c.tenant_id = i.tenant_id AND c.id = i.customer_id
			WHERE i.tenant_id = $1
			  AND i.status NOT IN ('PAID','VOID')
			  AND i.due_date IS NOT NULL
			  AND i.due_date <= (SELECT horizon FROM cutoff)
			UNION ALL
			SELECT si.id, si.number, s.name AS party_name, 'supplier' AS direction,
			       si.invoice_date, si.due_date, si.payable_cents AS amount_cents, si.status
			FROM supplier_invoices si
			JOIN suppliers s ON s.tenant_id = si.tenant_id AND s.id = si.supplier_id
			WHERE si.tenant_id = $1
			  AND si.status NOT IN ('PAID','VOID')
			  AND si.due_date IS NOT NULL
			  AND si.due_date <= (SELECT horizon FROM cutoff)
		) AS combined
		ORDER BY due_date ASC, direction ASC
	`, tenant, strconv.Itoa(daysAhead))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r dueDateReminder
			if err := rows.Scan(&r.ID, &r.Number, &r.PartyName, &r.Direction,
				&r.InvoiceDate, &r.DueDate, &r.AmountCents, &r.Status, &r.DaysOverdue); err != nil {
				return err
			}
			reminders = append(reminders, r)
		}
		return rows.Err()
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "REMINDER_FAILED", err.Error())
		return
	}

	writeJSON(writer, http.StatusOK, reminders)
}
