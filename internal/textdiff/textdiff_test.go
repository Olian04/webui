package textdiff

import (
	"strings"
	"testing"
)

func render(hunks []Hunk) string {
	var b strings.Builder
	for i, h := range hunks {
		if i > 0 {
			b.WriteString("@@\n")
		}
		for _, l := range h.Lines {
			b.WriteString(map[Kind]string{Equal: " ", Delete: "-", Insert: "+"}[l.Kind] + l.Text + "\n")
		}
	}
	return b.String()
}

func TestAChangeIsShownWithItsContext(t *testing.T) {
	t.Parallel()

	a := "one\ntwo\nthree\nfour\nfive\nsix\nseven\n"
	b := "one\ntwo\nthree\nFOUR\nfive\nsix\nseven\n"
	hunks, added, removed, ok := Unified(a, b, 2)
	if !ok || added != 1 || removed != 1 {
		t.Fatalf("ok=%v added=%d removed=%d", ok, added, removed)
	}
	want := " two\n three\n-four\n+FOUR\n five\n six\n"
	if got := render(hunks); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLineNumbersAreThoseOfEachText(t *testing.T) {
	t.Parallel()

	hunks, _, _, _ := Unified("a\nb\nc\n", "a\nx\nb\nc\n", 1)
	var inserted Line
	for _, l := range hunks[0].Lines {
		if l.Kind == Insert {
			inserted = l
		}
	}
	if inserted.New != 2 || inserted.Old != 0 || inserted.Text != "x" {
		t.Errorf("inserted = %+v", inserted)
	}
	for _, l := range hunks[0].Lines {
		if l.Kind == Equal && l.Text == "b" && (l.Old != 2 || l.New != 3) {
			t.Errorf("b = %+v, want old 2 and new 3", l)
		}
	}
}

func TestChangesFarApartAreSeparateHunksAndNearOnesAreOne(t *testing.T) {
	t.Parallel()

	var a, b []string
	for i := range 30 {
		a = append(a, strings.Repeat("l", i+1))
		b = append(b, strings.Repeat("l", i+1))
	}
	b[2], b[25] = "CHANGED", "CHANGED"
	hunks, _, _, _ := Unified(strings.Join(a, "\n"), strings.Join(b, "\n"), 3)
	if len(hunks) != 2 {
		t.Errorf("%d hunks, want 2 for changes 23 lines apart", len(hunks))
	}
	b[2], b[25] = a[2], a[25]
	b[5], b[9] = "X", "Y"
	hunks, _, _, _ = Unified(strings.Join(a, "\n"), strings.Join(b, "\n"), 3)
	if len(hunks) != 1 {
		t.Errorf("%d hunks, want 1 for changes closer than twice the context", len(hunks))
	}
}

func TestEqualTextsHaveNoHunksAndEmptyAgainstTextIsAllAdded(t *testing.T) {
	t.Parallel()

	if hunks, added, removed, ok := Unified("same\n", "same\n", 3); !ok || len(hunks) != 0 || added+removed != 0 {
		t.Errorf("equal texts: %v %d %d %v", hunks, added, removed, ok)
	}
	hunks, added, removed, ok := Unified("", "a\nb\n", 3)
	if !ok || added != 2 || removed != 0 || len(hunks) != 1 {
		t.Errorf("from nothing: hunks=%d added=%d removed=%d ok=%v", len(hunks), added, removed, ok)
	}
	hunks, added, removed, ok = Unified("a\nb\n", "", 3)
	if !ok || added != 0 || removed != 2 || len(hunks) != 1 {
		t.Errorf("to nothing: hunks=%d added=%d removed=%d ok=%v", len(hunks), added, removed, ok)
	}
}

func TestTextsTooDifferentOrTooLargeAreRefused(t *testing.T) {
	t.Parallel()

	var a, b strings.Builder
	for i := range 3000 {
		a.WriteString("a" + string(rune('a'+i%26)) + "\n")
		b.WriteString("b" + string(rune('a'+i%26)) + "\n")
	}
	if _, _, _, ok := Unified(a.String(), b.String(), 3); ok {
		t.Error("3000 lines that share nothing were compared")
	}
	if _, _, _, ok := Unified(strings.Repeat("x\n", maxLines+1), "", 3); ok {
		t.Error("a text of more than the maximum lines was compared")
	}
}

func TestTheScriptReplaysToTheSecondText(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"a\nb\nc\nd\n", "a\nc\nd\ne\n"},
		{"x\ny\nz\n", "z\ny\nx\n"},
		{"1\n2\n3\n4\n5\n6\n7\n8\n", "1\n2\nA\nB\n5\n6\n7\n8\n9\n"},
		{"same\nsame\nsame\n", "same\nsame\n"},
	}
	for _, c := range cases {
		script, ok := diff(lines(c[0]), lines(c[1]))
		if !ok {
			t.Fatalf("refused %q → %q", c[0], c[1])
		}
		var out []string
		for _, l := range script {
			if l.Kind != Delete {
				out = append(out, l.Text)
			}
		}
		if got := strings.Join(out, "\n") + "\n"; got != c[1] {
			t.Errorf("%q → %q replays as %q", c[0], c[1], got)
		}
	}
}

func TestRandomTextsReplayAndNumberTheirLinesAsTheyAre(t *testing.T) {
	t.Parallel()

	seed := uint64(7)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	text := func() []string {
		out := make([]string, next(12))
		for i := range out {
			out[i] = string([]byte{"abcd"[next(4)]})
		}
		return out
	}
	for range 400 {
		x, y := text(), text()
		script, ok := diff(x, y)
		if !ok {
			t.Fatalf("refused %v → %v", x, y)
		}
		var replayed []string
		for _, l := range script {
			switch l.Kind {
			case Equal:
				if l.Old < 1 || l.New < 1 || x[l.Old-1] != l.Text || y[l.New-1] != l.Text {
					t.Fatalf("equal line %+v is not in both of %v and %v", l, x, y)
				}
				replayed = append(replayed, l.Text)
			case Insert:
				if y[l.New-1] != l.Text {
					t.Fatalf("inserted %+v is not in %v", l, y)
				}
				replayed = append(replayed, l.Text)
			case Delete:
				if x[l.Old-1] != l.Text {
					t.Fatalf("deleted %+v is not in %v", l, x)
				}
			}
		}
		if strings.Join(replayed, "|") != strings.Join(y, "|") {
			t.Fatalf("%v → %v replays as %v", x, y, replayed)
		}
	}
}
