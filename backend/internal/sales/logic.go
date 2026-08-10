package sales

import (
	"fmt"
	"math"
	"strings"
)

// Quotation statuses. SQ is a commitment only and never posts a journal.
const (
	statusDraft     = "DRAFT"
	statusSent      = "SENT"
	statusConverted = "CONVERTED"
	statusExpired   = "EXPIRED"
	statusCancelled = "CANCELLED"
)

// QuotationLineRequest is one line of a create-quotation request.
type QuotationLineRequest struct {
	ItemID         int64   `json:"item_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	DiscountCents  int64   `json:"discount_cents"`
	TaxRate        float64 `json:"tax_rate"`
	Description    string  `json:"description"`
}

// CreateQuotationRequest is the POST /quotations body.
type CreateQuotationRequest struct {
	CustomerID    int64                  `json:"customer_id"`
	QuotationDate string                 `json:"quotation_date"`
	ValidUntil    string                 `json:"valid_until"`
	PaymentTermID int64                  `json:"payment_term_id"`
	Notes         string                 `json:"notes"`
	SourceRef     string                 `json:"source_ref"`
	Lines         []QuotationLineRequest `json:"lines"`
}

// preparedLine carries a validated line plus its computed line total so the
// handler can insert it and sum the quotation total without recomputing.
type preparedLine struct {
	Line           QuotationLineRequest
	LineTotalCents int64
}

// lineTotalCents computes line_total_cents = qty * unit_price_cents - discount_cents.
// qty is NUMERIC(18,3); to avoid float64 rounding errors we convert qty to
// milliunits (qty * 1000, rounded to nearest integer), multiply by
// unit_price_cents (integer), then divide by 1000 with rounding (round half up).
func lineTotalCents(qty float64, unitPriceCents, discountCents int64) int64 {
	qtyMilli := int64(math.Round(qty * 1000))
	if qtyMilli <= 0 {
		return -discountCents
	}
	grossMilli := qtyMilli * unitPriceCents
	grossCents := (grossMilli + 500) / 1000 // round half up
	return grossCents - discountCents
}

// validDate reports whether raw is a YYYY-MM-DD date.
func validDate(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := parseDate(raw)
	return err == nil
}

// validateCreateRequest validates the create body. It returns "" code on
// success and, on failure, the error code and message. It is pure (no DB access).
func validateCreateRequest(req CreateQuotationRequest) (string, string) {
	if req.CustomerID <= 0 {
		return "INVALID_REQUEST", "customer_id is required"
	}
	if !validDate(req.QuotationDate) {
		return "INVALID_REQUEST", "quotation_date must be a valid date in YYYY-MM-DD format"
	}
	if req.ValidUntil != "" && !validDate(req.ValidUntil) {
		return "INVALID_REQUEST", "valid_until must be a valid date in YYYY-MM-DD format"
	}
	if len(req.Lines) == 0 {
		return "INVALID_REQUEST", "at least one line is required"
	}
	for index, line := range req.Lines {
		if line.ItemID <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: item_id is required", index)
		}
		if line.Qty <= 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: qty must be greater than 0", index)
		}
		if line.UnitPriceCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: unit_price_cents must be >= 0", index)
		}
		if line.DiscountCents < 0 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: discount_cents must be >= 0", index)
		}
		if line.TaxRate < 0 || line.TaxRate > 100 {
			return "INVALID_REQUEST", fmt.Sprintf("lines[%d]: tax_rate must be between 0 and 100", index)
		}
	}
	return "", ""
}

// prepareLines validates again and computes each line total. It is pure and is
// the single source of the quotation total used by the handler.
func prepareLines(lines []QuotationLineRequest) ([]preparedLine, int64, error) {
	prepared := make([]preparedLine, 0, len(lines))
	var total int64
	for _, line := range lines {
		if line.Qty <= 0 {
			return nil, 0, fmt.Errorf("lines: qty must be greater than 0")
		}
		if line.UnitPriceCents < 0 {
			return nil, 0, fmt.Errorf("lines: unit_price_cents must be >= 0")
		}
		if line.DiscountCents < 0 {
			return nil, 0, fmt.Errorf("lines: discount_cents must be >= 0")
		}
		lineTotal := lineTotalCents(line.Qty, line.UnitPriceCents, line.DiscountCents)
		total += lineTotal
		prepared = append(prepared, preparedLine{Line: line, LineTotalCents: lineTotal})
	}
	return prepared, total, nil
}

// canSend reports whether a DRAFT quotation may be sent.
func canSend(current string) bool {
	return current == statusDraft
}

// canCancel reports whether the quotation may be cancelled.
func canCancel(current string) bool {
	return current == statusDraft || current == statusSent
}

// canExpire reports whether the quotation may be marked expired. It is skipped
// for already-finished quotations (CANCELLED/CONVERTED).
func canExpire(current string) bool {
	return current != statusCancelled && current != statusConverted
}
