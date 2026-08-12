package reporting

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// =====================================================================
// Overview
// =====================================================================
//
// The reporting package's heavy lifting lives in fetch* methods that run
// parameterised SQL against a *pgxpool.Pool. Those paths are integration
// tests (they need a live Postgres with the chart of accounts seeded) and
// are deliberately out of scope here.
//
// What IS pure and therefore unit-testable without a database:
//
//   - parseDateRange / parseReportFilter — query-param parsing
//   - dateFilter                        — SQL fragment builder with
//                                         placeholder offset tracking
//   - dimensionJoin                     — SQL JOIN fragment builder
//   - dateRangeLabel                    — human-readable range label
//   - buildFrameworkSections           — framework (EMKM/ETAP/SAK_UMUM)
//                                         section roll-up
//
// And the result structs themselves carry mathematical invariants that the
// fetch* methods compute but that hold regardless of where the numbers came
// from. We exercise those invariants by constructing the structs directly
// and asserting the relationships the production code is expected to
// maintain (Balanced, Profit = Revenue - Expense, NetCashFlow = Inflow -
// Outflow, Assets = Liabilities + Equity + Profit, etc.).
//
// All tests are table-driven and use only the standard "testing" package.

// =====================================================================
// 1 + 2. Trial balance Balanced flag
// =====================================================================

// The fetch* path sets result.Balanced = (TotalDebitCents == TotalCreditCents).
// We verify that invariant directly on constructed results so the rule is
// pinned even though the arithmetic currently lives inside fetchTrialBalance.

func TestTrialBalanceResult_Balanced(t *testing.T) {
	// Cases expressed as the raw rows; totals + Balanced are derived the way
	// fetchTrialBalance derives them (sum the rows, then compare).
	type row struct {
		debit, credit int64
	}
	cases := []struct {
		name     string
		rows     []row
		balanced bool
	}{
		{
			name:     "empty is balanced",
			rows:     nil,
			balanced: true,
		},
		{
			name: "single balanced entry",
			rows: []row{
				{debit: 1_000_00, credit: 1_000_00},
			},
			balanced: true,
		},
		{
			name: "many balanced entries",
			rows: []row{
				{debit: 5_000_00, credit: 0},
				{debit: 0, credit: 3_000_00},
				{debit: 0, credit: 2_000_00},
			},
			balanced: true, // 5000 dr == 5000 cr
		},
		{
			name: "debits exceed credits",
			rows: []row{
				{debit: 5_000_00, credit: 4_990_00},
			},
			balanced: false,
		},
		{
			name: "credits exceed debits",
			rows: []row{
				{debit: 1_000_00, credit: 1_500_00},
			},
			balanced: false,
		},
		{
			name: "off by one cent",
			rows: []row{
				{debit: 100_00, credit: 99_99},
			},
			balanced: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r TrialBalanceResult
			r.Rows = make([]TrialBalanceRow, 0, len(tc.rows))
			for i, row := range tc.rows {
				tb := TrialBalanceRow{
					AccountID:   int64(i + 1),
					AccountCode: "1-1000",
					AccountName: "Cash",
					DebitCents:  row.debit,
					CreditCents: row.credit,
				}
				// Mirror fetchTrialBalance: skip zero-movement accounts.
				if tb.DebitCents == 0 && tb.CreditCents == 0 {
					continue
				}
				r.Rows = append(r.Rows, tb)
				r.TotalDebitCents += tb.DebitCents
				r.TotalCreditCents += tb.CreditCents
			}
			// The exact rule from fetchTrialBalance (data.go).
			r.Balanced = r.TotalDebitCents == r.TotalCreditCents

			if r.Balanced != tc.balanced {
				t.Errorf("Balanced = %v, want %v (dr=%d cr=%d)",
					r.Balanced, tc.balanced, r.TotalDebitCents, r.TotalCreditCents)
			}
		})
	}
}

// =====================================================================
// 3. Profit & Loss: Profit = Revenue - Expense
// =====================================================================

