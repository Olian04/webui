package webui_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// story is one thing of a list: a headline, where it came from, how it did, and a picture.
type story struct {
	ID     string
	Title  string
	Age    string
	Source string
	Points int
	Kind   string
	Image  string
	Link   string
}

type storyArgs struct{ ID string }

var (
	pTitle  = webui.String[story]{Label: "Title", Load: func(p story) string { return p.Title }}
	pLink   = func(p story) string { return p.Link }
	pAge    = webui.String[story]{Label: "Age", Load: func(p story) string { return p.Age }}
	pSource = webui.String[story]{Label: "Source", Load: func(p story) string { return p.Source }}
	pPoints = webui.Int[story]{Label: "Points", Load: func(p story) int { return p.Points }, Format: func(n int) string { return fmt.Sprintf("%d points", n) }}
	pKind   = webui.Select[story]{Label: "Kind", Load: func(p story) []string { return []string{p.Kind} }, Options: []webui.Option{
		{Value: "live", Label: "Live", Tone: webui.ToneCritical}, {Value: "news", Label: "News"},
	}}
)

var stories = []story{
	{ID: "a", Title: "Show HN: Wealthfolio", Age: "6h", Source: "wealthfolio.app", Points: 389, Kind: "news", Link: "https://wealthfolio.app/"},
	{ID: "b", Title: "Nginx has moved to GitHub", Age: "3h", Source: "mailman.nginx.org", Points: 100, Kind: "news", Link: "https://mailman.nginx.org/"},
	{ID: "c", Title: "Infinity, realistic AI characters", Age: "2h", Source: "", Points: 71, Kind: "live", Link: "https://example.com/c"},
	{ID: "d", Title: "A thermonuclear secondary", Age: "11h", Source: "blog.example", Points: 482, Kind: "news", Link: "https://example.com/d"},
	{ID: "e", Title: "2M users but no money", Age: "12h", Source: "exercism.org", Points: 217, Kind: "news", Link: "https://example.com/e"},
}

// listLoader serves the stories, and notes how many it was asked for.
type listLoader struct {
	mu     sync.Mutex
	limits []int
}

func (f *listLoader) load(_ context.Context, limit int) ([]story, webui.Loaded) {
	f.mu.Lock()
	f.limits = append(f.limits, limit)
	f.mu.Unlock()
	return stories[:min(limit, len(stories))], webui.Ready()
}

// loader is the source as a loader that is asked for the first items: it notes the size of the window, which is what the list asked for.
func (f *listLoader) loader() webui.Loader[story] {
	return webui.PagedLoader[story]{Load: func(ctx context.Context, q webui.PagedQuery) (webui.PagedWindow[story], webui.Loaded) {
		items, loaded := f.load(ctx, q.Limit)
		return webui.PagedWindow[story]{Items: items}, loaded
	}}
}

func (f *listLoader) asked() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.limits...)
}

