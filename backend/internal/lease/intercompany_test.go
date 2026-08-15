package lease

import (
	"testing"
)

// TestMatchEliminationPairs_SalePurchase verifies the canonical elimination:
// a SALE registered by tenant A against B pairs with the PURCHASE registered
// by tenant B against A, same amount.
func TestMatchEliminationPairs_SalePurchase(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 500000},
		{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: "PURCHASE", journalEntryID: 20, amountCents: 500000},
	}
	pairs := matchEliminationPairs(txs)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0][0].id != 1 || pairs[0][1].id != 2 {
		t.Errorf("pair = (%d, %d), want (1, 2)", pairs[0][0].id, pairs[0][1].id)
	}
}

// TestMatchEliminationPairs_SameTypeMirror verifies LOAN/INTEREST/DIVIDEND
// pair with the same tx_type in the opposite direction.
func TestMatchEliminationPairs_SameTypeMirror(t *testing.T) {
	tests := []struct {
		txType string
	}{
		{txType: "LOAN"},
		{txType: "INTEREST"},
		{txType: "DIVIDEND"},
	}
	for _, tc := range tests {
		t.Run(tc.txType, func(t *testing.T) {
			txs := []icTx{
				{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: tc.txType, journalEntryID: 10, amountCents: 100000},
				{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: tc.txType, journalEntryID: 20, amountCents: 100000},
			}
			pairs := matchEliminationPairs(txs)
			if len(pairs) != 1 {
				t.Fatalf("expected 1 pair, got %d", len(pairs))
			}
		})
	}
}

// TestMatchEliminationPairs_AmountMismatch verifies transactions of the same
// type and counterparties but different amounts never pair.
func TestMatchEliminationPairs_AmountMismatch(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 500000},
		{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: "PURCHASE", journalEntryID: 20, amountCents: 499999},
	}
	if pairs := matchEliminationPairs(txs); len(pairs) != 0 {
		t.Errorf("amount mismatch must not pair, got %d pairs", len(pairs))
	}
}

// TestMatchEliminationPairs_WrongCounterparty verifies same amounts and types
// but between different tenant pairs do not match.
func TestMatchEliminationPairs_WrongCounterparty(t *testing.T) {
	txs := []icTx{
		// A sells to B, but C records a PURCHASE against D — different pair.
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 100},
		{id: 2, tenantID: 3, counterpartyTenantID: 4, txType: "PURCHASE", journalEntryID: 20, amountCents: 100},
	}
	if pairs := matchEliminationPairs(txs); len(pairs) != 0 {
		t.Errorf("different tenant pairs must not match, got %d pairs", len(pairs))
	}
}

// TestMatchEliminationPairs_OneToOne verifies strict 1:1 matching: two SALEs
// of the same amount from A→B against only one PURCHASE from B→A pairs just
// once; the second SALE stays unmatched.
func TestMatchEliminationPairs_OneToOne(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 100},
		{id: 2, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 11, amountCents: 100},
		{id: 3, tenantID: 2, counterpartyTenantID: 1, txType: "PURCHASE", journalEntryID: 20, amountCents: 100},
	}
	pairs := matchEliminationPairs(txs)
	if len(pairs) != 1 {
		t.Fatalf("expected exactly 1 pair, got %d", len(pairs))
	}
	if pairs[0][1].id != 3 {
		t.Errorf("PURCHASE leg must be id 3, got %d", pairs[0][1].id)
	}
}

// TestMatchEliminationPairs_ManagementFeeNeverPairs: MANAGEMENT_FEE has no
// mirror leg in the pairing rules and must never auto-eliminate.
func TestMatchEliminationPairs_ManagementFeeNeverPairs(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "MANAGEMENT_FEE", journalEntryID: 10, amountCents: 100},
		{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: "MANAGEMENT_FEE", journalEntryID: 20, amountCents: 100},
	}
	if pairs := matchEliminationPairs(txs); len(pairs) != 0 {
		t.Errorf("MANAGEMENT_FEE must not auto-pair, got %d pairs", len(pairs))
	}
}

// TestMatchEliminationPairs_SaleNeedsPurchase: SALE against SALE (same
// direction type) must not pair — only the SALE↔PURCHASE mirror matches.
func TestMatchEliminationPairs_SaleNeedsPurchase(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 100},
		{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: "SALE", journalEntryID: 20, amountCents: 100},
	}
	if pairs := matchEliminationPairs(txs); len(pairs) != 0 {
		t.Errorf("SALE↔SALE must not pair, got %d pairs", len(pairs))
	}
}

