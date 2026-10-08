package args

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// ErrEmptyPathArg matches (errors.Is) the error Encode returns when a path
// argument is the zero value: a path segment cannot be absent, and reflection
// cannot catch this at startup.
var ErrEmptyPathArg = errors.New("path argument is empty")

// EmptyPathArgError names the empty argument.
type EmptyPathArgError struct{ Name string }

func (e EmptyPathArgError) Error() string { return fmt.Sprintf("path argument %q is empty", e.Name) }

// Is reports whether target is ErrEmptyPathArg.
func (e EmptyPathArgError) Is(target error) bool { return target == ErrEmptyPathArg }

// ErrInvalidArg is wrapped by Decode when a raw value does not parse.
var ErrInvalidArg = errors.New("invalid argument")

// Codec converts between the raw string values of a request and the page's
// argument struct. Everything reflective is resolved in NewCodec, so Decode
// and Encode do no type discovery per request.
type Codec struct {
	typ    reflect.Type
	fields []codecField
}

type codecField struct {
	spec  ir.ArgSpec
	index int
}

// NewCodec builds a Codec for t from specs previously returned by Spec for the
// same type. It panics if a spec names no field: Spec is the only producer, so
// a mismatch is a programming error, not input.
func NewCodec(t reflect.Type, specs []ir.ArgSpec) *Codec {
	c := &Codec{typ: t, fields: make([]codecField, 0, len(specs))}
	for _, s := range specs {
		f, ok := t.FieldByName(s.Field)
		if !ok {
			panic(fmt.Sprintf("args: spec for %q names no field %s on %s", s.Name, s.Field, t))
		}
		c.fields = append(c.fields, codecField{spec: s, index: f.Index[0]})
	}
	return c
}

// Decode builds a value of the argument type from raw values keyed by argument
// name. A missing or empty value leaves the field zero. Every parse failure is
// reported, joined.
func (c *Codec) Decode(raw map[string]string) (any, error) {
	v := reflect.New(c.typ).Elem()
	var errs []error
	for _, f := range c.fields {
		s := raw[f.spec.Name]
		if s == "" {
			continue
		}
		if err := setValue(v.Field(f.index), f.spec.Kind, s); err != nil {
			errs = append(errs, fmt.Errorf("%w: %q: %w", ErrInvalidArg, f.spec.Name, err))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return v.Interface(), nil
}

// Encode splits a value of the argument type into path values and query
// values. Zero fields are omitted, so a URL never carries a default. A zero
// path argument is an error.
func (c *Codec) Encode(args any) (path, query map[string]string, err error) {
	v := reflect.ValueOf(args)
	if v.Type() != c.typ {
		return nil, nil, fmt.Errorf("args: got %s, want %s", v.Type(), c.typ)
	}
	path, query = map[string]string{}, map[string]string{}
	for _, f := range c.fields {
		fv := v.Field(f.index)
		if fv.IsZero() {
			if f.spec.InPath {
				return nil, nil, EmptyPathArgError{Name: f.spec.Name}
			}
			continue
		}
		s := formatValue(fv, f.spec.Kind)
		if f.spec.InPath {
			path[f.spec.Name] = s
		} else {
			query[f.spec.Name] = s
		}
	}
	return path, query, nil
}

func setValue(fv reflect.Value, kind ir.ValueKind, s string) error {
	switch kind {
	case ir.KindString:
		fv.SetString(s)
	case ir.KindBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case ir.KindInt, ir.KindInt64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		if fv.OverflowInt(n) {
			return fmt.Errorf("%d overflows %s", n, fv.Type())
		}
		fv.SetInt(n)
	case ir.KindFloat:
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(x)
	}
	return nil
}

func formatValue(fv reflect.Value, kind ir.ValueKind) string {
	switch kind {
	case ir.KindString:
		return fv.String()
	case ir.KindBool:
		return strconv.FormatBool(fv.Bool())
	case ir.KindInt, ir.KindInt64:
		return strconv.FormatInt(fv.Int(), 10)
	case ir.KindFloat:
		return strconv.FormatFloat(fv.Float(), 'f', -1, 64)
	}
	return ""
}

// Href joins a path template, its path values and its query values into a
// relative URL. Path segments and query values are escaped.
func Href(template string, path, query map[string]string) string {
	segs := strings.Split(template, "/")
	for i, seg := range segs {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			segs[i] = url.PathEscape(path[seg[1:len(seg)-1]])
		}
	}
	out := strings.Join(segs, "/")
	if len(query) == 0 {
		return out
	}
	q := url.Values{}
	for k, v := range query {
		q.Set(k, v)
	}
	return out + "?" + q.Encode() // Encode sorts by key: stable hrefs
}
