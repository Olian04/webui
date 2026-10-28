package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// Story is one thing that a feed says.
type Story struct {
	Title, Age, Source, URL, Picture string
	Points                           int
}

// A Feed lists items: a headline, a line of small facts, and, when it has them, a badge and a picture. Its source is a plain list, asked for one more than is shown,
// which is how the leaf knows to offer Show more. A headline is text, and where it leads is Link (an address, which opens in a tab of its own) or Open (a page of the app).
func ExampleFeed() {
	stories := []Story{
		{"Nginx has moved to GitHub", "3h", "mailman.nginx.org", "https://mailman.nginx.org/", "", 100},
		{"Show HN: Wealthfolio", "6h", "wealthfolio.app", "https://wealthfolio.app/", "/img/wealthfolio.png", 389},
		{"2M users but no money", "12h", "exercism.org", "https://exercism.org/", "https://cdn.example.com/exercism.png", 217},
	}
	page := webui.Page[webui.NoArgs]{
		Path: "/news",
		Body: webui.Feed[Story]{
			Title: "Latest", PageSize: 2,
			Load: func(_ context.Context, limit int) ([]Story, webui.Loaded) {
				return stories[:min(limit, len(stories))], webui.Ready()
			},
			Headline: webui.String[Story]{Label: "Title", Load: func(s Story) string { return s.Title }},
			Link:     func(s Story) string { return s.URL },
			Meta: []webui.Accessor[Story]{
				webui.String[Story]{Label: "Age", Load: func(s Story) string { return s.Age }},
				webui.Int[Story]{Label: "Points", Load: func(s Story) int { return s.Points }, Format: func(n int) string { return fmt.Sprintf("%d points", n) }},
			},
			// A picture from this site, or from an origin that is trusted: any other is not drawn.
			Image:          func(s Story) string { return s.Picture },
			TrustedOrigins: []string{"https://cdn.example.com"},
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/news", nil))
	body := rec.Body.String()
	fmt.Println(strings.Count(body, `<li class="item">`), "items shown")
	fmt.Println(strings.Contains(body, `href="/admin/news?latest.limit=4"`), "Show more is in the address")
	fmt.Println(strings.Contains(body, `target="_blank" rel="noopener noreferrer"`), "a headline leads to another site")
	fmt.Println(strings.Contains(body, `src="/img/wealthfolio.png"`), "a picture from this site is drawn")

	// Output:
	// 2 items shown
	// true Show more is in the address
	// true a headline leads to another site
	// true a picture from this site is drawn
}
