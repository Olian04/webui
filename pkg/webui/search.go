package webui

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rank"
	"github.com/Olian04/webui/internal/tablequery"
)

// Hit is one result of a [Page]'s Search: where it leads, and what it says. It leads to the page
// the Search belongs to, at Args.
type Hit[A any] struct {
	// Args are the page's arguments that the result leads to, such as the ID of the device it
	// is. A result is offered only if the page's Guard allows the visitor to open it with them.
	Args A

	// Title is the result's first line. It is the one thing a result must have. What the visitor typed is matched against it to
	// order the results: a title that is what was typed, starts or ends with it, or has it as a word, is above one that has it
	// in the middle of a word, and that is above one that has only its characters in order, with others between them.
	Title string

	// Desc is the line beneath it, such as the device's address and site. It may be empty. It orders the results too, for a little over
	// half as much as the Title does.
	Desc string
}

// SearchRows makes a [Page]'s Search from rows held in memory, so the library does the
// matching: a row is a result when any of the fields, as they are shown, contains what the
// visitor typed, ignoring case. The best matches of its hits, by Title and Desc, are the ones kept when there
// are more than the limit, and the global search orders every page's results the same way. rows is called on every search, and nothing is kept between
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
		fields, err := ir.ResolveChoices(ctx, cols) // a select that reads its options, for what it says
		if err != nil {
			return nil, err
		}
		// Every match is made a result and ranked, and then the best limit of them are kept: the first rows to match are not the best.
		found := tablequery.Find(ctx, convert.Any(all), fields, query, 0)
		hits := convert.Slice(found, func(r any) Hit[A] { return hit(r.(M)) })
		slices.SortStableFunc(hits, func(a, b Hit[A]) int {
			return cmp.Compare(rank.Hit(query, b.Title, b.Desc), rank.Hit(query, a.Title, a.Desc))
		})
		if limit > 0 && len(hits) > limit {
			hits = hits[:limit]
		}
		return hits, nil
	}
}
