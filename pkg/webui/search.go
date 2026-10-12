package webui

import (
	"context"
	"errors"

	"github.com/Olian04/webui/internal/tablequery"
)

// Hit is one result of a [Page]'s Search: where it leads, and what it says. It leads to the page
// the Search belongs to, at Args.
type Hit[A any] struct {
	// Args are the page's arguments that the result leads to, such as the ID of the device it
	// is. A result is offered only if the page's Guard allows the visitor to open it with them.
	Args A

	// Title is the result's first line. It is the one thing a result must have.
	Title string

	// Desc is the line beneath it, such as the device's address and site. It may be empty.
	Desc string
}

// SearchRows makes a [Page]'s Search from rows held in memory, so the library does the
// matching: a row is a result when any of the fields, as they are shown, contains what the
// visitor typed, ignoring case. rows is called on every search, and nothing is kept between
// them. hit says what a row is as a result.
//
//	Search: webui.SearchRows(loadAlerts,
//		func(a Alert) webui.Hit[AlertArgs] {
//			return webui.Hit[AlertArgs]{Args: AlertArgs{ID: a.ID}, Title: a.ID, Desc: a.Message}
//		},
//		AlertID, AlertDevice, Message),
func SearchRows[M, A any](rows func(ctx context.Context) ([]M, error), hit func(M) Hit[A], fields ...Accessor[M]) func(ctx context.Context, query string, limit int) ([]Hit[A], error) {
	cols := lowerAccessors(fields, "s")
	return func(ctx context.Context, query string, limit int) ([]Hit[A], error) {
		if rows == nil || hit == nil || len(fields) == 0 {
			return nil, errors.New("webui: SearchRows needs rows, a hit and at least one field")
		}
		all, err := rows(ctx)
		if err != nil {
			return nil, err
		}
		items := make([]any, len(all))
		for i, r := range all {
			items[i] = r
		}
		found := tablequery.Find(items, cols, query, limit)
		out := make([]Hit[A], len(found))
		for i, r := range found {
			out[i] = hit(r.(M))
		}
		return out, nil
	}
}