// TestMatchEliminationPairs_MultiplePairs verifies several independent pairs
// match in one pass.
func TestMatchEliminationPairs_MultiplePairs(t *testing.T) {
	txs := []icTx{
		{id: 1, tenantID: 1, counterpartyTenantID: 2, txType: "SALE", journalEntryID: 10, amountCents: 700},
		{id: 2, tenantID: 2, counterpartyTenantID: 1, txType: "PURCHASE", journalEntryID: 20, amountCents: 700},
		{id: 3, tenantID: 1, counterpartyTenantID: 3, txType: "LOAN", journalEntryID: 11, amountCents: 900},
		{id: 4, tenantID: 3, counterpartyTenantID: 1, txType: "LOAN", journalEntryID: 30, amountCents: 900},
		// Unmatched leftover.
		{id: 5, tenantID: 2, counterpartyTenantID: 1, txType: "PURCHASE", journalEntryID: 21, amountCents: 123},
	}
	pairs := matchEliminationPairs(txs)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	got := map[[2]int64]bool{}
	for _, p := range pairs {
		got[[2]int64{p[0].id, p[1].id}] = true
	}
	if !got[[2]int64{1, 2}] || !got[[2]int64{3, 4}] {
		t.Errorf("expected pairs (1,2) and (3,4), got %v", got)
	}
}

// TestMatchEliminationPairs_Empty: no rows, no pairs, no panic.
func TestMatchEliminationPairs_Empty(t *testing.T) {
	if pairs := matchEliminationPairs(nil); len(pairs) != 0 {
		t.Errorf("nil input must yield no pairs, got %d", len(pairs))
	}
}

// ---------------------------------------------------------------------------
// create request validation
// ---------------------------------------------------------------------------

func TestValidateCreateInterCompanyTx(t *testing.T) {
	valid := CreateInterCompanyTxRequest{
		CounterpartyTenantID: 2,
		TxType:               "SALE",
		JournalEntryID:       10,
		AmountCents:          500000,
		TxDate:               "2026-08-15",
		Description:          "monthly resale",
	}
	tests := []struct {
		name       string
		mutate     func(*CreateInterCompanyTxRequest)
		wantErr    bool
		wantErrMsg string
	}{
		{name: "valid", mutate: func(*CreateInterCompanyTxRequest) {}, wantErr: false},
		{name: "valid without journal entry", mutate: func(r *CreateInterCompanyTxRequest) { r.JournalEntryID = 0 }, wantErr: false},
		{name: "missing counterparty", mutate: func(r *CreateInterCompanyTxRequest) { r.CounterpartyTenantID = 0 },
			wantErr: true, wantErrMsg: "counterparty_tenant_id is required"},
		{name: "invalid tx type", mutate: func(r *CreateInterCompanyTxRequest) { r.TxType = "GIFT" },
			wantErr: true, wantErrMsg: "tx_type must be one of"},
		{name: "zero amount", mutate: func(r *CreateInterCompanyTxRequest) { r.AmountCents = 0 },
			wantErr: true, wantErrMsg: "amount_cents must be greater than 0"},
		{name: "negative amount", mutate: func(r *CreateInterCompanyTxRequest) { r.AmountCents = -5 },
			wantErr: true, wantErrMsg: "amount_cents must be greater than 0"},
		{name: "bad date", mutate: func(r *CreateInterCompanyTxRequest) { r.TxDate = "15/08/2026" },
			wantErr: true, wantErrMsg: "tx_date must be a valid"},
		{name: "negative journal id", mutate: func(r *CreateInterCompanyTxRequest) { r.JournalEntryID = -1 },
			wantErr: true, wantErrMsg: "journal_entry_id must be a positive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			code, msg := validateCreateInterCompanyTx(req)
			if (code != "") != tc.wantErr {
				t.Fatalf("code = %q, msg = %q; wantErr = %v", code, msg, tc.wantErr)
			}
			if tc.wantErr && msg == "" {
				t.Fatal("expected a message with the error code")
			}
			if tc.wantErr && wantContains(msg, tc.wantErrMsg) == false {
				t.Errorf("msg = %q, want it to contain %q", msg, tc.wantErrMsg)
			}
		})
	}
}

func wantContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestValidICTxTypes mirrors the DB CHECK constraint: every accepted tx_type
// must be in the map, and unknown types rejected.
func TestValidICTxTypes(t *testing.T) {
	for _, want := range []string{"SALE", "PURCHASE", "LOAN", "INTEREST", "DIVIDEND", "MANAGEMENT_FEE"} {
		if !validICTxTypes[want] {
			t.Errorf("tx_type %q must be valid", want)
		}
	}
	for _, bad := range []string{"", "sale", "Sale", "GIFT", "TAX"} {
		if validICTxTypes[bad] {
			t.Errorf("tx_type %q must be invalid", bad)
		}
	}
}