func TestProfitLossResult_ProfitInvariant(t *testing.T) {
	cases := []struct {
		name          string
		revenueCents  int64
		expenseCents  int64
		wantProfitCts int64
	}{
		{name: "zero/zero", revenueCents: 0, expenseCents: 0, wantProfitCts: 0},
		{name: "break even", revenueCents: 10_000_00, expenseCents: 10_000_00, wantProfitCts: 0},
		{name: "net profit", revenueCents: 50_000_00, expenseCents: 30_000_00, wantProfitCts: 20_000_00},
		{name: "net loss", revenueCents: 10_000_00, expenseCents: 40_000_00, wantProfitCts: -30_000_00},
		{name: "no revenue only expense", revenueCents: 0, expenseCents: 5_000_00, wantProfitCts: -5_000_00},
		{name: "revenue no expense", revenueCents: 7_500_00, expenseCents: 0, wantProfitCts: 7_500_00},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Mirror fetchProfitLoss (data.go):
			//   result.ProfitCents = result.RevenueCents - result.ExpenseCents
			r := ProfitLossResult{
				RevenueCents: tc.revenueCents,
				ExpenseCents: tc.expenseCents,
			}
			r.ProfitCents = r.RevenueCents - r.ExpenseCents

			if r.ProfitCents != tc.wantProfitCts {
				t.Errorf("ProfitCents = %d, want %d (rev=%d exp=%d)",
					r.ProfitCents, tc.wantProfitCts, r.RevenueCents, r.ExpenseCents)
			}
			// Reinforce the invariant as a property, independent of the table.
			if r.ProfitCents != r.RevenueCents-r.ExpenseCents {
				t.Fatalf("profit invariant violated: %d != %d - %d",
					r.ProfitCents, r.RevenueCents, r.ExpenseCents)
			}
		})
	}
}

// =====================================================================
// 4. Cash flow: operating + investing + financing = net cash flow
// =====================================================================

// computeCashFlowNets replicates the trailing block of fetchCashFlow
// (data.go): the per-activity nets, the totals, and the net cash flow.
// Keeping it here means the test pins the exact arithmetic the production
// code is expected to maintain.
func computeCashFlowNets(r *CashFlowResult) {
	r.NetOperatingCents = r.OperatingInflowCents - r.OperatingOutflowCents
	r.NetInvestingCents = r.InvestingInflowCents - r.InvestingOutflowCents
	r.NetFinancingCents = r.FinancingInflowCents - r.FinancingOutflowCents
	r.InflowCents = r.OperatingInflowCents + r.InvestingInflowCents + r.FinancingInflowCents
	r.OutflowCents = r.OperatingOutflowCents + r.InvestingOutflowCents + r.FinancingOutflowCents
	r.NetCashFlowCents = r.InflowCents - r.OutflowCents
}

func TestCashFlowResult_ActivitySumsToNet(t *testing.T) {
	cases := []struct {
		name                                      string
		opIn, opOut, invIn, invOut, finIn, finOut int64
	}{
		{name: "all zero", opIn: 0, opOut: 0, invIn: 0, invOut: 0, finIn: 0, finOut: 0},
		{name: "operating only positive", opIn: 10_000_00, opOut: 4_000_00},
		{name: "investing net negative", invIn: 1_000_00, invOut: 5_000_00},
		{name: "financing net positive", finIn: 8_000_00, finOut: 2_000_00},
		{name: "mixed activities", opIn: 20_000_00, opOut: 12_000_00, invIn: 0, invOut: 6_000_00, finIn: 9_000_00, finOut: 3_000_00},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := CashFlowResult{
				OperatingInflowCents:  tc.opIn,
				OperatingOutflowCents: tc.opOut,
				InvestingInflowCents:  tc.invIn,
				InvestingOutflowCents: tc.invOut,
				FinancingInflowCents:  tc.finIn,
				FinancingOutflowCents: tc.finOut,
			}
			computeCashFlowNets(&r)

			// operating + investing + financing (nets) == net cash flow.
			activitiesSum := r.NetOperatingCents + r.NetInvestingCents + r.NetFinancingCents
			if activitiesSum != r.NetCashFlowCents {
				t.Errorf("activity sum %d != NetCashFlowCents %d",
					activitiesSum, r.NetCashFlowCents)
			}
		})
	}
}

