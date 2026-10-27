package args

import (
	"reflect"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

type okArgs struct {
	Id     string
	Debug  bool
	Offset int
	Big    int64
	Rate   float64
	Skip   string `webui:"-"`
	Named  string `webui:"n"`
	hidden string //nolint:unused
}

func TestSpecResolvesNamesKindsAndPlacement(t *testing.T) {
	t.Parallel()

	specs, problems := Spec(reflect.TypeFor[okArgs](), "/device/{id}")
	assert.Equal(t, len(problems), 0)
	assert.DeepEqual(t, specs, []ir.ArgSpec{
		{Name: "id", Field: "Id", Kind: ir.KindString, InPath: true},
		{Name: "debug", Field: "Debug", Kind: ir.KindBool},
		{Name: "offset", Field: "Offset", Kind: ir.KindInt},
		{Name: "big", Field: "Big", Kind: ir.KindInt64},
		{Name: "rate", Field: "Rate", Kind: ir.KindFloat},
		{Name: "n", Field: "Named", Kind: ir.KindString},
	})
}

func TestSpecReportsEveryProblem(t *testing.T) {
	t.Parallel()

	type bad struct {
		When map[string]int
		A    string `webui:"x"`
		B    string `webui:"x"`
	}
	_, problems := Spec(reflect.TypeFor[bad](), "/event/{id}")
	assert.Equal(t, len(problems), 3)
	assert.Equal(t, problems[0].Detail, "field When has unsupported type map[string]int")
	assert.Contains(t, problems[1].Detail, `two fields both map to the argument "x"`)
	assert.Equal(t, problems[2].Detail, "the path declares {id} but bad has no field for it")
	assert.Equal(t, problems[2].Fix,
		"Add a field named Id to bad, or change the placeholder to match an existing field.")
}

func TestSpecRejectsNonStruct(t *testing.T) {
	t.Parallel()

	_, problems := Spec(reflect.TypeFor[int](), "/x")
	assert.Equal(t, len(problems), 1)
	assert.Contains(t, problems[0].Detail, "not a struct")
}

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[okArgs]()
	specs, _ := Spec(typ, "/device/{id}")
	c := NewCodec(typ, specs)

	got, err := c.Decode(map[string]string{"id": "abc", "debug": "true", "offset": "50", "rate": "1.5", "n": "z"})
	assert.NoError(t, err)
	want := okArgs{Id: "abc", Debug: true, Offset: 50, Rate: 1.5, Named: "z"}
	assert.DeepEqual(t, got, want)

	path, query, err := c.Encode(want)
	assert.NoError(t, err)
	assert.DeepEqual(t, path, map[string]string{"id": "abc"})
	assert.DeepEqual(t, query, map[string]string{"debug": "true", "offset": "50", "rate": "1.5", "n": "z"})
}

func TestCodecDecodeCollectsErrors(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[okArgs]()
	specs, _ := Spec(typ, "/")
	_, err := NewCodec(typ, specs).Decode(map[string]string{"offset": "x", "debug": "maybe"})
	assert.Contains(t, err.Error(), "invalid argument")
	assert.Contains(t, err.Error(), `"offset"`)
	assert.Contains(t, err.Error(), `"debug"`)
}

func TestCodecEmptyValueIsAbsent(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[okArgs]()
	specs, _ := Spec(typ, "/")
	got, err := NewCodec(typ, specs).Decode(map[string]string{"offset": ""})
	assert.NoError(t, err)
	assert.DeepEqual(t, got, okArgs{})
}

func TestCodecEncodeRejectsZeroPathArg(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[okArgs]()
	specs, _ := Spec(typ, "/device/{id}")
	_, _, err := NewCodec(typ, specs).Encode(okArgs{})
	assert.Equal(t, err.Error(), `path argument "id" is empty`)
}

func TestHrefEscapesAndSorts(t *testing.T) {
	t.Parallel()

	got := Href("/device/{id}", map[string]string{"id": "a b/c"}, map[string]string{"q": "x&y", "a": "1"})
	assert.Equal(t, got, "/device/a%20b%2Fc?a=1&q=x%26y")
	assert.Equal(t, Href("/device", nil, nil), "/device")
}

func FuzzCodecDecode(f *testing.F) {
	typ := reflect.TypeFor[okArgs]()
	specs, _ := Spec(typ, "/")
	c := NewCodec(typ, specs)
	f.Add("1", "true", "1.5")
	f.Fuzz(func(_ *testing.T, a, b, c2 string) {
		_, _ = c.Decode(map[string]string{"offset": a, "debug": b, "rate": c2, "big": a})
	})
}