// listPages is the page a list is on, and the page its items open, which has an argument.
func listPages(body webui.View, guard func(context.Context, storyArgs) error) (http.Handler, webui.Page[storyArgs]) {
	detail := webui.Page[storyArgs]{Path: "/story/{id}", Guard: guard, View: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{Path: "/e", View: body}
	return webui.App{Pages: webui.Pages{list, detail}}.MustCompile("/admin"), detail
}

func openStory(detail webui.Page[storyArgs]) webui.Link[story, storyArgs] {
	return webui.Link[story, storyArgs]{Page: detail, Args: func(_ context.Context, p story) storyArgs { return storyArgs{ID: p.ID} }}
}

func TestAListIsAColumnOfItemsEachWithAHeadlineAndALineOfFacts(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{
		Title: "Latest", Loader: src.loader(), PageSize: 3,
		Headline: pTitle, Meta: []webui.Accessor[story]{pAge, pPoints, pSource},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())

	assert.Equal(t, len(p.ByRole("heading", "Latest")), 1)
	// PageSize items, and not the one more that was asked for to know there is more, in the order given, each a headline and its facts in order.
	assert.DeepEqual(t, items(p), []string{
		"Show HN: Wealthfolio 6h 389 points wealthfolio.app",
		"Nginx has moved to GitHub 3h 100 points mailman.nginx.org",
		"Infinity, realistic AI characters 2h 71 points", // a fact with no value (this one has no source) is left out
	})
	assert.False(t, strings.Contains(rec.Body.String(), "thermonuclear"))

	// A dot between the facts, and not after the last.
	li := p.ByRole("listitem", "")
	assert.Equal(t, strings.Count(li[0].AllText(), "·"), 2) // between three facts
	assert.Equal(t, strings.Count(li[2].AllText(), "·"), 1)
	assert.Equal(t, len(p.ByRole("script", "")), 0)                  // and nothing that the data brought runs
	assert.Equal(t, strings.Count(rec.Body.String(), "<script>"), 1) // the one that sets the theme
}

func TestASourceIsAskedForOneMoreThanIsShownAndShowMoreIsARealLinkThatAsksForMore(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 2, Headline: pTitle})

	p := pageOf(t, h, "/admin/e")
	assert.DeepEqual(t, src.asked(), []int{3})
	assert.Equal(t, len(items(p)), 2)
	assert.Contains(t, p.Root().Text(), "2 shown")
	more := page.One(t, "the Show more link", p.ByRoleContaining("link", "Show more"))
	assert.Equal(t, more.Attr("href"), "/admin/e?latest.limit=4") // the same page with more, in the address, so a copied link shows the same
	assert.True(t, more.Has("data-in-place"))                     // with script it replaces the list, and without it is a navigation

	// Following it shows four, and asks for five.
	p = pageOf(t, h, "/admin/e?latest.limit=4")
	assert.DeepEqual(t, src.asked(), []int{3, 5})
	assert.Equal(t, len(items(p)), 4)
	assert.Equal(t, page.One(t, "the Show more link", p.ByRoleContaining("link", "Show more")).Attr("href"), "/admin/e?latest.limit=6")

	// The fifth is the last there is: the source gave no more than was asked for, so there is nothing more and no link.
	p = pageOf(t, h, "/admin/e?latest.limit=5")
	assert.Equal(t, len(items(p)), 5)
	assert.Equal(t, len(p.ByRoleContaining("link", "Show more")), 0)
	assert.False(t, strings.Contains(p.Root().Text(), "shown"))
}

// The address says how many, and a hand-edited one cannot ask a source for more than a list may show, or for fewer than it shows at first.
func TestTheLimitInTheAddressIsHeldToWhatAListMayShow(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 2, Headline: pTitle})
	for _, raw := range []string{"999999", "-5", "0", "x", ""} {
		serve(h, http.MethodGet, "/admin/e?latest.limit="+raw)
	}
	assert.DeepEqual(t, src.asked(), []int{201, 3, 3, 3, 3})
}

func TestAListIsNamedInTheAddressByItsTitleAndTwoWithNoTitleAreTold(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Stack{
		webui.List[story]{Loader: src.loader(), PageSize: 1, Headline: pTitle},
		webui.List[story]{Loader: src.loader(), PageSize: 1, Headline: pTitle},
		webui.Cards[story]{Loader: src.loader(), PageSize: 1, Headline: pTitle},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `href="/admin/e?list.limit=2"`)
	assert.Contains(t, body, `href="/admin/e?list-2.limit=2"`)
	assert.Contains(t, body, `href="/admin/e?cards.limit=2"`)

	// The state of one is not another's.
	asked := len(src.asked())
	serve(h, http.MethodGet, "/admin/e?list-2.limit=3")
	got := src.asked()[asked:]
	slices.Sort(got)                         // the panels of a page load at the same time, so the order they asked in is not fixed
	assert.DeepEqual(t, got, []int{2, 2, 4}) // the first list and the cards at one past what they show, and the second list at three, which is four
}

