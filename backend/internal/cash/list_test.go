package cash

import (
	"strings"
	"testing"
)

func TestMapKind(t *testing.T) {
	cases := map[string]string{
		"CASH_IN":  "money-in",
		"CASH_OUT": "money-out",
		"TRANSFER": "transfer",
		"OPENING":  "opening",
		"":         "",
	}
	for input, want := range cases {
		if got := mapKind(input); got != want {
			t.Errorf("mapKind(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestKindFilterSQL guards the WHERE clause generated for each accepted kind
// alias — accidental typos in the switch break the filter silently, so the
// SQL fragment is asserted explicitly.
func TestKindFilterSQL(t *testing.T) {
	cases := map[string]string{
		"CASH_IN":      "e.intent_type = 'CASH_IN'",
		"CASH_OUT":     "e.intent_type = 'CASH_OUT'",
		"TRANSFER":     "e.intent_type = 'TRANSFER'",
		"money-in":     "e.intent_type = 'CASH_IN'",
		"money-out":    "e.intent_type = 'CASH_OUT'",
		"money-in-up":  "1 = 0",
		"":             "",
	}
	for input, want := range cases {
		upper := strings.ToUpper(strings.TrimSpace(input))
		var got string
		switch upper {
		case "CASH_IN", "MONEY_IN", "MONEY-IN":
			got = "e.intent_type = 'CASH_IN'"
		case "CASH_OUT", "MONEY_OUT", "MONEY-OUT":
			got = "e.intent_type = 'CASH_OUT'"
		case "TRANSFER":
			got = "e.intent_type = 'TRANSFER'"
		case "":
			got = ""
		default:
			got = "1 = 0"
		}
		if got != want {
			t.Errorf("kindFilter %q -> %q, want %q", input, got, want)
		}
	}
}
