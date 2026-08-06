package accounting

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
)

type IntentType string

const (
	IntentCashIn   IntentType = "CASH_IN"
	IntentCashOut  IntentType = "CASH_OUT"
	IntentTransfer IntentType = "TRANSFER"
	IntentOpening  IntentType = "OPENING_BALANCE"
	IntentReversal IntentType = "REVERSAL"
)

type AccountType string

const (
	AccountCash            AccountType = "CASH"
	AccountBank            AccountType = "BANK"
	AccountRevenue         AccountType = "REVENUE"
	AccountExpense         AccountType = "EXPENSE"
	AccountEquity          AccountType = "EQUITY"
	AccountOtherReceivable AccountType = "OTHER_RECEIVABLE"
)

type Account struct {
	ID          int64
	ReportGroup string
	Type        AccountType
	IsGroup     bool
	IsActive    bool
}

type Line struct {
	AccountID     int64
	DebitCents    int64
	CreditCents   int64
	SourceLineRef string
}

type Journal struct {
	TenantID     int64
	SourceRef    string
	IntentType   IntentType
	EntryDate    string
	Description  string
	Lines        []Line
	PreviousHash string
	Hash         string
}

type CashIntent struct {
	TenantID       int64
	SourceRef      string
	EntryDate      string
	CashAccount    Account
	CounterAccount Account
	AmountCents    int64
	Description    string
}

type TransferIntent struct {
	TenantID    int64
	SourceRef   string
	EntryDate   string
	FromAccount Account
	ToAccount   Account
	AmountCents int64
	Description string
}

type OpeningBalanceLine struct {
	AccountID   int64
	DebitCents  int64
	CreditCents int64
}

type OpeningIntent struct {
	TenantID      int64
	SourceRef     string
	EntryDate     string
	Balances      []OpeningBalanceLine
	EquityAccount Account
	Description   string
}

var (
	ErrNotBalanced         = errors.New("NOT_BALANCED")
	ErrInvalidAmount       = errors.New("INVALID_AMOUNT")
	ErrAccountNotPostable  = errors.New("ACCOUNT_NOT_POSTABLE")
	ErrAccountTypeMismatch = errors.New("ACCOUNT_TYPE_MISMATCH")
	ErrSameTransferAccount = errors.New("SAME_TRANSFER_ACCOUNT")
	ErrNoLines             = errors.New("NO_JOURNAL_LINES")
	ErrInvalidOpening      = errors.New("INVALID_OPENING_BALANCE")
)

func CashIn(intent CashIntent) (Journal, error) {
	if err := validateAmount(intent.AmountCents); err != nil {
		return Journal{}, err
	}
	if err := validatePostable(intent.CashAccount); err != nil {
		return Journal{}, err
	}
	if err := validatePostable(intent.CounterAccount); err != nil {
		return Journal{}, err
	}
	journal := Journal{
		TenantID:    intent.TenantID,
		SourceRef:   intent.SourceRef,
		IntentType:  IntentCashIn,
		EntryDate:   intent.EntryDate,
		Description: intent.Description,
		Lines: []Line{
			{AccountID: intent.CashAccount.ID, DebitCents: intent.AmountCents, SourceLineRef: "cash"},
			{AccountID: intent.CounterAccount.ID, CreditCents: intent.AmountCents, SourceLineRef: "counter"},
		},
	}
	return finalize(journal)
}

func CashOut(intent CashIntent) (Journal, error) {
	if err := validateAmount(intent.AmountCents); err != nil {
		return Journal{}, err
	}
	if err := validatePostable(intent.CashAccount); err != nil {
		return Journal{}, err
	}
	if err := validatePostable(intent.CounterAccount); err != nil {
		return Journal{}, err
	}
	journal := Journal{
		TenantID:    intent.TenantID,
		SourceRef:   intent.SourceRef,
		IntentType:  IntentCashOut,
		EntryDate:   intent.EntryDate,
		Description: intent.Description,
		Lines: []Line{
			{AccountID: intent.CounterAccount.ID, DebitCents: intent.AmountCents, SourceLineRef: "counter"},
			{AccountID: intent.CashAccount.ID, CreditCents: intent.AmountCents, SourceLineRef: "cash"},
		},
	}
	return finalize(journal)
}