func TestAListRefreshesAsAPanelOfItsOwn(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Stack{
		webui.List[story]{Title: "First", Loader: src.loader(), PageSize: 1, Headline: pTitle},
		webui.List[story]{Title: "Second", Loader: src.loader(), PageSize: 2, Headline: pTitle},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e?second.limit=3", nil)
	req.Header.Set("X-Webui-View", "p.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, page.IsFragment(body))
	p := page.Parse(t, body)
	assert.Equal(t, len(items(p)), 3)
	assert.Equal(t, len(p.ByRole("heading", "First")), 0)
	assert.Equal(t, page.One(t, "the panel", p.Views()).Attr("data-view"), "p.1")
}

func TestAFailingListLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Loader: webui.RowsLoader[story]{Load: func(context.Context) ([]story, webui.Loaded) {
			return nil, webui.Failed(errors.New("store secret-bucket is gone"))
		}},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())
	assert.Contains(t, p.Root().Text(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "secret-bucket"))
	assert.Equal(t, len(p.ByRole("list", "")), 0)
}

func TestAListWithNoItemsSaysSoAndHasNoFoot(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Loader: webui.RowsLoader[story]{Load: func(context.Context) ([]story, webui.Loaded) { return nil, webui.Ready() }},
	})
	p := pageOf(t, h, "/admin/e")
	assert.Contains(t, p.Root().Text(), "Nothing here yet")
	assert.Equal(t, len(p.ByRole("list", "")), 0)
	assert.Equal(t, len(p.ByRoleContaining("link", "Show more")), 0)
}

// A list of something that is still going asks to be asked again, as any panel does.
func TestAListThatIsNotFinalIsAskedForAgain(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Loader: webui.RowsLoader[story]{Load: func(context.Context) ([]story, webui.Loaded) {
			return stories[:1], webui.Ready().RefreshAfter(5 * 1e9)
		}},
	})
	assert.Equal(t, page.One(t, "the panel that is not final", pageOf(t, h, "/admin/e").ByAttr("data-refresh-after", "")).Attr("data-refresh-after"), "5")
}

// ----------------------------------------------------------- headline and links

func TestAnHrefLeadsToAnotherSiteAndALinkLeadsInTheApp(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	_, detail := listPages(webui.Stack{}, nil)

	external, _ := listPages(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 1, Headline: pTitle, Open: webui.Href[story]{URL: pLink}}, nil)
	out := itemLinks(pageOf(t, external, "/admin/e"))
	assert.Equal(t, len(out), 1)
	assert.Equal(t, out[0].Href, "https://wealthfolio.app/")
	assert.True(t, out[0].NewTab)
	assert.True(t, strings.HasPrefix(out[0].Name, "Show HN: Wealthfolio") && strings.HasSuffix(out[0].Name, "(opens in a new tab)")) // and a reader is told so

	inApp, _ := listPages(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 1, Headline: pTitle, Open: openStory(detail)}, nil)
	assert.DeepEqual(t, itemLinks(pageOf(t, inApp, "/admin/e")), []page.Link{{Name: "Show HN: Wealthfolio", Href: "/admin/story/a"}})
}

// An item that leads to a page that refuses the visitor is its headline and nothing more: a link they cannot follow is not offered.
func TestAnItemWhosePageRefusesTheVisitorIsNotALink(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	_, detail := listPages(webui.Stack{}, nil)
	refuses := func(_ context.Context, a storyArgs) error {
		if a.ID == "b" {
			return errors.New("not for you")
		}
		return nil
	}
	h, _ := listPages(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 2, Headline: pTitle, Open: openStory(detail)}, refuses)
	p := pageOf(t, h, "/admin/e")
	assert.DeepEqual(t, itemLinks(p), []page.Link{{Name: "Show HN: Wealthfolio", Href: "/admin/story/a"}})
	assert.Contains(t, p.Root().Text(), "Nginx has moved to GitHub") // still there, as text
}

