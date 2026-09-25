package args

import (
	"errors"
	"fmt"
	"net/http"
)

var ErrPathValueRequired = errors.New("path value is required")

type ArgParser struct {
	PathKeys  []string
	QueryKeys []string
}

func (a *ArgParser) Parse(r *http.Request) (map[string]string, error) {
	args := make(map[string]string)

	errs := []error{}
	for _, key := range a.PathKeys {
		value := r.PathValue(key)
		if value != "" {
			args[key] = value
		} else {
			errs = append(errs, fmt.Errorf("%w: %q", ErrPathValueRequired, key))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	values := r.URL.Query()
	for _, key := range a.QueryKeys {
		if values.Has(key) {
			args[key] = values.Get(key)
		}
	}

	return args, nil
}
