package args

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// ListSep joins the values of a parameter that may repeat ("a=1&a=2") into one
// string, so a request's arguments stay a map of strings. It is a control
// character: it cannot be typed into a field, so no value contains it. Href
// splits on it again, so a list round-trips through an address as repeated
// parameters, which is also what a plain HTML form of checkboxes submits.
const ListSep = "\x1f"

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

	errs := []error{}
	for _, key := range a.PathKeys {
		value := r.PathValue(key)
		if value != "" {
			args[key] = value
		} else {
			errs = append(errs, fmt.Errorf("path value is required: %q", key))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
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
				args[key] = strings.Join(kept, ListSep)
			}
			continue
		}
		if values.Has(key) {
			args[key] = values.Get(key)
		}
	}

	return args, nil
}
