package item

import "testing"

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
				IsTrackedStock: false,
			},
			want: "",
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