func TestABadgeIsDrawnBesideTheHeadlineAsItIsInACell(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 3, Headline: pTitle, Badge: pKind})
	got := items(pageOf(t, h, "/admin/e"))
	assert.Equal(t, len(got), 3)
	live := 0 // the live one says Live in words, and not only in red
	for _, text := range got {
		if strings.Contains(text, "Live") {
			live++
		}
	}
	assert.Equal(t, live, 1)
}

// ------------------------------------------------------------------- pictures

func TestAPictureIsDrawnFromThisSiteOrATrustedOriginAndFromNowhereElse(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		image string
		drawn string // what the picture's address is, or "" when it is not drawn
	}{
		"this site":              {"/uploads/a.png", "/uploads/a.png"},
		"a trusted origin":       {"https://cdn.example.com/a.png?w=40", "https://cdn.example.com/a.png?w=40"},
		"a trusted origin, case": {"https://CDN.example.com/a.png", "https://cdn.example.com/a.png"},
		"another origin":         {"https://evil.example/a.png", ""},
		"not https":              {"http://cdn.example.com/a.png", ""},
		"another site's path":    {"//evil.example/a.png", ""},
		"a script":               {"javascript:alert(1)", ""},
		"a data address":         {"data:image/svg+xml;base64,PHN2Zz4=", ""},
		"a backslash":            {`/\evil.example/a.png`, ""},
		"a user":                 {"https://u:p@cdn.example.com/a.png", ""},
		"a space":                {"/a b.png", ""},
		"nothing":                {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			image := tc.image
			h := editorApp(webui.List[story]{
				Title: "Latest", Headline: pTitle, TrustedOrigins: []string{"https://cdn.example.com"},
				Loader: webui.RowsLoader[story]{Load: func(context.Context) ([]story, webui.Loaded) {
					return []story{{ID: "a", Title: "X"}}, webui.Ready()
				}},
				Image: func(story) string { return image },
			})
			body := serve(h, http.MethodGet, "/admin/e").Body.String()
			p := page.Parse(t, body)
			pictures := p.ByRole("presentation", "") // a picture that is decoration: the headline beside it says what the item is
			if tc.drawn == "" {
				assert.Equal(t, len(pictures), 0)
				assert.False(t, strings.Contains(body, "evil.example"))
				assert.DeepEqual(t, items(p), []string{"X"}) // the item is still there, with no picture, as one that has none
				return
			}
			assert.Equal(t, len(pictures), 1)
			assert.Equal(t, pictures[0].Attr("src"), tc.drawn)
			assert.Equal(t, pictures[0].Attr("loading"), "lazy") // a card that is not in view is not fetched
			assert.Equal(t, pictures[0].Attr("referrerpolicy"), "no-referrer")
		})
	}
}

func TestThePolicyOfAPageThatTrustsAnOriginForPicturesHasItAndNoOtherPageDoes(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	with := webui.Page[webui.NoArgs]{Path: "/with", View: webui.Stack{
		webui.List[story]{Title: "A", Loader: src.loader(), Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"https://a.example", "https://shared.example"}},
		webui.Cards[story]{Title: "B", Loader: src.loader(), Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"https://shared.example", "https://B.example:443"}},
	}}
	without := webui.Page[webui.NoArgs]{Path: "/without", View: webui.List[story]{Title: "C", Loader: src.loader(), Headline: pTitle}}
	h := webui.App{Pages: webui.Pages{with, without}}.MustCompile("/admin")

	want := strings.Replace(basePolicy, "img-src 'self' data:", "img-src 'self' data: https://a.example https://shared.example https://b.example", 1)
	assert.Equal(t, policyOf(h, "/admin/with"), want)
	assert.Equal(t, policyOf(h, "/admin/without"), basePolicy)
}

// ---------------------------------------------------------------------- cards

