package webui

import "context"

// Action is a mutation of M. Guard enables the control and authorises Run.
type Action[M any] struct {
	Guard func(ctx context.Context, m M) error
	Run   func(ctx context.Context, m M) (Effect, error)
}

func (Action[M]) isRowClick() {}

// Effect is the outcome of an understood action. A nil error on Run plus
// Fields is a validation rejection the user can fix. Redirect navigates.
type Effect struct {
	Toast    string
	Fields   FieldErrors
	Redirect Target
}

// FieldErrors is Fields[M] with the model erased so Effect stays non-generic.
type FieldErrors interface {
	isFieldErrors()
}

// Fields is a list of per-accessor messages for model M.
type Fields[M any] []FieldError[M]

func (Fields[M]) isFieldErrors() {}

// FieldError is one accessor's rejection message. Use keyed literals.
type FieldError[M any] struct {
	Field   Accessor[M]
	Message string
}

// ASSERT: Action implements RowClick; Fields implements FieldErrors
var (
	_ RowClick[struct{}] = Action[struct{}]{}
	_ FieldErrors        = Fields[struct{}](nil)
)
