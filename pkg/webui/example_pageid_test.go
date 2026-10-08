package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// Two pages that link to each other. A page that is linked back to declares its
// path as a PageID constant, which is not a variable, so naming it from the other
// page is not an initialization cycle.
func ExamplePageID() {
	type item struct{ ID, Name string }
	type itemArgs struct{ ID string }

	const listPath webui.PageID[webui.NoArgs] = "/item"

	id := webui.String[item]{Label: "ID", Load: func(i item) string { return i.ID }}

	// The detail page's form saves, and then goes back to the list by its PageID.
	detail := webui.Page[itemArgs]{
		Path: "/item/{id}",
		Body: webui.Form[item]{
			Load:   func(ctx context.Context) (item, error) { return item{ID: webui.ArgsOf[itemArgs](ctx).ID}, nil },
			Fields: []webui.Accessor[item]{id},
			Submit: webui.Action[item]{
				Run: func(ctx context.Context, _ item) (webui.Outcome, error) {
					return webui.Success("Saved").Then(webui.Open(ctx, listPath, webui.NoArgs{})), nil
				},
			},
		},
	}

	// The list links forward to the detail page by its variable.
	list := webui.Page[webui.NoArgs]{
		Path: listPath,
		Nav:  webui.Nav{Label: "Items"},
		Body: webui.Table[item]{
			Rows: func(context.Context) ([]item, error) { return []item{{ID: "a", Name: "Alpha"}}, nil },
			RowClick: webui.Link[item, itemArgs]{
				Page: detail,
				Args: func(_ context.Context, i item) itemArgs { return itemArgs{ID: i.ID} },
			},
			Columns: []webui.Accessor[item]{id},
		},
	}

	handler := webui.App{Pages: webui.Pages{list, detail}}.MustCompile("/admin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/item", nil))
	fmt.Println(strings.Contains(rec.Body.String(), `href="/admin/item/a?webui.from=%2Fadmin%2Fitem"`))
	// Output:
	// true
}