func TestCardsAreAGridThatWrapsAndEachHasItsPictureOverItsHeadline(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Cards[story]{
		Title: "Videos", Loader: src.loader(), PageSize: 2, Headline: pTitle, Meta: []webui.Accessor[story]{pAge, pSource},
		Image: func(p story) string { return "/img/" + p.ID + ".png" },
	})
	p := pageOf(t, h, "/admin/e")
	assert.Equal(t, len(items(p)), 2)
	card := p.ByRole("listitem", "")[0]
	picture := page.One(t, "the first card's picture", card.ByRole("presentation", ""))
	assert.Equal(t, picture.Attr("src"), "/img/a.png")
	assert.True(t, picture.Before(page.One(t, "its headline", card.ByText("Wealthfolio")))) // the picture first
	assert.True(t, len(p.ByRoleContaining("link", "Show more")) == 1)                       // and the same Show more
	assert.Equal(t, p.ByRoleContaining("link", "Show more")[0].Attr("href"), "/admin/e?videos.limit=4")
	assert.Equal(t, len(p.ByRole("region", "Videos, scrolls sideways")), 0) // a grid that wraps does not scroll
}

func TestCardsInARowScrollSidewaysAndTheRowCanBeReachedByTheKeyboard(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Cards[story]{Title: "Videos", Loader: src.loader(), PageSize: 3, Headline: pTitle, Layout: webui.CardsScroll})
	p := pageOf(t, h, "/admin/e")
	row := page.One(t, "the row that scrolls", p.ByRole("region", "Videos, scrolls sideways"))
	assert.Equal(t, row.Attr("tabindex"), "0") // a stop of the keys, so that a keyboard can scroll it
	assert.Equal(t, len(items(p)), 3)
}

// A List and a Tabs make the widget with two views of what is new: only the selected tab is read.
func TestTabsOfListsAreReadOnlyForTheSelectedTab(t *testing.T) {
	t.Parallel()

	first, second := &listLoader{}, &listLoader{}
	h := editorApp(webui.Tabs{
		{Label: "Hacker News", View: webui.List[story]{Title: "Hacker News", Loader: first.loader(), Headline: pTitle}},
		{Label: "Lobsters", View: webui.List[story]{Title: "Lobsters", Loader: second.loader(), Headline: pTitle}},
	})
	serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, len(first.asked()), 1)
	assert.Equal(t, len(second.asked()), 0)
	serve(h, http.MethodGet, "/admin/e?tabs.tab=lobsters")
	assert.Equal(t, len(second.asked()), 1)
}

// ------------------------------------------------------------------- validate

func TestAListOrCardsThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	load := (&listLoader{}).loader()
	for name, tc := range map[string]struct {
		body  webui.View
		names []string
	}{
		"no load":               {webui.List[story]{Headline: pTitle}, []string{"List", "Load"}},
		"no headline":           {webui.List[story]{Loader: load}, []string{"List.Headline"}},
		"a group headline":      {webui.List[story]{Loader: load, Headline: webui.Group[story]{pTitle}}, []string{"List.Headline", "Group"}},
		"a headline no load":    {webui.List[story]{Loader: load, Headline: webui.String[story]{Label: "T"}}, []string{"List.Headline", "Load"}},
		"a meta no label":       {webui.List[story]{Loader: load, Headline: pTitle, Meta: []webui.Accessor[story]{webui.String[story]{Load: pTitle.Load}}}, []string{"List.Meta[0]", "Label"}},
		"a negative page":       {webui.List[story]{Loader: load, Headline: pTitle, PageSize: -1}, []string{"List.PageSize"}},
		"an href with no url":   {webui.List[story]{Loader: load, Headline: pTitle, Open: webui.Href[story]{}}, []string{"List.Open", "Href", "URL"}},
		"a url headline":        {webui.List[story]{Loader: load, Headline: webui.URL[story]{Label: "T", Load: pLink}}, []string{"List.Headline", "URL"}},
		"a badge no load":       {webui.List[story]{Loader: load, Headline: pTitle, Badge: webui.Select[story]{Label: "K"}}, []string{"List.Badge", "Load"}},
		"a badge with a store":  {webui.List[story]{Loader: load, Headline: pTitle, Badge: webui.Select[story]{Label: "K", Load: pKind.Load, Store: func(*story, []string) {}, Options: pKind.Options}}, []string{"List.Badge", "Store"}},
		"a bad origin":          {webui.List[story]{Loader: load, Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"http://x.example"}}, []string{"\"http://x.example\"", "List.TrustedOrigins[0]"}},
		"origins and no image":  {webui.List[story]{Loader: load, Headline: pTitle, TrustedOrigins: []string{"https://x.example"}}, []string{"List", "TrustedOrigins", "Image"}},
		"cards, no load":        {webui.Cards[story]{Headline: pTitle}, []string{"Cards", "Load"}},
		"an unknown layout":     {webui.Cards[story]{Loader: load, Headline: pTitle, Layout: "carousel"}, []string{"\"carousel\"", "Cards.Layout"}},
		"cards, a bad headline": {webui.Cards[story]{Loader: load}, []string{"Cards.Headline"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			refuses(t, editorErrors(t, tc.body), tc.names...)
		})
	}
}