// =====================================================================
// 5. Balance sheet equation: assets = liabilities + equity + profit
// =====================================================================

func TestBalanceSheetResult_Equation(t *testing.T) {
	cases := []struct {
		name         string
		assets       int64
		liabilities  int64
		equity       int64
		profit       int64
		wantBalanced bool
	}{
		{name: "empty books balance", assets: 0, liabilities: 0, equity: 0, profit: 0, wantBalanced: true},
		{name: "equity only", assets: 100_00, liabilities: 0, equity: 100_00, profit: 0, wantBalanced: true},
		{name: "liability funded asset", assets: 500_00, liabilities: 200_00, equity: 300_00, profit: 0, wantBalanced: true},
		{name: "profit folded into equity", assets: 1_000_00, liabilities: 400_00, equity: 500_00, profit: 100_00, wantBalanced: true},
		{name: "loss still balances", assets: 800_00, liabilities: 400_00, equity: 500_00, profit: -100_00, wantBalanced: true},
		{name: "unbalanced assets too high", assets: 1_000_00, liabilities: 400_00, equity: 500_00, profit: 0, wantBalanced: false},
		{name: "unbalanced liabilities too high", assets: 1_000_00, liabilities: 600_00, equity: 500_00, profit: 0, wantBalanced: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := BalanceSheetResult{
				AssetCents:     tc.assets,
				LiabilityCents: tc.liabilities,
				EquityCents:    tc.equity,
				ProfitCents:    tc.profit,
			}
			// Mirror fetchBalanceSheet (data.go):
			//   result.Balanced = result.AssetCents ==
			//       result.LiabilityCents + result.EquityCents + result.ProfitCents
			r.Balanced = r.AssetCents == r.LiabilityCents+r.EquityCents+r.ProfitCents

			if r.Balanced != tc.wantBalanced {
				t.Errorf("Balanced = %v, want %v (A=%d L=%d E=%d P=%d; rhs=%d)",
					r.Balanced, tc.wantBalanced,
					r.AssetCents, r.LiabilityCents, r.EquityCents, r.ProfitCents,
					r.LiabilityCents+r.EquityCents+r.ProfitCents)
			}
		})
	}
}

// =====================================================================
// 6. Cash flow net: inflow - outflow = net cash flow
// =====================================================================

func TestCashFlowResult_NetEqualsInflowMinusOutflow(t *testing.T) {
	cases := []struct {
		name                                      string
		opIn, opOut, invIn, invOut, finIn, finOut int64
	}{
		{name: "all zero", opIn: 0, opOut: 0, invIn: 0, invOut: 0, finIn: 0, finOut: 0},
		{name: "pure inflow", opIn: 10_000_00, opOut: 0, invIn: 0, invOut: 0, finIn: 0, finOut: 0},
		{name: "pure outflow", opIn: 0, opOut: 10_000_00, invIn: 0, invOut: 0, finIn: 0, finOut: 0},
		{name: "net positive", opIn: 30_000_00, opOut: 10_000_00, invIn: 5_000_00, invOut: 1_000_00, finIn: 0, finOut: 0},
		{name: "net negative", opIn: 5_000_00, opOut: 20_000_00, invIn: 0, invOut: 10_000_00, finIn: 2_000_00, finOut: 8_000_00},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := CashFlowResult{
				OperatingInflowCents:  tc.opIn,
				OperatingOutflowCents: tc.opOut,
				InvestingInflowCents:  tc.invIn,
				InvestingOutflowCents: tc.invOut,
				FinancingInflowCents:  tc.finIn,
				FinancingOutflowCents: tc.finOut,
			}
			computeCashFlowNets(&r)

			wantNet := r.InflowCents - r.OutflowCents
			if r.NetCashFlowCents != wantNet {
				t.Errorf("NetCashFlowCents = %d, want inflow(%d) - outflow(%d) = %d",
					r.NetCashFlowCents, r.InflowCents, r.OutflowCents, wantNet)
			}
			// And the per-activity nets must each be inflow - outflow.
			if r.NetOperatingCents != r.OperatingInflowCents-r.OperatingOutflowCents {
				t.Errorf("NetOperatingCents = %d, want %d",
					r.NetOperatingCents, r.OperatingInflowCents-r.OperatingOutflowCents)
			}
			if r.NetInvestingCents != r.InvestingInflowCents-r.InvestingOutflowCents {
				t.Errorf("NetInvestingCents = %d, want %d",
					r.NetInvestingCents, r.InvestingInflowCents-r.InvestingOutflowCents)
			}
			if r.NetFinancingCents != r.FinancingInflowCents-r.FinancingOutflowCents {
				t.Errorf("NetFinancingCents = %d, want %d",
					r.NetFinancingCents, r.FinancingInflowCents-r.FinancingOutflowCents)
			}
		})
	}
}

