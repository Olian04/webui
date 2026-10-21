package render

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

// SearchHit is one result of the global search: what it says and where it leads.
type SearchHit struct {
	Title, Desc, Href string
}

// SearchGroup is the results of one page, under its name.
type SearchGroup struct {
	Label string
	Hits  []SearchHit
}

// SearchFragment is the results as the box lists them, for script to put in it: the labels and the links, or that nothing matches.
func SearchFragment(groups []SearchGroup) templ.Component {
	return SearchList(groups, true)
}

// SearchResults is the page of results of a search, in the shell of the app: what a search without script is answered with.
func SearchResults(query string, groups []SearchGroup) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return SearchPage(query, groups).Render(ctx, w)
	})
}