// The link of an Open is checked as any is: it must name a page that is mounted, with the arguments that page takes.
func TestAnOpenIsCheckedLikeAnyLink(t *testing.T) {
	t.Parallel()

	elsewhere := webui.Page[webui.NoArgs]{Path: "/elsewhere", View: webui.Stack{}}
	lost := webui.Link[story, webui.NoArgs]{Page: elsewhere, Args: func(context.Context, story) webui.NoArgs { return webui.NoArgs{} }}
	got := editorErrors(t, webui.List[story]{Loader: (&listLoader{}).loader(), Headline: pTitle, Open: lost})
	assert.Contains(t, got, `a link targets "/elsewhere", which is not mounted in this app`)
}

// An Href is a destination that is an address: where the model says, as a link that opens another site in a tab of its own, and only where the address
// may be one. A script in the model is not a link, so the headline is text.
func TestAnHrefIsCheckedAtEveryItemAndALinkThatIsNotOneIsText(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h, _ := listPages(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 5, Headline: pTitle, Open: webui.Href[story]{URL: func(s story) string {
		if s.ID == "b" {
			return "javascript:alert(1)"
		}
		return pLink(s)
	}}}, nil)
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `href="https://wealthfolio.app/" target="_blank" rel="noopener noreferrer">Show HN: Wealthfolio`)
	assert.False(t, strings.Contains(body, "javascript:"))
	assert.Contains(t, body, "Nginx has moved to GitHub") // still shown, as text
}

// A click on an item's headline can be an Action, as a click on a row can: the headline is a button in a form that posts the item's key, the action is run with the item
// that was found again by that key, and an item that its Guard refuses is text.
func TestAnActionOnAnItemRunsWithTheItemThatTheKeyNames(t *testing.T) {
	t.Parallel()

	var ran []string
	open := webui.Action[story]{
		Guard: func(_ context.Context, s story) error {
			if s.ID == "b" {
				return webui.Disabled("not this one") // not the button of a row: no button for an item that it refuses, whichever error says so
			}
			return nil
		},
		Run: func(_ context.Context, s story) (webui.Outcome, error) {
			ran = append(ran, s.ID)
			return webui.Success("Opened " + s.ID), nil
		},
	}
	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Loader: src.loader(), PageSize: 5, Headline: pTitle, Open: open, Key: func(s story) string { return s.ID }})
	p := pageOf(t, h, "/admin/e")

	view, _ := p.Field("_view")
	assert.Equal(t, view.Value, "p")
	assert.Equal(t, len(p.ByRole("button", "Show HN: Wealthfolio")), 1)
	assert.True(t, page.One(t, "the headline's button", p.ByRole("button", "Show HN: Wealthfolio")).Attr("value") == "click:0:a")
	for _, f := range fieldsNamed(p, "_act") {
		assert.False(t, f.Value == "click:0:b") // refused: plain text
	}
	assert.Contains(t, p.Root().Text(), "Nginx has moved to GitHub")

	assert.Equal(t, post(h, "/admin/e", formValues("_view", "p", "_act", "click:0:a")).Code, http.StatusSeeOther)
	assert.DeepEqual(t, ran, []string{"a"})

	// What names no item that there is, or posts the Guard's refusal, runs nothing.
	for _, act := range []string{"click:0:nobody", "click:0:b", "row:0:a", "click:0:"} {
		post(h, "/admin/e", formValues("_view", "p", "_act", act))
	}
	assert.DeepEqual(t, ran, []string{"a"})
}

func TestAnActionOnAnItemIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	load := (&listLoader{}).loader()
	run := func(context.Context, story) (webui.Outcome, error) { return webui.Success("done"), nil }
	refuses(t, editorErrors(t, webui.List[story]{Loader: load, Headline: pTitle, Open: webui.Action[story]{Run: run}}), "List.Open", "Key")
	refuses(t, editorErrors(t, webui.Cards[story]{Loader: load, Headline: pTitle, Open: webui.Action[story]{Run: run}}), "Cards.Open", "Key")
	refuses(t, editorErrors(t, webui.List[story]{Loader: load, Headline: pTitle, Key: func(s story) string { return s.ID }, Open: webui.Action[story]{Run: run, Confirm: "Sure?"}}), "List.Open", "Confirm")
	refuses(t, editorErrors(t, webui.List[story]{Loader: load, Headline: pTitle, Key: func(s story) string { return s.ID }, Open: webui.Action[story]{}}), "List.Open", "Run")
}

// A list takes any kind of Loader, as a table does, and asks it for the first items: one more than it shows, to know there are more, from the start, with no sort and no filter.
func TestAListTakesAnyKindOfLoaderAndAsksForTheFirstItems(t *testing.T) {
	t.Parallel()

	shown := func(body string) int { return len(items(page.Parse(t, body))) }
	more := `Show more`

	// Every item at hand: the library shows the first of them.
	rows := editorApp(webui.List[story]{Title: "Latest", PageSize: 2, Headline: pTitle, Loader: webui.RowsLoader[story]{Load: func(context.Context) ([]story, webui.Loaded) { return stories, webui.Ready() }}})
	body := serve(rows, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, shown(body), 2)
	assert.Contains(t, body, more)

	// A source that pages: handed the window of the first items, and nothing else of a table's query.
	var asked webui.PagedQuery
	paged := editorApp(webui.List[story]{Title: "Latest", PageSize: 2, Headline: pTitle, Loader: webui.PagedLoader[story]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[story], webui.Loaded) {
		asked = q
		return webui.PagedWindow[story]{Items: stories[:min(q.Limit, len(stories))]}, webui.Ready()
	}}})
	body = serve(paged, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, shown(body), 2)
	assert.Contains(t, body, more)
	assert.Equal(t, asked.Offset, 0)
	assert.Equal(t, asked.Limit, 3)
	assert.Equal(t, asked.Sort, "")
	assert.Equal(t, len(asked.Filters), 0)

	// A source that pages by a cursor: the first page, of that size.
	var after string
	var limit int
	feed := editorApp(webui.List[story]{Title: "Latest", PageSize: 2, Headline: pTitle, Loader: webui.FeedLoader[story]{Load: func(_ context.Context, a string, n int) ([]story, string, webui.Loaded) {
		after, limit = a, n
		return stories[:min(n, len(stories))], "next", webui.Ready()
	}}})
	body = serve(feed, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, shown(body), 2)
	assert.Equal(t, after, "")
	assert.Equal(t, limit, 3)
}

// itemLinks are the links to where an item leads: the links of the items of a page, which are not the pager's.
func itemLinks(p *page.Page) []page.Link {
	var out []page.Link
	for _, li := range p.ByRole("listitem", "") {
		out = append(out, li.Links()...)
	}
	return out
}
