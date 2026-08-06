package accounting

import (
	"errors"
	"testing"
)

func testAccount(id int64, accountType AccountType) Account {
	return Account{ID: id, Type: accountType, IsActive: true}
}

func TestCashIn(t *testing.T) {
	journal, err := CashIn(CashIntent{
		TenantID:       1,
		SourceRef:      "BK-1",
		EntryDate:      "2026-08-06",
		CashAccount:    testAccount(1101, AccountCash),
		CounterAccount: testAccount(4101, AccountRevenue),
		AmountCents:    500000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if journal.IntentType != IntentCashIn || journal.Lines[0].DebitCents != 500000 || journal.Lines[1].CreditCents != 500000 {
		t.Fatalf("unexpected journal: %+v", journal)
	}
	if err := BalanceCheck(journal.Lines); err != nil {
		t.Fatal(err)
	}
}

func TestCashOut(t *testing.T) {
	journal, err := CashOut(CashIntent{
		TenantID:       1,
		SourceRef:      "KK-1",
		EntryDate:      "2026-08-06",
		CashAccount:    testAccount(1101, AccountCash),
		CounterAccount: testAccount(5202, AccountExpense),
		AmountCents:    200000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if journal.Lines[0].DebitCents != 200000 || journal.Lines[1].CreditCents != 200000 {
		t.Fatalf("unexpected journal: %+v", journal)
	}
}

func TestTransferDoesNotAcceptNonCashAccount(t *testing.T) {
	_, err := Transfer(TransferIntent{
		FromAccount: testAccount(1101, AccountCash),
		ToAccount:   testAccount(4101, AccountRevenue),
		AmountCents: 100,
	})
	if !errors.Is(err, ErrAccountTypeMismatch) {
		t.Fatalf("expected account type mismatch, got %v", err)
	}
}

func TestOpeningBalanceUsesEquityPlug(t *testing.T) {
	journal, err := OpeningBalance(OpeningIntent{
		TenantID:      1,
		SourceRef:     "OPEN-1",
		EntryDate:     "2026-08-01",
		EquityAccount: testAccount(3101, AccountEquity),
		Balances: []OpeningBalanceLine{
			{AccountID: 1101, DebitCents: 2000000},
			{AccountID: 2101, CreditCents: 500000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := journal.Lines[len(journal.Lines)-1].CreditCents; got != 1500000 {
		t.Fatalf("expected equity plug 1500000, got %d", got)
	}
}

func TestReverse(t *testing.T) {
	original, err := CashIn(CashIntent{
		TenantID:       1,
		SourceRef:      "BK-1",
		EntryDate:      "2026-08-06",
		CashAccount:    testAccount(1101, AccountCash),
		CounterAccount: testAccount(4101, AccountRevenue),
		AmountCents:    500000,
		Description:    "Penjualan",
	})
	if err != nil {
		t.Fatal(err)
	}
	reversal, err := Reverse(original, "REV-1", "2026-08-07")
	if err != nil {
		t.Fatal(err)
	}
	if reversal.Lines[0].DebitCents != original.Lines[0].CreditCents || reversal.Lines[0].CreditCents != original.Lines[0].DebitCents {
		t.Fatalf("reversal does not invert original: original=%+v reversal=%+v", original.Lines, reversal.Lines)
	}
}

func TestHashIsDeterministic(t *testing.T) {
	intent := CashIntent{
		TenantID:       1,
		SourceRef:      "BK-1",
		EntryDate:      "2026-08-06",
		CashAccount:    testAccount(1101, AccountCash),
		CounterAccount: testAccount(4101, AccountRevenue),
		AmountCents:    500000,
	}
	first, err := CashIn(intent)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CashIn(intent)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, got %q and %q", first.Hash, second.Hash)
	}
}

func TestBalanceCheckRejectsUnbalancedLines(t *testing.T) {
	err := BalanceCheck([]Line{{AccountID: 1, DebitCents: 100}})
	if !errors.Is(err, ErrNotBalanced) {
		t.Fatalf("expected NOT_BALANCED, got %v", err)
	}
}