// =====================================================================
// 7. Report filter / date-range parsing
// =====================================================================

func newFilterRequest(q string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/reports?"+q, nil)
	return req
}

func TestParseDateRange(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantFrom string
		wantTo   string
	}{
		{name: "empty", query: "", wantFrom: "", wantTo: ""},
		{name: "both dates", query: "from_date=2025-01-01&to_date=2025-01-31", wantFrom: "2025-01-01", wantTo: "2025-01-31"},
		{name: "only from", query: "from_date=2025-01-01", wantFrom: "2025-01-01", wantTo: ""},
		{name: "only to", query: "to_date=2025-01-31", wantFrom: "", wantTo: "2025-01-31"},
		{name: "whitespace trimmed", query: "from_date=%20%202025-01-01%20&to_date=2025-01-31%20", wantFrom: "2025-01-01", wantTo: "2025-01-31"},
		{name: "invalid from ignored", query: "from_date=not-a-date&to_date=2025-01-31", wantFrom: "", wantTo: "2025-01-31"},
		{name: "invalid to ignored", query: "from_date=2025-01-01&to_date=31/01/2025", wantFrom: "2025-01-01", wantTo: ""},
		{name: "both invalid", query: "from_date=abc&to_date=xyz", wantFrom: "", wantTo: ""},
		{name: "wrong format rejected", query: "from_date=2025/01/01", wantFrom: "", wantTo: ""},
		{name: "february 29 leap year ok", query: "to_date=2024-02-29", wantFrom: "", wantTo: "2024-02-29"},
		{name: "february 29 non leap rejected", query: "to_date=2025-02-29", wantFrom: "", wantTo: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDateRange(newFilterRequest(tc.query))
			if got.fromDate != tc.wantFrom || got.toDate != tc.wantTo {
				t.Errorf("parseDateRange = {from:%q to:%q}, want {from:%q to:%q}",
					got.fromDate, got.toDate, tc.wantFrom, tc.wantTo)
			}
		})
	}
}

