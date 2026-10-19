package args

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

type label string

type listArgs struct {
	ID    string
	Tags  []string
	Ids   []int
	Rates []float64
	Flags []bool
	Named []label
}

func listCodec(t *testing.T) *Codec {
	t.Helper()

	typ := reflect.TypeOf(listArgs{})
	specs, problems := Spec(typ, "/device/{id}")
	assert.Equal(t, len(problems), 0)
	return NewCodec(typ, specs)
}

// A parameter that repeats keeps every value it is given, in order, each parsed as one.
func TestARepeatedParameterIsDecodedIntoOneElementForEachValue(t *testing.T) {
	t.Parallel()

	c := listCodec(t)
	got, err := c.Decode(map[string]string{
		"id":    "a",
		"tags":  strings.Join([]string{"x", "y z", "x"}, ir.ListSep), // a repeat is kept: it is what was asked
		"ids":   strings.Join([]string{"3", "1", "2"}, ir.ListSep),
		"rates": strings.Join([]string{"0.5", "2"}, ir.ListSep),
		"flags": strings.Join([]string{"true", "false"}, ir.ListSep),
		"named": strings.Join([]string{"p", "q"}, ir.ListSep),
	})
	assert.NoError(t, err)
	assert.DeepEqual(t, got, listArgs{
		ID: "a", Tags: []string{"x", "y z", "x"}, Ids: []int{3, 1, 2}, Rates: []float64{0.5, 2},
		Flags: []bool{true, false}, Named: []label{"p", "q"},
	})

	// None is a nil slice, as a missing scalar is zero.
	none, err := c.Decode(map[string]string{"id": "a"})
	assert.NoError(t, err)
	assert.True(t, none.(listArgs).Tags == nil)
}

// A value that does not parse is reported with the argument and the value, and every one is: not the first.
func TestEveryValueOfARepeatedParameterThatDoesNotParseIsReported(t *testing.T) {
	t.Parallel()

	_, err := listCodec(t).Decode(map[string]string{"id": "a", "ids": strings.Join([]string{"1", "two", "3", "four"}, ir.ListSep)})
	if err == nil {
		t.Fatal("two values that are not numbers were accepted")
	}
	assert.Contains(t, err.Error(), `invalid argument "ids"`)
	assert.Contains(t, err.Error(), `"two"`)
	assert.Contains(t, err.Error(), `"four"`)
	assert.False(t, strings.Contains(err.Error(), `"3"`))
}

// What is encoded is what is decoded, and the address has the parameter once for each element, in order.
func TestARepeatedParameterIsEncodedAsOneParameterForEachElement(t *testing.T) {
	t.Parallel()

	c := listCodec(t)
	args := listArgs{ID: "a", Tags: []string{"b", "a c", "b"}, Ids: []int{10, 2}, Rates: []float64{1.5}, Flags: []bool{true}, Named: []label{"n"}}
	path, query, err := c.Encode(args)
	assert.NoError(t, err)
	assert.DeepEqual(t, path, map[string]string{"id": "a"})

	href := Href("/device/{id}", path, query)
	assert.Equal(t, href, "/device/a?flags=true&ids=10&ids=2&named=n&rates=1.5&tags=b&tags=a+c&tags=b")

	// The address is read back into the same value.
	u, err := url.Parse("http://x" + href)
	assert.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, u.String(), nil)
	r.SetPathValue("id", "a")
	parser := ArgParser{PathKeys: []string{"id"}, QueryKeys: []string{"tags", "ids", "rates", "flags", "named"}, ListKeys: []string{"tags", "ids", "rates", "flags", "named"}}
	raw, err := parser.Parse(r)
	assert.NoError(t, err)
	back, err := c.Decode(raw)
	assert.NoError(t, err)
	assert.DeepEqual(t, back, args)

	// An empty list, a nil one and a list of nothing are as absent as a zero scalar: the address carries no default.
	for name, v := range map[string]listArgs{"nil": {ID: "a"}, "empty": {ID: "a", Tags: []string{}, Ids: []int{}}, "empty strings": {ID: "a", Tags: []string{"", ""}}} {
		_, query, err := c.Encode(v)
		assert.NoError(t, err)
		if len(query) != 0 {
			t.Errorf("%s: the address carries %v", name, query)
		}
	}
}

// An element that holds the separator of the list would be read back as two, so it is refused rather than changed.
func TestAnElementThatWouldBeReadBackAsTwoIsRefused(t *testing.T) {
	t.Parallel()

	_, _, err := listCodec(t).Encode(listArgs{ID: "a", Tags: []string{"fine", "a" + ir.ListSep + "b"}})
	if err == nil {
		t.Fatal("an element holding the separator was encoded")
	}
	assert.Contains(t, err.Error(), `argument "tags"`)
	assert.Contains(t, err.Error(), "element 2")
}

func FuzzARepeatedParameterRoundTrips(f *testing.F) {
	f.Add("a", "b c", "é")
	f.Add("?#%&=", "+", "x y")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		codec := listCodec(t)
		tags := []string{}
		for _, s := range []string{a, b, c} {
			if s != "" && !strings.Contains(s, ir.ListSep) {
				tags = append(tags, s)
			}
		}
		want := listArgs{ID: "id", Tags: tags}
		if len(tags) == 0 {
			want.Tags = nil
		}
		path, query, err := codec.Encode(listArgs{ID: "id", Tags: tags})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet, "http://x"+Href("/device/{id}", path, query), nil)
		r.SetPathValue("id", "id")
		raw, err := (&ArgParser{PathKeys: []string{"id"}, QueryKeys: []string{"tags"}, ListKeys: []string{"tags"}}).Parse(r)
		if err != nil {
			t.Fatal(err)
		}
		got, err := codec.Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}
