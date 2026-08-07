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
	TenantID    int64
	SourceRef   string
	EntryDate   string
	CashAccount Account
	// CounterAccount is the single-counter fallback. When CounterLines is
	// non-empty it overrides CounterAccount and the cash side distributes
	// across the lines.
	CounterAccount Account
	AmountCents    int64
	Description    string
	// CounterLines splits the counter side across multiple accounts. When
	// provided, the sum of AmountCents across lines must equal AmountCents
	// and CounterAccount is ignored.
	CounterLines []CounterLine
}

// CounterLine is one line on the counter side of a CASH_IN / CASH_OUT
// journal. Multiple lines let a single transaction touch several accounts
// (e.g. a receipt split across two revenue accounts).
type CounterLine struct {
	Account     Account
	AmountCents int64
	Description string
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
	lines, err := buildCounterCreditLines(intent)
	if err != nil {
		return Journal{}, err
	}
	journal := Journal{
		TenantID:    intent.TenantID,
		SourceRef:   intent.SourceRef,
		IntentType:  IntentCashIn,
		EntryDate:   intent.EntryDate,
		Description: intent.Description,
		Lines: append([]Line{
			{AccountID: intent.CashAccount.ID, DebitCents: intent.AmountCents, SourceLineRef: "cash"},
		}, lines...),
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
	lines, err := buildCounterCreditLines(intent)
	if err != nil {
		return Journal{}, err
	}
	journal := Journal{
		TenantID:    intent.TenantID,
		SourceRef:   intent.SourceRef,
		IntentType:  IntentCashOut,
		EntryDate:   intent.EntryDate,
		Description: intent.Description,
		Lines: append([]Line{
			{AccountID: intent.CashAccount.ID, CreditCents: intent.AmountCents, SourceLineRef: "cash"},
		}, invertLines(lines)...),
	}
	return finalize(journal)
}

// buildCounterCreditLines returns the credit-side lines of a CashIn
// intent. When CounterLines is provided the lines must be valid postable
// accounts, must each carry a positive amount, and the sum must equal the
// intent amount. When CounterLines is empty, the single CounterAccount
// is used.
func buildCounterCreditLines(intent CashIntent) ([]Line, error) {
	if len(intent.CounterLines) == 0 {
		if err := validatePostable(intent.CounterAccount); err != nil {
			return nil, err
		}
		return []Line{
			{AccountID: intent.CounterAccount.ID, CreditCents: intent.AmountCents, SourceLineRef: "counter"},
		}, nil
	}
	lines := make([]Line, 0, len(intent.CounterLines))
	total := int64(0)
	for index, cl := range intent.CounterLines {
		if err := validatePostable(cl.Account); err != nil {
			return nil, err
		}
		if cl.AmountCents <= 0 {
			return nil, ErrInvalidAmount
		}
		total += cl.AmountCents
		lines = append(lines, Line{
			AccountID:     cl.Account.ID,
			CreditCents:   cl.AmountCents,
			SourceLineRef: fmt.Sprintf("counter-%d", index),
		})
	}
	if total != intent.AmountCents {
		return nil, ErrNotBalanced
	}
	return lines, nil
}

// invertLines flips debit and credit amounts (used to swap the cash side
// of a CashIn line list into a CashOut line list).
func invertLines(lines []Line) []Line {
	out := make([]Line, len(lines))
	for i, line := range lines {
		out[i] = Line{
			AccountID:     line.AccountID,
			DebitCents:    line.CreditCents,
			CreditCents:   line.DebitCents,
			SourceLineRef: line.SourceLineRef,
		}
	}
	return out
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