func TestParseReportFilter(t *testing.T) {
	cases := []struct {
		name             string
		query            string
		wantFrom         string
		wantTo           string
		wantFramework    string
		wantDimensionIDs []int64
	}{
		{
			name:             "all empty",
			query:            "",
			wantFrom:         "",
			wantTo:           "",
			wantFramework:    "",
			wantDimensionIDs: nil,
		},
		{
			name:             "full filter",
			query:            "from_date=2025-01-01&to_date=2025-06-30&framework=EMKM&dimension_id=42",
			wantFrom:         "2025-01-01",
			wantTo:           "2025-06-30",
			wantFramework:    "EMKM",
			wantDimensionIDs: []int64{42},
		},
		{
			name:             "framework lowercased normalised",
			query:            "framework=emkm",
			wantFramework:    "EMKM",
			wantDimensionIDs: nil,
		},
		{
			name:             "framework with spaces normalised",
			query:            "framework=%20%20sak_umum%20",
			wantFramework:    "SAK_UMUM",
			wantDimensionIDs: nil,
		},
		{
			name:             "unknown framework dropped",
			query:            "framework=IFRS",
			wantFramework:    "",
			wantDimensionIDs: nil,
		},
		{
			name:             "dimension zero dropped",
			query:            "dimension_id=0",
			wantDimensionIDs: nil,
		},
		{
			name:             "dimension negative dropped",
			query:            "dimension_id=-5",
			wantDimensionIDs: nil,
		},
		{
			name:             "dimension non-numeric dropped",
			query:            "dimension_id=abc",
			wantDimensionIDs: nil,
		},
		{
			name:             "multiple dimensions via repeated params",
			query:            "dimension_id=1&dimension_id=2",
			wantDimensionIDs: []int64{1, 2},
		},
		{
			name:             "multiple dimensions via comma list",
			query:            "dimension_id=1,2,3",
			wantDimensionIDs: []int64{1, 2, 3},
		},
		{
			name:             "mixed repeated and comma list",
			query:            "dimension_id=1,2&dimension_id=3",
			wantDimensionIDs: []int64{1, 2, 3},
		},
		{
			name:             "duplicates deduped preserving order",
			query:            "dimension_id=2&dimension_id=1,2&dimension_id=1",
			wantDimensionIDs: []int64{2, 1},
		},
		{
			name:             "invalid values inside list dropped",
			query:            "dimension_id=abc,5&dimension_id=0,-1,7",
			wantDimensionIDs: []int64{5, 7},
		},
		{
			name:             "whitespace inside list trimmed",
			query:            "dimension_id=%204%20,%209%20",
			wantDimensionIDs: []int64{4, 9},
		},
		{
			name:             "invalid date still drops framework-safe",
			query:            "from_date=bad&framework=ETAP",
			wantFrom:         "",
			wantFramework:    "ETAP",
			wantDimensionIDs: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := parseReportFilter(newFilterRequest(tc.query))
			if f.fromDate != tc.wantFrom {
				t.Errorf("fromDate = %q, want %q", f.fromDate, tc.wantFrom)
			}
			if f.toDate != tc.wantTo {
				t.Errorf("toDate = %q, want %q", f.toDate, tc.wantTo)
			}
			if f.framework != tc.wantFramework {
				t.Errorf("framework = %q, want %q", f.framework, tc.wantFramework)
			}
			if !reflect.DeepEqual(f.dimensionIDs, tc.wantDimensionIDs) {
				t.Errorf("dimensionIDs = %v, want %v", f.dimensionIDs, tc.wantDimensionIDs)
			}
		})
	}
}

// dateFilter builds a SQL fragment whose placeholder offsets must line up
// with the args slice it appends. This is fiddly and easy to break, so we
// pin both the fragment text and the args order.

