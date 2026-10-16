package rules

import (
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

func ptr(f float64) *float64 { return &f }

func TestCheck(t *testing.T) {
	t.Parallel()

	ipv4, err := Compile(`^\d{1,3}(\.\d{1,3}){3}$`, "must be a valid IPv4 address")
	assert.NoError(t, err)

	tests := []struct {
		name    string
		rules   ir.Rules
		kind    ir.ValueKind
		raw     string
		wantMsg string
	}{
		{name: "empty optional passes", rules: ir.Rules{MinLen: 5}, raw: ""},
		{name: "required empty", rules: ir.Rules{Required: true}, raw: "  ", wantMsg: "This field is required."},
		{name: "too short", rules: ir.Rules{MinLen: 7}, raw: "1.2.3", wantMsg: "Must be at least 7 characters."},
		{name: "length counts runes", rules: ir.Rules{MaxLen: 2}, raw: "åäö", wantMsg: "Must be at most 2 characters."},
		{name: "pattern message verbatim", rules: ir.Rules{Pattern: ipv4}, raw: "nope", wantMsg: "must be a valid IPv4 address"},
		{name: "pattern ok", rules: ir.Rules{Pattern: ipv4}, raw: "10.0.0.1"},
		{name: "int not number", kind: ir.KindInt, raw: "1.5", wantMsg: "Must be a whole number."},
		{name: "float not number", kind: ir.KindFloat, raw: "abc", wantMsg: "Must be a number."},
		{name: "zero min is a bound", rules: ir.Rules{Min: ptr(0)}, kind: ir.KindInt, raw: "-1", wantMsg: "Must be at least 0."},
		{name: "max", rules: ir.Rules{Max: ptr(65535)}, kind: ir.KindInt, raw: "70000", wantMsg: "Must be at most 65535."},
		{name: "in range", rules: ir.Rules{Min: ptr(1), Max: ptr(3)}, kind: ir.KindFloat, raw: "2.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Check(tt.rules, tt.kind, tt.raw)
			assert.Equal(t, got, tt.wantMsg)
		})
	}
}

func TestCompileMentionsRE2(t *testing.T) {
	t.Parallel()

	_, err := Compile(`(?=x)`, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RE2")
}

// What the browser is told to say for a value it refuses is what the server says for the same value: one set of words.
func TestWhatTheBrowserSaysIsWhatTheServerSays(t *testing.T) {
	lowest, highest := 1.0, 100.0
	pattern, err := Compile(`\d+`, "Digits only.")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Compile(`[a-z]+`, "")
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		rules  ir.Rules
		kind   ir.ValueKind
		bad    map[string]string // a value that fails, by the way it fails
		absent []string          // ways that the field has no words for
	}{
		"a text": {
			rules:  ir.Rules{Required: true, MinLen: 3, MaxLen: 5, Pattern: pattern},
			kind:   ir.KindString,
			bad:    map[string]string{"valueMissing": "", "tooShort": "12", "tooLong": "123456", "patternMismatch": "abcd"},
			absent: []string{"rangeUnderflow", "badInput"},
		},
		"a pattern with no message of its own": {
			rules: ir.Rules{Pattern: plain},
			kind:  ir.KindString,
			bad:   map[string]string{"patternMismatch": "ABC"},
		},
		"a whole number": {
			rules:  ir.Rules{Min: &lowest, Max: &highest},
			kind:   ir.KindInt,
			bad:    map[string]string{"rangeUnderflow": "0", "rangeOverflow": "101", "badInput": "x", "stepMismatch": "1.5"},
			absent: []string{"tooShort", "patternMismatch", "valueMissing"},
		},
		"a number": {
			rules: ir.Rules{Required: true, Max: &highest},
			kind:  ir.KindFloat,
			bad:   map[string]string{"valueMissing": "", "rangeOverflow": "100.5", "badInput": "x"},
		},
	} {
		said := Validity(c.rules, c.kind)
		for reason, value := range c.bad {
			want := Check(c.rules, c.kind, value)
			if want == "" {
				t.Errorf("%s: %q does not fail %s", name, value, reason)
			}
			if said[reason] != want {
				t.Errorf("%s: %s: the browser is told %q, the server says %q", name, reason, said[reason], want)
			}
		}
		for _, reason := range c.absent {
			if _, ok := said[reason]; ok {
				t.Errorf("%s: %s has words but the field has no such constraint", name, reason)
			}
		}
	}
	if got := Validity(ir.Rules{}, ir.KindString); len(got) != 0 {
		t.Errorf("a field with no constraint is told %v", got)
	}
}
