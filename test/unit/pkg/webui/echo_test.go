package webui_test

import (
	"testing"

	"github.com/Olian04/webui/pkg/webui"
)

// The facade must behave exactly like the domain it wraps; only the types differ.
func TestEcho(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims surrounding space", in: " hello ", want: "hello"},
		{name: "leaves inner space", in: "hello world", want: "hello world"},
		{name: "empty stays empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webui.Echo(tt.in); got != tt.want {
				t.Fatalf("Echo(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
