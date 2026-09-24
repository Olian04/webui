// Package assert stops a test when a check fails.
// Argument order is got, then want.
package assert

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// TB is the testing surface assertions call.
// *testing.T and *testing.B both implement it.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Equal fails t when got != want.
func Equal[T comparable](t TB, got, want T) {
	t.Helper()
	if got != want {
		failNotEqual(t, got, want)
	}
}

func Contains(t TB, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		failNotEqual(t, got, want)
	}
}

// DeepEqual fails t when reflect.DeepEqual(got, want) is false.
// Nil and empty slices are not equal.
func DeepEqual(t TB, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		failNotEqual(t, got, want)
	}
}

// NoError fails t when err is not nil.
func NoError(t TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Error fails t when err is nil.
func Error(t TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error")
	}
}

// ErrorIs fails t unless errors.Is(err, target).
func ErrorIs(t TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("got %s, want errors.Is %s", format(err), format(target))
	}
}

// Nil fails t when v is not nil. Typed nils count as nil.
func Nil(t TB, v any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("got %s, want nil", format(v))
	}
}

// NotNil fails t when v is nil. Typed nils count as nil.
func NotNil(t TB, v any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("got %s, want non-nil", format(v))
	}
}

// True fails t when cond is false.
func True(t TB, cond bool) {
	t.Helper()
	if !cond {
		t.Fatalf("got false, want true")
	}
}

// False fails t when cond is true.
func False(t TB, cond bool) {
	t.Helper()
	if cond {
		t.Fatalf("got true, want false")
	}
}

func failNotEqual(t TB, got, want any) {
	t.Helper()
	t.Fatalf("got %s, want %s", format(got), format(want))
}

func format(v any) string {
	if isNil(v) {
		return fmt.Sprintf("%#v", v)
	}
	switch v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case error:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%#v", v)
	}
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}
