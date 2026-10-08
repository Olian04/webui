package tablequery

import (
	"slices"
	"strconv"
	"testing"

	"github.com/Olian04/webui/internal/ir"
)

type row struct {
	name   string
	status string
	count  int
}

var cols = []ir.Field{
	{Key: "name", Label: "Name", Get: func(m any) string { return m.(row).name }},
	{
		Key: "status", Label: "Status", Options: []string{"healthy", "degraded"},
		Get: func(m any) string { return m.(row).status },
	},
	{
		Key: "count", Label: "Count", Kind: ir.KindInt,
		Get: func(m any) string { return strconv.Itoa(m.(row).count) },
		Num: func(m any) float64 { return float64(m.(row).count) },
	},
}

func rows() []any {
	return []any{
		row{"banana", "healthy", 9},
		row{"Apple", "degraded", 10},
		row{"cherry", "healthy", 2},
		row{"apple", "healthy", 100},
	}
}

func names(rs []any) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.(row).name)
	}
	return out
}

func f(x float64) *float64 { return &x }

func TestNoQueryReturnsEveryRowInItsOrder(t *testing.T) {
	got, total := Apply(rows(), cols, ir.Query{})
	if !slices.Equal(names(got), []string{"banana", "Apple", "cherry", "apple"}) || total != 4 {
		t.Fatalf("got %v of %d", names(got), total)
	}
}

func TestTextSortsWithoutRegardToCaseAndNumbersAsNumbers(t *testing.T) {
	byName, _ := Apply(rows(), cols, ir.Query{Sort: "name"})
	if want := []string{"Apple", "apple", "banana", "cherry"}; !slices.Equal(names(byName), want) {
		t.Fatalf("by name: %v, want %v", names(byName), want)
	}
	// 100 sorts after 10 and 9: as text it would sort before them.
	byCount, _ := Apply(rows(), cols, ir.Query{Sort: "count"})
	if want := []string{"cherry", "banana", "Apple", "apple"}; !slices.Equal(names(byCount), want) {
		t.Fatalf("by count: %v, want %v", names(byCount), want)
	}
	desc, _ := Apply(rows(), cols, ir.Query{Sort: "count", Desc: true})
	if want := []string{"apple", "Apple", "banana", "cherry"}; !slices.Equal(names(desc), want) {
		t.Fatalf("descending: %v, want %v", names(desc), want)
	}
}

func TestTheSortIsStableForEqualValues(t *testing.T) {
	got, _ := Apply(rows(), cols, ir.Query{Sort: "status"})
	// degraded first, then the healthy ones in the order they came.
	if want := []string{"Apple", "banana", "cherry", "apple"}; !slices.Equal(names(got), want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
}

func TestATextFilterContainsIgnoringCaseAndAnOptionFilterTakesAnyOfTheOptions(t *testing.T) {
	got, total := Apply(rows(), cols, ir.Query{Filters: map[string][]string{"name": {"APP"}}})
	if want := []string{"Apple", "apple"}; !slices.Equal(names(got), want) || total != 2 {
		t.Fatalf("text: %v of %d", names(got), total)
	}
	got, _ = Apply(rows(), cols, ir.Query{Filters: map[string][]string{"status": {"degraded", "healthy"}}})
	if len(got) != 4 {
		t.Fatalf("both options: %v", names(got))
	}
	got, _ = Apply(rows(), cols, ir.Query{Filters: map[string][]string{"status": {"degraded"}}})
	if !slices.Equal(names(got), []string{"Apple"}) {
		t.Fatalf("one option: %v", names(got))
	}
}

func TestARangeIsInclusiveAndEitherEndMayBeOpen(t *testing.T) {
	got, _ := Apply(rows(), cols, ir.Query{Ranges: map[string]ir.Range{"count": {Min: f(9), Max: f(10)}}})
	if want := []string{"banana", "Apple"}; !slices.Equal(names(got), want) {
		t.Fatalf("both: %v", names(got))
	}
	got, _ = Apply(rows(), cols, ir.Query{Ranges: map[string]ir.Range{"count": {Min: f(10)}}})
	if want := []string{"Apple", "apple"}; !slices.Equal(names(got), want) {
		t.Fatalf("min only: %v", names(got))
	}
	got, _ = Apply(rows(), cols, ir.Query{Ranges: map[string]ir.Range{"count": {Max: f(2)}}})
	if want := []string{"cherry"}; !slices.Equal(names(got), want) {
		t.Fatalf("max only: %v", names(got))
	}
}

func TestFiltersApplyBeforeTheWindowAndTotalCountsWhatPasses(t *testing.T) {
	q := ir.Query{
		Filters: map[string][]string{"status": {"healthy"}}, Sort: "name", Limit: 2, Offset: 1,
	}
	got, total := Apply(rows(), cols, q)
	// healthy: apple, banana, cherry; the window of two from the second.
	if want := []string{"banana", "cherry"}; !slices.Equal(names(got), want) || total != 3 {
		t.Fatalf("got %v of %d", names(got), total)
	}
}

func TestAWindowPastTheEndIsEmptyAndANegativeOffsetIsTheStart(t *testing.T) {
	got, total := Apply(rows(), cols, ir.Query{Offset: 99, Limit: 2})
	if len(got) != 0 || total != 4 {
		t.Fatalf("past the end: %v of %d", names(got), total)
	}
	got, _ = Apply(rows(), cols, ir.Query{Offset: -5, Limit: 2})
	if len(got) != 2 {
		t.Fatalf("negative: %v", names(got))
	}
}

func TestAColumnTheTableDoesNotHaveIsIgnoredAndRowsAreNotChanged(t *testing.T) {
	in := rows()
	got, total := Apply(in, cols, ir.Query{Sort: "nope", Filters: map[string][]string{"nope": {"x"}}, Ranges: map[string]ir.Range{"nope": {Min: f(1)}}})
	if total != 4 || len(got) != 4 {
		t.Fatalf("got %v", names(got))
	}
	Apply(in, cols, ir.Query{Sort: "name", Desc: true})
	if !slices.Equal(names(in), []string{"banana", "Apple", "cherry", "apple"}) {
		t.Fatalf("the input was reordered: %v", names(in))
	}
}

func TestASearchFindsARowByAnyColumnIgnoringCaseAndAppliesBeforeTheWindow(t *testing.T) {
	// "ap" is in two names, and "degraded" is only in a status; "100" only in a number.
	got, total := Apply(rows(), cols, ir.Query{Search: "AP"})
	if want := []string{"Apple", "apple"}; !slices.Equal(names(got), want) || total != 2 {
		t.Fatalf("by name: %v of %d", names(got), total)
	}
	got, _ = Apply(rows(), cols, ir.Query{Search: "degrad"})
	if !slices.Equal(names(got), []string{"Apple"}) {
		t.Fatalf("by status: %v", names(got))
	}
	got, _ = Apply(rows(), cols, ir.Query{Search: "100"})
	if !slices.Equal(names(got), []string{"apple"}) {
		t.Fatalf("by number: %v", names(got))
	}
	got, total = Apply(rows(), cols, ir.Query{Search: "a", Limit: 2})
	if len(got) != 2 || total != 4 { // every row has an "a"
		t.Fatalf("window: %v of %d", names(got), total)
	}
	if got, _ = Apply(rows(), cols, ir.Query{Search: "nothing like this"}); len(got) != 0 {
		t.Fatalf("no match: %v", names(got))
	}
}
