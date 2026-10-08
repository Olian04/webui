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

	specs, problems := Spec(reflect.TypeOf(okArgs{}), "/device/{id}")
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
		When []string
		A    string `webui:"x"`
		B    string `webui:"x"`
	}
	_, problems := Spec(reflect.TypeOf(bad{}), "/event/{id}")
	assert.Equal(t, len(problems), 3)
	assert.Equal(t, problems[0].Detail, "field When has unsupported type []string")
	assert.Contains(t, problems[1].Detail, `two fields both map to the argument "x"`)
	assert.Equal(t, problems[2].Detail, "the path declares {id} but bad has no field for it")
	assert.Equal(t, problems[2].Fix,
		"Add a field named Id to bad, or change the placeholder to match an existing field.")
}

func TestSpecRejectsNonStruct(t *testing.T) {
	t.Parallel()

	_, problems := Spec(reflect.TypeOf(0), "/x")
	assert.Equal(t, len(problems), 1)
	assert.Contains(t, problems[0].Detail, "not a struct")
}

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(okArgs{})
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

	typ := reflect.TypeOf(okArgs{})
	specs, _ := Spec(typ, "/")
	_, err := NewCodec(typ, specs).Decode(map[string]string{"offset": "x", "debug": "maybe"})
	assert.ErrorIs(t, err, ErrInvalidArg)
	assert.Contains(t, err.Error(), `"offset"`)
	assert.Contains(t, err.Error(), `"debug"`)
}

func TestCodecEmptyValueIsAbsent(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(okArgs{})
	specs, _ := Spec(typ, "/")
	got, err := NewCodec(typ, specs).Decode(map[string]string{"offset": ""})
	assert.NoError(t, err)
	assert.DeepEqual(t, got, okArgs{})
}

func TestCodecEncodeRejectsZeroPathArg(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(okArgs{})
	specs, _ := Spec(typ, "/device/{id}")
	_, _, err := NewCodec(typ, specs).Encode(okArgs{})
	assert.ErrorIs(t, err, ErrEmptyPathArg)
	assert.Equal(t, err.Error(), `path argument "id" is empty`)
}

func TestHrefEscapesAndSorts(t *testing.T) {
	t.Parallel()

	got := Href("/device/{id}", map[string]string{"id": "a b/c"}, map[string]string{"q": "x&y", "a": "1"})
	assert.Equal(t, got, "/device/a%20b%2Fc?a=1&q=x%26y")
	assert.Equal(t, Href("/device", nil, nil), "/device")
}

func FuzzCodecDecode(f *testing.F) {
	typ := reflect.TypeOf(okArgs{})
	specs, _ := Spec(typ, "/")
	c := NewCodec(typ, specs)
	f.Add("1", "true", "1.5")
	f.Fuzz(func(_ *testing.T, a, b, c2 string) {
		_, _ = c.Decode(map[string]string{"offset": a, "debug": b, "rate": c2, "big": a})
	})
}
