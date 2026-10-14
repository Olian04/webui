package args

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

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
			errs = append(errs, fmt.Errorf("invalid argument %q: %w", f.spec.Name, err))
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
		// An empty list is as absent as a nil one, though only the nil one is the zero value.
		if fv.IsZero() || (f.spec.Kind == ir.KindStrings && fv.Len() == 0) {
			if f.spec.InPath {
				return nil, nil, fmt.Errorf("path argument %q is empty", f.spec.Name)
			}
			continue
		}
		if f.spec.Kind == ir.KindStrings {
			if err := checkSegments(f.spec.Name, fv); err != nil {
				return nil, nil, err
			}
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

// checkSegments refuses a path made of anything but whole segments: an empty element would
// be a doubled slash, and one holding a slash would be read back as two.
func checkSegments(name string, fv reflect.Value) error {
	for i := range fv.Len() {
		part := fv.Index(i).String()
		switch {
		case part == "":
			return fmt.Errorf("path argument %q: segment %d is empty", name, i+1)
		case strings.Contains(part, "/"):
			return fmt.Errorf("path argument %q: segment %d, %q, contains a slash, which would make it two", name, i+1, part)
		}
	}
	return nil
}

func setValue(fv reflect.Value, kind ir.ValueKind, s string) error {
	switch kind {
	case ir.KindStrings:
		parts := strings.Split(s, "/")
		if slices.Contains(parts, "") {
			return errors.New("it has an empty segment")
		}
		// Element by element: a slice of a defined string type is not convertible from []string.
		list := reflect.MakeSlice(fv.Type(), len(parts), len(parts))
		for i, part := range parts {
			list.Index(i).SetString(part)
		}
		fv.Set(list)
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
	case ir.KindStrings:
		parts := make([]string, fv.Len())
		for i := range parts {
			parts[i] = fv.Index(i).String()
		}
		return strings.Join(parts, "/")
	}
	return ""
}

// Href joins a path template, its path values and its query values into a
// relative URL. Path segments and query values are escaped.
func Href(template string, path, query map[string]string) string {
	segs := strings.Split(template, "/")
	for i, seg := range segs {
		name, rest, ok := Placeholder(seg)
		switch {
		case ok && rest:
			parts := strings.Split(path[name], "/") // the slashes are the path's, the rest is escaped
			for j, part := range parts {
				parts[j] = url.PathEscape(part)
			}
			segs[i] = strings.Join(parts, "/")
		case ok:
			segs[i] = url.PathEscape(path[name])
		}
	}
	out := strings.Join(segs, "/")
	if len(query) == 0 {
		return out
	}
	q := url.Values{}
	for k, v := range query {
		for _, part := range strings.Split(v, ir.ListSep) {
			if part != "" {
				q.Add(k, part) // a list is repeated parameters
			}
		}
	}
	if len(q) == 0 {
		return out
	}
	return out + "?" + q.Encode() // Encode sorts by key: stable hrefs
}
