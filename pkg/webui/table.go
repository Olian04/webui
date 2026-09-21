package webui

import "context"

// Table is a leaf that lists rows of M.
type Table[M any] struct {
	Load        func(ctx context.Context) ([]M, error)
	RowClick    RowClick[M]
	Actions     []Action[M]
	BulkActions []Action[[]M]
	Columns     []Accessor[M]
}

func (Table[M]) isPageBody() {}

// ASSERT: Table implements PageBody
var _ PageBody = Table[struct{}]{}
