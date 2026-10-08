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
			if tt.wantMsg == "" {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Equal(t, got.Message, tt.wantMsg)
		})
	}
}

func TestCompileMentionsRE2(t *testing.T) {
	t.Parallel()

	_, err := Compile(`(?=x)`, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RE2")
}
