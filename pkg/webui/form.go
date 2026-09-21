package webui

import "context"

// Form is a leaf that loads one M and submits through Action.
type Form[M any] struct {
	Load   func(ctx context.Context) (M, error)
	Submit Action[M]
	Fields []Accessor[M]
}

func (Form[M]) isPageBody() {}

// ASSERT: Form implements PageBody
var _ PageBody = Form[struct{}]{}