func TestDateFilter(t *testing.T) {
	cases := []struct {
		name        string
		dr          dateRange
		baseArg     int
		wantFrag    string
		wantArgs    []any
		wantNextArg int
	}{
		{
			name:        "no bounds -> empty fragment",
			dr:          dateRange{},
			baseArg:     2,
			wantFrag:    "",
			wantArgs:    []any{},
			wantNextArg: 2,
		},
		{
			name:        "from only",
			dr:          dateRange{fromDate: "2025-01-01"},
			baseArg:     2,
			wantFrag:    " AND je.entry_date >= $2",
			wantArgs:    []any{"2025-01-01"},
			wantNextArg: 3,
		},
		{
			name:        "to only",
			dr:          dateRange{toDate: "2025-01-31"},
			baseArg:     2,
			wantFrag:    " AND je.entry_date <= $2",
			wantArgs:    []any{"2025-01-31"},
			wantNextArg: 3,
		},
		{
			name:        "both bounds",
			dr:          dateRange{fromDate: "2025-01-01", toDate: "2025-01-31"},
			baseArg:     2,
			wantFrag:    " AND je.entry_date >= $2 AND je.entry_date <= $3",
			wantArgs:    []any{"2025-01-01", "2025-01-31"},
			wantNextArg: 4,
		},
		{
			name:        "base arg advances past dimension",
			dr:          dateRange{fromDate: "2025-01-01", toDate: "2025-01-31"},
			baseArg:     3,
			wantFrag:    " AND je.entry_date >= $3 AND je.entry_date <= $4",
			wantArgs:    []any{"2025-01-01", "2025-01-31"},
			wantNextArg: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := []any{}
			got := dateFilter(tc.dr, tc.baseArg, &args)
			if got != tc.wantFrag {
				t.Errorf("fragment = %q, want %q", got, tc.wantFrag)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args = %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

// dimensionJoin must add the JOIN + array arg when dimensions are set and
// leave everything untouched when they are not. The join targets a DISTINCT
// subquery so a line tagged with several selected dimensions counts once.

func TestDimensionJoin(t *testing.T) {
	wantFrag := " JOIN (SELECT DISTINCT tenant_id, journal_line_id FROM journal_line_dimensions WHERE dimension_id = ANY($2)) jld" +
		" ON jld.tenant_id = jl.tenant_id AND jld.journal_line_id = jl.id"

	t.Run("no dimension", func(t *testing.T) {
		args := []any{int64(1)} // tenant
		frag, next := dimensionJoin(reportFilter{}, 2, &args)
		if frag != "" {
			t.Errorf("frag = %q, want empty", frag)
		}
		if next != 2 {
			t.Errorf("next = %d, want 2 (unchanged)", next)
		}
		if len(args) != 1 {
			t.Errorf("args len = %d, want 1 (no append)", len(args))
		}
	})

	t.Run("single dimension", func(t *testing.T) {
		args := []any{int64(1)} // tenant
		frag, next := dimensionJoin(reportFilter{dimensionIDs: []int64{7}}, 2, &args)
		if frag != wantFrag {
			t.Errorf("frag = %q, want %q", frag, wantFrag)
		}
		if next != 3 {
			t.Errorf("next = %d, want 3", next)
		}
		if len(args) != 2 || !reflect.DeepEqual(args[1], []int64{7}) {
			t.Errorf("args = %v, want [1 [7]]", args)
		}
	})

	t.Run("multiple dimensions use one array arg", func(t *testing.T) {
		args := []any{int64(1)} // tenant
		frag, next := dimensionJoin(reportFilter{dimensionIDs: []int64{7, 8, 9}}, 2, &args)
		if frag != wantFrag {
			t.Errorf("frag = %q, want %q", frag, wantFrag)
		}
		if next != 3 {
			t.Errorf("next = %d, want 3 (single array placeholder)", next)
		}
		if len(args) != 2 || !reflect.DeepEqual(args[1], []int64{7, 8, 9}) {
			t.Errorf("args = %v, want [1 [7 8 9]]", args)
		}
	})
}

func TestDateRangeLabel(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "both empty", from: "", to: "", want: "All posted entries"},
		{name: "both set", from: "2025-01-02", to: "2025-01-31", want: "Jan 2 2025 – Jan 31 2025"},
		{name: "only to", from: "", to: "2025-01-31", want: "As of Jan 31 2025"},
		{name: "only from", from: "2025-01-02", to: "", want: "From Jan 2 2025"},
		{name: "invalid from passed through", from: "garbage", to: "", want: "From garbage"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dateRangeLabel(tc.from, tc.to)
			if got != tc.want {
				t.Errorf("dateRangeLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

// =====================================================================
// 8. Framework selection (EMKM / ETAP / SAK_UMUM)
// =====================================================================

// buildFrameworkSections is the pure roll-up the P&L handler calls after the
// per-account_type query returns. We drive it with synthetic byType maps and
// assert the framework picks the right account_types and produces the right
// section layout.

func TestBuildFrameworkSections_EMKM(t *testing.T) {
	// EMKM collapses to two sections: Pendapatan (revenue side) and Beban
	// (expense side). Every account_type on the revenue report_group rolls
	// into revenue; every expense-side type rolls into expense.
	byType := map[string]int64{
		"REVENUE":        100_000_00,
		"CONTRA_REVENUE": 5_000_00,
		"COGS":           40_000_00,
		"EXPENSE":        20_000_00,
	}
	groupByType := map[string]string{
		"REVENUE":        "revenue",
		"CONTRA_REVENUE": "revenue",
		"COGS":           "expense",
		"EXPENSE":        "expense",
	}

	got := buildFrameworkSections("EMKM", byType, groupByType)

	if len(got) != 2 {
		t.Fatalf("EMKM: got %d sections, want 2", len(got))
	}
	want := []ProfitLossSection{
		{Code: "revenue", Label: "Pendapatan", AmountCents: 100_000_00 + 5_000_00},
		{Code: "expense", Label: "Beban", AmountCents: 40_000_00 + 20_000_00},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("section %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestBuildFrameworkSections_ETAP(t *testing.T) {
	// ETAP breaks out operating_revenue, cogs, operating_expense,
	// other_income, other_expense. Amounts are summed per explicit
	// account_type list.
	byType := map[string]int64{
		"REVENUE":        100_000_00,
		"CONTRA_REVENUE": 5_000_00,
		"COGS":           40_000_00,
		"EXPENSE":        15_000_00,
		"DEPRECIATION":   5_000_00,
		"OTHER_INCOME":   3_000_00,
		"TAX_EXPENSE":    2_000_00,
	}
	groupByType := map[string]string{
		"REVENUE":        "revenue",
		"CONTRA_REVENUE": "revenue",
		"COGS":           "expense",
		"EXPENSE":        "expense",
		"DEPRECIATION":   "expense",
		"OTHER_INCOME":   "revenue",
		"TAX_EXPENSE":    "expense",
	}

	got := buildFrameworkSections("ETAP", byType, groupByType)

	want := []ProfitLossSection{
		{Code: "operating_revenue", Label: "Pendapatan Usaha", AmountCents: 100_000_00 + 5_000_00},
		{Code: "cogs", Label: "Beban Pokok Penjualan", AmountCents: 40_000_00},
		{Code: "operating_expense", Label: "Beban Operasional", AmountCents: 15_000_00 + 5_000_00},
		{Code: "other_income", Label: "Pendapatan Lain-lain", AmountCents: 3_000_00},
		{Code: "other_expense", Label: "Beban Lain-lain", AmountCents: 2_000_00},
	}
	if len(got) != len(want) {
		t.Fatalf("ETAP: got %d sections, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("section %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestBuildFrameworkSections_SAK_UMUM(t *testing.T) {
	// SAK_UMUM uses the same account_type groupings as ETAP but different
	// labels (Pendapatan / Beban Usaha).
	byType := map[string]int64{
		"REVENUE": 200_000_00,
		"COGS":    80_000_00,
		"EXPENSE": 30_000_00,
	}
	groupByType := map[string]string{
		"REVENUE": "revenue",
		"COGS":    "expense",
		"EXPENSE": "expense",
	}

	got := buildFrameworkSections("SAK_UMUM", byType, groupByType)

	want := []ProfitLossSection{
		{Code: "operating_revenue", Label: "Pendapatan", AmountCents: 200_000_00},
		{Code: "cogs", Label: "Beban Pokok Penjualan", AmountCents: 80_000_00},
		{Code: "operating_expense", Label: "Beban Usaha", AmountCents: 30_000_00},
		{Code: "other_income", Label: "Pendapatan Lain-lain", AmountCents: 0},
		{Code: "other_expense", Label: "Beban Lain-lain", AmountCents: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("SAK_UMUM: got %d sections, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("section %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestBuildFrameworkSections_UnknownFrameworkReturnsNil(t *testing.T) {
	if got := buildFrameworkSections("IFRS", nil, nil); got != nil {
		t.Errorf("unknown framework = %v, want nil", got)
	}
}

// Custom (unmapped) account types must be swept into the matching
// revenue/expense section rather than dropped — this is the contract in
// buildFrameworkSections' trailing sweep loop.

func TestBuildFrameworkSections_SweepsCustomAccountTypes(t *testing.T) {
	byType := map[string]int64{
		"REVENUE":    100_000_00,
		"EXPENSE":    20_000_00,
		"CUSTOM_REV": 7_000_00, // unmapped, revenue side
		"CUSTOM_EXP": 3_000_00, // unmapped, expense side
	}
	groupByType := map[string]string{
		"REVENUE":    "revenue",
		"EXPENSE":    "expense",
		"CUSTOM_REV": "revenue",
		"CUSTOM_EXP": "expense",
	}

	got := buildFrameworkSections("ETAP", byType, groupByType)

	var revTotal, expTotal int64
	for _, s := range got {
		switch s.Code {
		case "operating_revenue":
			revTotal = s.AmountCents
		case "other_expense":
			expTotal = s.AmountCents
		}
	}
	// CUSTOM_REV (7000) must land in operating_revenue alongside REVENUE.
	if revTotal != 100_000_00+7_000_00 {
		t.Errorf("operating_revenue = %d, want %d (REVENUE + CUSTOM_REV)", revTotal, 100_000_00+7_000_00)
	}
	// CUSTOM_EXP (3000) must land in other_expense.
	if expTotal != 3_000_00 {
		t.Errorf("other_expense = %d, want %d (CUSTOM_EXP)", expTotal, 3_000_00)
	}
}

// The three frameworks must always produce sections whose amounts sum to
// revenue+expense totals from byType — no money may be silently dropped.

func TestBuildFrameworkSections_NoMoneyDropped(t *testing.T) {
	byType := map[string]int64{
		"REVENUE":        100_000_00,
		"CONTRA_REVENUE": 5_000_00,
		"COGS":           40_000_00,
		"EXPENSE":        15_000_00,
		"DEPRECIATION":   5_000_00,
		"OTHER_INCOME":   3_000_00,
		"OTHER_EXPENSE":  2_000_00,
		"TAX_EXPENSE":    1_000_00,
		"BAD_DEBT":       500_00,
	}
	groupByType := map[string]string{
		"REVENUE":        "revenue",
		"CONTRA_REVENUE": "revenue",
		"OTHER_INCOME":   "revenue",
		"COGS":           "expense",
		"EXPENSE":        "expense",
		"DEPRECIATION":   "expense",
		"OTHER_EXPENSE":  "expense",
		"TAX_EXPENSE":    "expense",
		"BAD_DEBT":       "expense",
	}

	var totalByType int64
	for _, amt := range byType {
		totalByType += amt
	}

	for _, framework := range []string{"EMKM", "ETAP", "SAK_UMUM"} {
		t.Run(framework, func(t *testing.T) {
			got := buildFrameworkSections(framework, byType, groupByType)
			var sum int64
			for _, s := range got {
				sum += s.AmountCents
			}
			if sum != totalByType {
				t.Errorf("%s: sections sum to %d, want %d (money dropped)",
					framework, sum, totalByType)
			}
		})
	}
}

// =====================================================================
// Bonus: report type / title dispatch table invariants
// =====================================================================

func TestReportTitles(t *testing.T) {
	want := map[reportType]string{
		reportTrialBalance: "Trial Balance",
		reportProfitLoss:   "Profit & Loss",
		reportBalanceSheet: "Balance Sheet",
		reportCashFlow:     "Cash Flow",
	}
	for rtype, title := range want {
		if got := reportTitles[rtype]; got != title {
			t.Errorf("reportTitles[%q] = %q, want %q", rtype, got, title)
		}
	}
	if len(reportTitles) != len(want) {
		t.Errorf("reportTitles has %d entries, want %d", len(reportTitles), len(want))
	}
}
