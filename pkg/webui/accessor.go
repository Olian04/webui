package webui

// Accessor is a named, typed projection of M. A table cell and a form input
// share the same accessor.
type Accessor[M any] interface {
	isAccessor()
}

// String projects a string field of M.
type String[M any] struct {
	Label string
	Load  func(M) string
	Store func(*M, string)
	Rules StringRules
}

func (String[M]) isAccessor() {}

// StringRules is declarative validation for a string value.
// Scalar zero means "no constraint". Pattern is a pointer because the
// expression needs a user-facing message.
type StringRules struct {
	Required bool
	MinLen   int
	Pattern  *PatternRule
}

// PatternRule is an RE2 expression plus the message shown when it fails.
type PatternRule struct {
	Expr    string
	Message string
}

// Int projects an int field of M.
type Int[M any] struct {
	Label string
	Load  func(M) int
	Store func(*M, int)
	Rules IntRules
}

func (Int[M]) isAccessor() {}

// IntRules is declarative validation for an int value.
// Min and Max are pointers because a zero bound is a real constraint.
type IntRules struct {
	Required bool
	Min      *int
	Max      *int
}

// Float projects a float64 field of M.
type Float[M any] struct {
	Label string
	Load  func(M) float64
	Store func(*M, float64)
	Rules FloatRules
}

func (Float[M]) isAccessor() {}

// FloatRules is declarative validation for a float64 value.
type FloatRules struct {
	Required bool
	Min      *float64
	Max      *float64
}

// Group composes accessors inside a form. It is not a table column;
// Compile rejects Group listed in Columns.
type Group[M any] []Accessor[M]

func (Group[M]) isAccessor() {}

// Sortable decorates an accessor with the query key used for sorting.
type Sortable[M any] struct {
	Accessor Accessor[M]
	Key      string
}

func (Sortable[M]) isAccessor() {}

// Placeholder decorates an accessor with placeholder text.
type Placeholder[M any] struct {
	Accessor Accessor[M]
	Text     string
}

func (Placeholder[M]) isAccessor() {}

// ASSERT: accessors implement Accessor
var (
	_ Accessor[struct{}] = String[struct{}]{}
	_ Accessor[struct{}] = Int[struct{}]{}
	_ Accessor[struct{}] = Float[struct{}]{}
	_ Accessor[struct{}] = Group[struct{}](nil)
	_ Accessor[struct{}] = Sortable[struct{}]{}
	_ Accessor[struct{}] = Placeholder[struct{}]{}
)