func TestHrefEmitsAListAsRepeatedParameters(t *testing.T) {
	t.Parallel()

	got := Href("/p", nil, map[string]string{"t.filter.s": "a" + ir.ListSep + "b c", "q": "x"})
	assert.Equal(t, got, "/p?q=x&t.filter.s=a&t.filter.s=b+c")
	assert.Equal(t, Href("/p", nil, map[string]string{"k": ir.ListSep}), "/p") // nothing left: no query at all
}

func TestRangeKeysAreViewKeysNamedByColumn(t *testing.T) {
	t.Parallel()

	lo, hi := RangeKeys("devices", "count")
	assert.Equal(t, lo, "devices.min.count")
	assert.Equal(t, hi, "devices.max.count")
	assert.True(t, IsViewKey(lo) && IsViewKey(hi))
}

// A slice of one of the types is a query parameter that repeats: the parameter once for each element.
func TestASliceIsAQueryParameterThatRepeats(t *testing.T) {
	t.Parallel()

	type tag string
	type lists struct {
		ID    string
		Tags  []string
		Ids   []int
		Big   []int64
		Rates []float64
		Flags []bool
		Named []tag
	}
	specs, problems := Spec(reflect.TypeFor[lists](), "/device/{id}")
	assert.Equal(t, len(problems), 0)
	assert.DeepEqual(t, specs, []ir.ArgSpec{
		{Name: "id", Field: "ID", Kind: ir.KindString, InPath: true},
		{Name: "tags", Field: "Tags", Kind: ir.KindString, List: true},
		{Name: "ids", Field: "Ids", Kind: ir.KindInt, List: true},
		{Name: "big", Field: "Big", Kind: ir.KindInt64, List: true},
		{Name: "rates", Field: "Rates", Kind: ir.KindFloat, List: true},
		{Name: "flags", Field: "Flags", Kind: ir.KindBool, List: true},
		{Name: "named", Field: "Named", Kind: ir.KindString, List: true},
	})

	// What a slice of is still one of the types, and a path segment is still one value.
	type unsupported struct {
		Bytes  []byte
		Nested [][]string
		Maps   []map[string]string
	}
	_, problems = Spec(reflect.TypeFor[unsupported](), "/x")
	assert.Equal(t, len(problems), 3)
	for _, p := range problems {
		assert.Contains(t, p.Detail, "unsupported type")
	}
	type segment struct {
		ID   string
		Tags []string
	}
	_, problems = Spec(reflect.TypeFor[segment](), "/device/{id}/{tags}")
	assert.Equal(t, len(problems), 1) // not also that the path has no field for {tags}
	assert.Contains(t, problems[0].Detail, "is one segment of the path")
	assert.Contains(t, problems[0].Fix, "{tags...}")
}

// A field that is wrong is one problem. The path is not also said to have no field for it, which both cannot be: the field
// is there, and its type, its name or its place is what is wrong with it.
func TestAFieldThatIsWrongIsNotAlsoMissing(t *testing.T) {
	t.Parallel()

	type unsupported struct{ When map[string]int }
	type dotted struct {
		Page string `webui:"a.b"`
	}
	type segment struct{ Tags []string }
	for name, c := range map[string]struct {
		typ      reflect.Type
		template string
		want     string
	}{
		"an unsupported type in the path": {reflect.TypeFor[unsupported](), "/event/{when}", "field When has unsupported type map[string]int"},
		"a reserved name in the path":     {reflect.TypeFor[dotted](), "/x/{a.b}", `maps to the argument "a.b"`},
		"a slice for one segment":         {reflect.TypeFor[segment](), "/x/{tags}", "is one segment of the path"},
	} {
		_, problems := Spec(c.typ, c.template)
		if len(problems) != 1 {
			t.Errorf("%s: %d problems, want 1: %v", name, len(problems), problems)
			continue
		}
		assert.Contains(t, problems[0].Detail, c.want)
	}

	// A path that really has no field for a placeholder still says so, beside a field that is wrong.
	_, problems := Spec(reflect.TypeFor[unsupported](), "/event/{id}")
	assert.Equal(t, len(problems), 2)
	assert.Contains(t, problems[1].Detail, "the path declares {id} but unsupported has no field for it")
}
