package args

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestParsePathAndBothQueryKeys(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts?sort_key=name&sort_order=asc",
		want:    map[string]string{"id": "123", "sort_key": "name", "sort_order": "asc"},
	})
}

func TestParsePathOnly(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/54633/posts",
		want:    map[string]string{"id": "54633"},
	})
}

func TestParseOneQueryKey(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/abcdef/posts?sort_key=email",
		want:    map[string]string{"id": "abcdef", "sort_key": "email"},
	})
}

func TestParseTrailingSlash(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts/",
		url:     "/users/123/posts/",
		want:    map[string]string{"id": "123"},
	})
}

func TestParseEmptyQueryValue(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts?sort_key=",
		want:    map[string]string{"id": "123", "sort_key": ""},
	})
}

func TestParseFirstQueryValueWins(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts?sort_key=name&sort_key=age",
		want:    map[string]string{"id": "123", "sort_key": "name"},
	})
}

func TestParseUnlistedQueryKeyIgnored(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts?page=2",
		want:    map[string]string{"id": "123"},
	})
}

func TestParseQueryValueDecoded(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts?sort_key=a%20b",
		want:    map[string]string{"id": "123", "sort_key": "a b"},
	})
}

func TestParseTooFewSegments(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123",
		wantErr: `path value is required: "id"`,
	})
}

func TestParseTooManySegments(t *testing.T) {
	t.Parallel()

	runParseTest(t, idParser(), parseTest{
		pattern: "/users/{id}/posts",
		url:     "/users/123/posts/extra",
		wantErr: `path value is required: "id"`,
	})
}

func TestParseMultipleDynamicSegments(t *testing.T) {
	t.Parallel()

	runParseTest(t, ArgParser{PathKeys: []string{"user", "post"}}, parseTest{
		pattern: "/users/{user}/posts/{post}",
		url:     "/users/9/posts/42",
		want:    map[string]string{"user": "9", "post": "42"},
	})
}

func TestParseStaticOnly(t *testing.T) {
	t.Parallel()

	runParseTest(t, ArgParser{}, parseTest{
		pattern: "/healthz/live",
		url:     "/healthz/live",
		want:    map[string]string{},
	})
}

func idParser() ArgParser {
	return ArgParser{PathKeys: []string{"id"}, QueryKeys: []string{"sort_key", "sort_order"}}
}

func mustRequest(t *testing.T, pattern, rawURL string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	assert.NoError(t, err)

	// ServeMux fills PathValue only when the pattern matches.
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(http.ResponseWriter, *http.Request) {})
	mux.ServeHTTP(httptest.NewRecorder(), req)
	return req
}

type parseTest struct {
	pattern string
	url     string
	want    map[string]string
	wantErr string
}

func runParseTest(t *testing.T, parser ArgParser, test parseTest) {
	t.Helper()

	got, err := parser.Parse(mustRequest(t, test.pattern, test.url))
	if test.wantErr != "" {
		assert.ErrorIs(t, err, ErrPathValueRequired)
		assert.Equal(t, err.Error(), test.wantErr)
		assert.Nil(t, got)
		return
	}
	assert.NoError(t, err)
	assert.DeepEqual(t, got, test.want)
}