func Transfer(intent TransferIntent) (Journal, error) {
	if err := validateAmount(intent.AmountCents); err != nil {
		return Journal{}, err
	}
	if intent.FromAccount.ID == intent.ToAccount.ID {
		return Journal{}, ErrSameTransferAccount
	}
	if !isCashOrBank(intent.FromAccount.Type) || !isCashOrBank(intent.ToAccount.Type) {
		return Journal{}, ErrAccountTypeMismatch
	}
	if err := validatePostable(intent.FromAccount); err != nil {
		return Journal{}, err
	}
	if err := validatePostable(intent.ToAccount); err != nil {
		return Journal{}, err
	}
	journal := Journal{
		TenantID:    intent.TenantID,
		SourceRef:   intent.SourceRef,
		IntentType:  IntentTransfer,
		EntryDate:   intent.EntryDate,
		Description: intent.Description,
		Lines: []Line{
			{AccountID: intent.ToAccount.ID, DebitCents: intent.AmountCents, SourceLineRef: "to"},
			{AccountID: intent.FromAccount.ID, CreditCents: intent.AmountCents, SourceLineRef: "from"},
		},
	}
	return finalize(journal)
}

func OpeningBalance(intent OpeningIntent) (Journal, error) {
	if len(intent.Balances) == 0 {
		return Journal{}, ErrInvalidOpening
	}
	if err := validatePostable(intent.EquityAccount); err != nil {
		return Journal{}, err
	}
	lines := make([]Line, 0, len(intent.Balances)+1)
	debitTotal := int64(0)
	creditTotal := int64(0)
	for _, balance := range intent.Balances {
		if balance.AccountID <= 0 || balance.DebitCents < 0 || balance.CreditCents < 0 || (balance.DebitCents > 0 && balance.CreditCents > 0) {
			return Journal{}, ErrInvalidOpening
		}
		if balance.DebitCents == 0 && balance.CreditCents == 0 {
			continue
		}
		lines = append(lines, Line{AccountID: balance.AccountID, DebitCents: balance.DebitCents, CreditCents: balance.CreditCents, SourceLineRef: fmt.Sprintf("account-%d", balance.AccountID)})
		debitTotal += balance.DebitCents
		creditTotal += balance.CreditCents
	}
	plug := debitTotal - creditTotal
	if plug > 0 {
		lines = append(lines, Line{AccountID: intent.EquityAccount.ID, CreditCents: plug, SourceLineRef: "equity-plug"})
	} else if plug < 0 {
		lines = append(lines, Line{AccountID: intent.EquityAccount.ID, DebitCents: -plug, SourceLineRef: "equity-plug"})
	}
	journal := Journal{TenantID: intent.TenantID, SourceRef: intent.SourceRef, IntentType: IntentOpening, EntryDate: intent.EntryDate, Description: intent.Description, Lines: lines}
	return finalize(journal)
}

func Reverse(original Journal, sourceRef string, entryDate string) (Journal, error) {
	if err := BalanceCheck(original.Lines); err != nil {
		return Journal{}, err
	}
	if sourceRef == "" || entryDate == "" {
		return Journal{}, ErrInvalidOpening
	}
	lines := make([]Line, len(original.Lines))
	for index, line := range original.Lines {
		lines[index] = Line{AccountID: line.AccountID, DebitCents: line.CreditCents, CreditCents: line.DebitCents, SourceLineRef: line.SourceLineRef}
	}
	journal := Journal{TenantID: original.TenantID, SourceRef: sourceRef, IntentType: IntentReversal, EntryDate: entryDate, Description: "Reversal: " + original.Description, Lines: lines}
	return finalize(journal)
}

func BalanceCheck(lines []Line) error {
	if len(lines) == 0 {
		return ErrNoLines
	}
	debitTotal := int64(0)
	creditTotal := int64(0)
	for _, line := range lines {
		if line.AccountID <= 0 || line.DebitCents < 0 || line.CreditCents < 0 || (line.DebitCents > 0 && line.CreditCents > 0) || line.DebitCents+line.CreditCents == 0 {
			return ErrNotBalanced
		}
		debitTotal += line.DebitCents
		creditTotal += line.CreditCents
	}
	if debitTotal != creditTotal {
		return ErrNotBalanced
	}
	return nil
}

func finalize(journal Journal) (Journal, error) {
	if err := BalanceCheck(journal.Lines); err != nil {
		return Journal{}, err
	}
	journal.PreviousHash = "genesis"
	journal.Hash = hashJournal(journal)
	return journal, nil
}

func validateAmount(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

func validatePostable(account Account) error {
	if account.ID <= 0 || account.IsGroup || !account.IsActive {
		return ErrAccountNotPostable
	}
	return nil
}

func isCashOrBank(accountType AccountType) bool {
	return accountType == AccountCash || accountType == AccountBank
}

func hashJournal(journal Journal) string {
	lines := append([]Line(nil), journal.Lines...)
	sort.Slice(lines, func(left, right int) bool { return lines[left].SourceLineRef < lines[right].SourceLineRef })
	payload := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v", journal.TenantID, journal.SourceRef, journal.IntentType, journal.EntryDate, journal.PreviousHash, lines)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
