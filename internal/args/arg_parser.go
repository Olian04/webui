package args

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

type ArgParser struct {
	PathKeys  []string
	QueryKeys []string

	// ListKeys are the query keys that keep every value they are given, joined
	// with ListSep. Any other key keeps the first, as a URL carries one value
	// per name.
	ListKeys []string
}

func (a *ArgParser) Parse(r *http.Request) (map[string]string, error) {
	args := make(map[string]string)
	for _, key := range a.PathKeys {
		value := r.PathValue(key)
		if value == "" {
			return nil, fmt.Errorf("path value is required: %q", key)
		}
		args[key] = value
	}

	values := r.URL.Query()
	for _, key := range a.QueryKeys {
		if slices.Contains(a.ListKeys, key) {
			var kept []string
			for _, v := range values[key] {
				if v != "" {
					kept = append(kept, v)
				}
			}
			if len(kept) > 0 {
				args[key] = strings.Join(kept, ir.ListSep)
			}
			continue
		}
		if values.Has(key) {
			args[key] = values.Get(key)
		}
	}

	return args, nil
}
