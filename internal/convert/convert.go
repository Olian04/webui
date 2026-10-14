// Package convert is the one way a list becomes another: the conversion between a declaration's
// list and the IR's, and between the IR's and what a response is made of.
package convert

// Slice is in with f applied to each element. An empty list is an empty list, not nil.
func Slice[T, U any](in []T, f func(T) U) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// Any is rows as a list of any, which is how the IR holds rows whatever their model.
func Any[M any](rows []M) []any { return Slice(rows, func(r M) any { return r }) }
