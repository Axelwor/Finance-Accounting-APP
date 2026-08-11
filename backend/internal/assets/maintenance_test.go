package assets

import (
	"testing"
)

// F-13: asset maintenance validation tests (pure function, no DB).
// computeNBV tests live in register_test.go.

func TestValidateMaintenance(t *testing.T) {
	valid := maintenanceRequest{
		AssetID:         1,
		MaintenanceDate: "2026-08-11",
		MaintenanceType: "ROUTINE",
		Description:     "oil change",
		CostCents:       50000,
		NextDueDate:     "2026-11-11",
	}
	if code, msg := validateMaintenance(valid); code != "" {
		t.Fatalf("valid request rejected: %s %s", code, msg)
	}

	cases := []struct {
		name    string
		mutate  func(*maintenanceRequest)
		wantErr bool
	}{
		{"zero asset_id", func(r *maintenanceRequest) { r.AssetID = 0 }, true},
		{"negative asset_id", func(r *maintenanceRequest) { r.AssetID = -1 }, true},
		{"empty maintenance_date", func(r *maintenanceRequest) { r.MaintenanceDate = "" }, true},
		{"bad maintenance_date", func(r *maintenanceRequest) { r.MaintenanceDate = "11-08-2026" }, true},
		{"empty maintenance_type", func(r *maintenanceRequest) { r.MaintenanceType = "" }, true},
		{"bad maintenance_type", func(r *maintenanceRequest) { r.MaintenanceType = "BREAKDOWN" }, true},
		{"lowercase type accepted (uppercased)", func(r *maintenanceRequest) { r.MaintenanceType = "repair" }, false},
		{"negative cost", func(r *maintenanceRequest) { r.CostCents = -1 }, true},
		{"zero cost allowed", func(r *maintenanceRequest) { r.CostCents = 0 }, false},
		{"empty next_due_date allowed", func(r *maintenanceRequest) { r.NextDueDate = "" }, false},
		{"bad next_due_date", func(r *maintenanceRequest) { r.NextDueDate = "soon" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			code, _ := validateMaintenance(req)
			if tc.wantErr && code == "" {
				t.Errorf("expected error for %s, got none", tc.name)
			}
			if !tc.wantErr && code != "" {
				t.Errorf("unexpected error for %s: %s", tc.name, code)
			}
		})
	}
}
