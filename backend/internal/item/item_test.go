package item

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func ptr(v string) *string { return &v }
func ptrI(v int64) *int64  { return &v }

func TestValidateCreateService(t *testing.T) {
	cases := []struct {
		name string
		req  ItemRequest
		want string
	}{
		{
			name: "valid goods",
			req: ItemRequest{
				Code: "BRG-001", Name: "Widget", ItemType: "goods",
				CostingMethod: ptr("fifo"), InventoryAccountID: ptrI(1301),
				CogsAccountID: ptrI(5101), IsTrackedStock: true,
			},
			want: "",
		},
		{
			name: "valid service",
			req: ItemRequest{
				Code: "JAS-001", Name: "Consulting", ItemType: "service",
				SaleAccountID: ptrI(4101), IsTrackedStock: false,
			},
			want: "",
		},
		{
			name: "service missing sale account (QA-19)",
			req: ItemRequest{
				Code: "JAS-002", Name: "Freight", ItemType: "service",
				IsTrackedStock: false,
			},
			want: `item JAS-002: service requires sale_account_id`,
		},
		{
			name: "missing code",
			req:  ItemRequest{Name: "X", ItemType: "service"},
			want: "code is required",
		},
		{
			name: "missing name",
			req:  ItemRequest{Code: "X", ItemType: "service"},
			want: "name is required",
		},
		{
			name: "bad type",
			req:  ItemRequest{Code: "X", Name: "Y", ItemType: "weapon"},
			want: "item_type must be 'goods' or 'service'",
		},
		{
			name: "goods missing costing",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "goods",
				InventoryAccountID: ptrI(1301), IsTrackedStock: true,
			},
			want: "goods requires a costing_method",
		},
		{
			name: "goods missing inventory account",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "goods",
				CostingMethod: ptr("fifo"), IsTrackedStock: true,
			},
			want: "goods requires an inventory_account_id",
		},
		{
			name: "goods missing cogs account",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "goods",
				CostingMethod: ptr("fifo"), InventoryAccountID: ptrI(1301), IsTrackedStock: true,
			},
			want: "goods requires a cogs_account_id",
		},
		{
			name: "goods not tracked stock",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "goods",
				CostingMethod: ptr("fifo"), InventoryAccountID: ptrI(1301),
				CogsAccountID: ptrI(5101), IsTrackedStock: false,
			},
			want: "goods must set is_tracked_stock = true",
		},
		{
			name: "service with inventory account",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "service",
				InventoryAccountID: ptrI(1301), IsTrackedStock: false,
			},
			want: "service cannot have an inventory_account_id",
		},
		{
			name: "service tracked stock",
			req: ItemRequest{
				Code: "X", Name: "Y", ItemType: "service", IsTrackedStock: true,
			},
			want: "service cannot be tracked stock",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := validateCreate(&c.req)
			if got != c.want {
				t.Errorf("validateCreate(%q) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestOrDefault(t *testing.T) {
	if got := orDefault("", "pcs"); got != "pcs" {
		t.Errorf("orDefault empty = %q, want pcs", got)
	}
	if got := orDefault("kg", "pcs"); got != "kg" {
		t.Errorf("orDefault kg = %q, want kg", got)
	}
}

// TestNormalizeItemRequest covers the QA-15 fix: costing_method is accepted
// case-insensitively and normalized to the DB CHECK's lowercase form.
func TestNormalizeItemRequest(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "uppercase FIFO", input: "FIFO", want: "fifo"},
		{name: "mixed case Moving_Average", input: "Moving_Average", want: "moving_average"},
		{name: "padded Specific", input: "  Specific ", want: "specific"},
		{name: "already lowercase", input: "fifo", want: "fifo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := ItemRequest{CostingMethod: ptr(tc.input)}
			normalizeItemRequest(&req)
			if req.CostingMethod == nil || *req.CostingMethod != tc.want {
				t.Errorf("normalizeItemRequest costing_method = %v, want %q", req.CostingMethod, tc.want)
			}
		})
	}
	t.Run("nil pointer untouched", func(t *testing.T) {
		req := ItemRequest{}
		normalizeItemRequest(&req)
		if req.CostingMethod != nil {
			t.Errorf("costing_method = %v, want nil", *req.CostingMethod)
		}
	})
}

// checkErr builds a pgconn check_violation error like PostgreSQL returns.
func checkErr(constraint string) error {
	return &pgconn.PgError{Code: "23514", ConstraintName: constraint, Message: "new row violates check constraint"}
}

// TestItemCheckViolationMessage covers the QA-15 fix: each items CHECK
// constraint maps to a field-specific message instead of the misleading
// generic "invalid field value (check abc_classification)".
func TestItemCheckViolationMessage(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantSubstr string
	}{
		{name: "abc_classification", err: checkErr("items_abc_classification_check"), wantSubstr: "abc_classification must be A, B, or C"},
		{name: "costing_method", err: checkErr("items_costing_method_check"), wantSubstr: "costing_method must be one of fifo, moving_average, specific"},
		{name: "item_type", err: checkErr("items_item_type_check"), wantSubstr: "item_type must be 'goods' or 'service'"},
		{name: "revenue_recognition_method", err: checkErr("items_revenue_recognition_method_check"), wantSubstr: "revenue_recognition_method must be point_in_time, over_time, milestone, or straight_line"},
		{name: "goods composite rule", err: checkErr("items_check"), wantSubstr: "goods requires a costing_method and inventory_account_id"},
		{name: "service composite rule", err: checkErr("items_check1"), wantSubstr: "service cannot have an inventory_account_id or cogs_account_id"},
		{name: "unknown constraint names it", err: checkErr("items_future_rule_check"), wantSubstr: "invalid field value (items_future_rule_check)"},
		{name: "non-check error stays generic", err: errors.New("some other failure"), wantSubstr: "invalid field value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := itemCheckViolationMessage(tc.err)
			if got != tc.wantSubstr {
				t.Errorf("itemCheckViolationMessage = %q, want %q", got, tc.wantSubstr)
			}
		})
	}
}

// TestValidateCreateNormalizedCostingMethod proves an uppercase FIFO request
// passes validation after normalization (previously bounced with a 400 that
// blamed abc_classification).
func TestValidateCreateNormalizedCostingMethod(t *testing.T) {
	req := ItemRequest{
		Code: "BRG-FIFO", Name: "Widget", ItemType: "goods",
		CostingMethod: ptr("FIFO"), InventoryAccountID: ptrI(1301),
		CogsAccountID: ptrI(5101), IsTrackedStock: true,
	}
	normalizeItemRequest(&req)
	if verr := validateCreate(&req); verr != "" {
		t.Errorf("uppercase FIFO should validate after normalization, got %q", verr)
	}
	if *req.CostingMethod != "fifo" {
		t.Errorf("normalized = %q, want fifo", *req.CostingMethod)
	}
}

func TestPathID(t *testing.T) {
	if _, err := pathID("0"); err == nil {
		t.Error("expected error for id 0")
	}
	if _, err := pathID("abc"); err == nil {
		t.Error("expected error for non-numeric id")
	}
	if id, err := pathID("42"); err != nil || id != 42 {
		t.Errorf("pathID(42) = %d, %v; want 42, nil", id, err)
	}
}
