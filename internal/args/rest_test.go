package args

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

type objectArgs struct {
	Bucket string
	Key    []string
}

func objectCodec(t *testing.T) *Codec {
	t.Helper()

	typ := reflect.TypeOf(objectArgs{})
	specs, problems := Spec(typ, "/bucket/{bucket}/{key...}")
	assert.Equal(t, len(problems), 0)
	assert.Equal(t, specs[1].Kind, ir.KindStrings)
	return NewCodec(typ, specs)
}

func TestARestPlaceholderIsDecodedIntoOneElementForEachSegment(t *testing.T) {
	t.Parallel()

	got, err := objectCodec(t).Decode(map[string]string{"bucket": "photos", "key": "2026/summer/img one.png"})
	assert.NoError(t, err)
	assert.DeepEqual(t, got, objectArgs{Bucket: "photos", Key: []string{"2026", "summer", "img one.png"}})

	one, err := objectCodec(t).Decode(map[string]string{"bucket": "photos", "key": "cover.png"})
	assert.NoError(t, err)
	assert.DeepEqual(t, one, objectArgs{Bucket: "photos", Key: []string{"cover.png"}})
}

func TestARestPlaceholderWithAnEmptySegmentIsNotDecoded(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"a//b", "a/", "/a", "//"} {
		_, err := objectCodec(t).Decode(map[string]string{"bucket": "b", "key": key})
		if err == nil {
			t.Errorf("%q decoded", key)
		}
	}
}

func TestARestPlaceholderIsEncodedAsTheSegmentsJoinedByASlash(t *testing.T) {
	t.Parallel()

	path, query, err := objectCodec(t).Encode(objectArgs{Bucket: "photos", Key: []string{"2026", "summer", "a b"}})
	assert.NoError(t, err)
	assert.Equal(t, path["key"], "2026/summer/a b")
	assert.Equal(t, len(query), 0)
	// Each element is escaped on its own when the address is built.
	assert.Equal(t, Href("/bucket/{bucket}/{key...}", path, nil), "/bucket/photos/2026/summer/a%20b")
}

func TestARestPlaceholderMustHaveWholeSegments(t *testing.T) {
	t.Parallel()

	c := objectCodec(t)
	for name, key := range map[string][]string{"empty list": {}, "nil list": nil, "empty element": {"a", "", "b"}, "slash in element": {"a", "b/c"}} {
		_, _, err := c.Encode(objectArgs{Bucket: "b", Key: key})
		if err == nil {
			t.Errorf("%s was encoded", name)
		}
	}
	_, _, err := c.Encode(objectArgs{Bucket: "b", Key: []string{"a", "b/c"}})
	assert.Contains(t, err.Error(), `segment 2, "b/c", contains a slash`)
}

func TestAStringIsNotARestPlaceholdersType(t *testing.T) {
	t.Parallel()

	type text struct{ Key string }
	_, problems := Spec(reflect.TypeOf(text{}), "/bucket/{key...}")
	assert.Equal(t, len(problems), 1)
	assert.Contains(t, problems[0].Detail, "{key...}, but Key is not a []string")
	assert.Contains(t, problems[0].Fix, "one element for each")
}

func TestAListIsOnlyForTheRestPlaceholder(t *testing.T) {
	t.Parallel()

	type tags struct {
		ID   string
		Tags []string
	}
	for _, template := range []string{"/device/{id}", "/device/{id}/{tags}", "/device/{id}/{tags...}/more"} {
		_, problems := Spec(reflect.TypeOf(tags{}), template)
		if len(problems) == 0 {
			t.Errorf("%s accepted a list that is not the rest", template)
		}
	}
	_, problems := Spec(reflect.TypeOf(tags{}), "/device/{id}")
	assert.Contains(t, problems[0].Detail, "a []string, but the path has no {tags...}")
	// Not a query argument either: a URL carries one value per name.
	assert.Contains(t, problems[0].Fix, "only for the last segment")

	type ints struct{ N []int }
	_, problems = Spec(reflect.TypeOf(ints{}), "/x")
	assert.Contains(t, problems[0].Detail, "unsupported type")
}

func FuzzARestPlaceholderRoundTrips(f *testing.F) {
	typ := reflect.TypeOf(objectArgs{})
	specs, _ := Spec(typ, "/bucket/{bucket}/{key...}")
	c := NewCodec(typ, specs)
	f.Add("a", "b", "c")
	f.Add("a b", "é", "?#%")
	f.Fuzz(func(t *testing.T, a, b, c2 string) {
		want := objectArgs{Bucket: "x", Key: []string{a, b, c2}}
		path, query, err := c.Encode(want)
		if err != nil {
			return // not whole segments: refused, which is the point
		}
		got, err := c.Decode(map[string]string{"bucket": "x", "key": path["key"]})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%q came back as %v, %v", want.Key, got, err)
		}
		if len(query) != 0 || strings.Count(path["key"], "/") != 2 {
			t.Fatalf("unexpected encoding %v %v", path, query)
		}
	})
}
