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

func (f *listLoader) asked() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.limits...)
}

// listPages is the page a list is on, and the page its items open, which has an argument.
func listPages(body webui.PageBody, guard func(context.Context, storyArgs) error) (http.Handler, webui.Page[storyArgs]) {
	detail := webui.Page[storyArgs]{Path: "/story/{id}", Guard: guard, Body: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{Path: "/e", Body: body}
	return webui.App{Pages: webui.Pages{list, detail}}.MustCompile("/admin"), detail
}

func openStory(detail webui.Page[storyArgs]) webui.Link[story, storyArgs] {
	return webui.Link[story, storyArgs]{Page: detail, Args: func(_ context.Context, p story) storyArgs { return storyArgs{ID: p.ID} }}
}

func TestAListIsAColumnOfItemsEachWithAHeadlineAndALineOfFacts(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{
		Title: "Latest", Load: src.load, PageSize: 3,
		Headline: pTitle, Meta: []webui.Accessor[story]{pAge, pPoints, pSource},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	assert.Contains(t, body, `<h2 class="panel-title">Latest</h2>`)
	assert.Contains(t, body, `<ul class="list">`)
	assert.Equal(t, strings.Count(body, `<li class="item">`), 3) // PageSize, and not the one more that was asked for to know there is more
	assert.Contains(t, body, `<span class="item-title">`)
	assert.Contains(t, body, "Show HN: Wealthfolio")
	assert.True(t, strings.Index(body, "Wealthfolio") < strings.Index(body, "Nginx") && strings.Index(body, "Nginx") < strings.Index(body, "Infinity")) // in the order given
	assert.False(t, strings.Contains(body, "thermonuclear"))

	// The facts are in order, with a dot between them and not after the last; a fact with no value (the third item has no source) is left out.
	first := body[strings.Index(body, `<div class="item-meta">`):]
	first = first[:strings.Index(first, "</div></div></li>")]
	assert.True(t, strings.Index(first, "<span>6h</span>") < strings.Index(first, "<span>389 points</span>") && strings.Index(first, "<span>389 points</span>") < strings.Index(first, "<span>wealthfolio.app</span>"))
	assert.Equal(t, strings.Count(first, `class="item-sep"`), 2) // between three facts
	third := body[strings.Index(body, "Infinity"):]
	third = third[:strings.Index(third, "</li>")]
	assert.Equal(t, strings.Count(third, `class="item-sep"`), 1)
	assert.False(t, strings.Contains(third, "dash"))
	assert.False(t, strings.Contains(body, "<script>"))
}

func TestASourceIsAskedForOneMoreThanIsShownAndShowMoreIsARealLinkThatAsksForMore(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 2, Headline: pTitle})

	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.DeepEqual(t, src.asked(), []int{3})
	assert.Equal(t, strings.Count(body, `<li class="item">`), 2)
	assert.Contains(t, body, `<div class="pager-info">2 shown</div>`)
	assert.Contains(t, body, `href="/admin/e?latest.limit=4"`) // the same page with more, in the address, so a copied link shows the same
	assert.Contains(t, body, `data-in-place`)                  // with script it replaces the list, and without it is a navigation
	assert.Contains(t, body, `Show more`)

	// Following it shows four, and asks for five.
	body = serve(h, http.MethodGet, "/admin/e?latest.limit=4").Body.String()
	assert.DeepEqual(t, src.asked(), []int{3, 5})
	assert.Equal(t, strings.Count(body, `<li class="item">`), 4)
	assert.Contains(t, body, `href="/admin/e?latest.limit=6"`)

	// The fifth is the last there is: the source gave no more than was asked for, so there is nothing more and no link.
	body = serve(h, http.MethodGet, "/admin/e?latest.limit=5").Body.String()
	assert.Equal(t, strings.Count(body, `<li class="item">`), 5)
	assert.False(t, strings.Contains(body, "Show more"))
	assert.False(t, strings.Contains(body, "pager-info"))
}

// The address says how many, and a hand-edited one cannot ask a source for more than a list may show, or for fewer than it shows at first.
func TestTheLimitInTheAddressIsHeldToWhatAListMayShow(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 2, Headline: pTitle})
	for _, raw := range []string{"999999", "-5", "0", "x", ""} {
		serve(h, http.MethodGet, "/admin/e?latest.limit="+raw)
	}
	assert.DeepEqual(t, src.asked(), []int{201, 3, 3, 3, 3})
}

func TestAListIsNamedInTheAddressByItsTitleAndTwoWithNoTitleAreTold(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Stack{
		webui.List[story]{Load: src.load, PageSize: 1, Headline: pTitle},
		webui.List[story]{Load: src.load, PageSize: 1, Headline: pTitle},
		webui.Cards[story]{Load: src.load, PageSize: 1, Headline: pTitle},
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

func TestAListRefreshesAsALeafOfItsOwn(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Stack{
		webui.List[story]{Title: "First", Load: src.load, PageSize: 1, Headline: pTitle},
		webui.List[story]{Title: "Second", Load: src.load, PageSize: 2, Headline: pTitle},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e?second.limit=3", nil)
	req.Header.Set("X-Webui-Leaf", "p.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Equal(t, strings.Count(body, `<li class="item">`), 3)
	assert.False(t, strings.Contains(body, "<html"))
	assert.False(t, strings.Contains(body, ">First<"))
	assert.Contains(t, body, `data-leaf="p.1"`)
}

func TestAFailingListLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Load: func(context.Context, int) ([]story, webui.Loaded) {
			return nil, webui.Failed(errors.New("store secret-bucket is gone"))
		},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "secret-bucket"))
	assert.False(t, strings.Contains(rec.Body.String(), `class="list"`))
}

func TestAListWithNoItemsSaysSoAndHasNoFoot(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Load: func(context.Context, int) ([]story, webui.Loaded) { return nil, webui.Ready() },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Nothing here yet")
	assert.False(t, strings.Contains(body, `<ul class="list">`))
	assert.False(t, strings.Contains(body, "Show more"))
}

// A list of something that is still going asks to be asked again, as any panel does.
func TestAListThatIsNotFinalIsAskedForAgain(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.List[story]{
		Title: "Latest", Headline: pTitle,
		Load: func(context.Context, int) ([]story, webui.Loaded) {
			return stories[:1], webui.Ready().RefreshAfter(5 * 1e9)
		},
	})
	assert.Contains(t, serve(h, http.MethodGet, "/admin/e").Body.String(), `data-refresh-after="5"`)
}

// ----------------------------------------------------------- headline and links

func TestAHeadlineWithALinkLeadsToAnotherSiteAndOneWithAnOpenLeadsInTheApp(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	_, detail := listPages(webui.Stack{}, nil)

	external, _ := listPages(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 1, Headline: pTitle, Link: pLink}, nil)
	body := serve(external, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<a class="item-link ext" href="https://wealthfolio.app/" target="_blank" rel="noopener noreferrer">Show HN: Wealthfolio`)
	assert.Contains(t, body, "(opens in a new tab)") // the mark that says so, for a reader

	inApp, _ := listPages(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 1, Headline: pTitle, Open: openStory(detail)}, nil)
	body = serve(inApp, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<a class="item-link" href="/admin/story/a">Show HN: Wealthfolio</a>`)
	assert.False(t, strings.Contains(body, `target="_blank"`))
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
	h, _ := listPages(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 2, Headline: pTitle, Open: openStory(detail)}, refuses)
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `href="/admin/story/a`)
	assert.False(t, strings.Contains(body, `href="/admin/story/b`))
	assert.Contains(t, body, "Nginx has moved to GitHub") // still there, as text
}

func TestABadgeIsDrawnBesideTheHeadlineAsItIsInACell(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.List[story]{Title: "Latest", Load: src.load, PageSize: 3, Headline: pTitle, Badge: pKind})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `class="badge badge-bad"`), 1) // the live one, which says Live in words and not only in red
	assert.Contains(t, body, "Live")
	assert.Equal(t, strings.Count(body, `class="item-head"`), 3)
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
				Load: func(context.Context, int) ([]story, webui.Loaded) {
					return []story{{ID: "a", Title: "X"}}, webui.Ready()
				},
				Image: func(story) string { return image },
			})
			body := serve(h, http.MethodGet, "/admin/e").Body.String()
			if tc.drawn == "" {
				assert.False(t, strings.Contains(body, "<img"))
				assert.False(t, strings.Contains(body, "evil.example"))
				assert.Contains(t, body, ">X<") // the item is still there, with no picture, as one that has none
				return
			}
			assert.Contains(t, body, `<img class="item-img" src="`+tc.drawn+`" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer">`)
		})
	}
}

func TestThePolicyOfAPageThatTrustsAnOriginForPicturesHasItAndNoOtherPageDoes(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	with := webui.Page[webui.NoArgs]{Path: "/with", Body: webui.Stack{
		webui.List[story]{Title: "A", Load: src.load, Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"https://a.example", "https://shared.example"}},
		webui.Cards[story]{Title: "B", Load: src.load, Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"https://shared.example", "https://B.example:443"}},
	}}
	without := webui.Page[webui.NoArgs]{Path: "/without", Body: webui.List[story]{Title: "C", Load: src.load, Headline: pTitle}}
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
		Title: "Videos", Load: src.load, PageSize: 2, Headline: pTitle, Meta: []webui.Accessor[story]{pAge, pSource},
		Image: func(p story) string { return "/img/" + p.ID + ".png" },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<ul class="cards">`)
	assert.Equal(t, strings.Count(body, `<li class="item item-card">`), 2)
	assert.True(t, strings.Index(body, `src="/img/a.png"`) < strings.Index(body, "Wealthfolio")) // the picture first
	assert.Contains(t, body, `href="/admin/e?videos.limit=4"`)                                   // and the same Show more
	assert.False(t, strings.Contains(body, "cards-scroller"))
}

func TestCardsInARowScrollSidewaysAndTheRowCanBeReachedByTheKeyboard(t *testing.T) {
	t.Parallel()

	src := &listLoader{}
	h := editorApp(webui.Cards[story]{Title: "Videos", Load: src.load, PageSize: 3, Headline: pTitle, Layout: webui.CardsScroll})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<div class="cards-scroller" tabindex="0" role="region" aria-label="Videos, scrolls sideways">`)
	assert.Contains(t, body, `<ul class="cards cards-row">`)
	assert.Equal(t, strings.Count(body, `<li class="item item-card">`), 3)
}

// A List and a Tabs make the widget with two views of what is new: only the selected tab is read.
func TestTabsOfListsAreReadOnlyForTheSelectedTab(t *testing.T) {
	t.Parallel()

	first, second := &listLoader{}, &listLoader{}
	h := editorApp(webui.Tabs{
		{Label: "Hacker News", Body: webui.List[story]{Title: "Hacker News", Load: first.load, Headline: pTitle}},
		{Label: "Lobsters", Body: webui.List[story]{Title: "Lobsters", Load: second.load, Headline: pTitle}},
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

	load := (&listLoader{}).load
	_, detail := listPages(webui.Stack{}, nil)
	act := webui.Action[story]{Label: "Go", Run: func(context.Context, story) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	for name, tc := range map[string]struct {
		body webui.PageBody
		want string
	}{
		"no load":               {webui.List[story]{Headline: pTitle}, "a List has no Load"},
		"no headline":           {webui.List[story]{Load: load}, "List.Headline is nil"},
		"a group headline":      {webui.List[story]{Load: load, Headline: webui.Group[story]{pTitle}}, "List.Headline is a Group"},
		"a headline no load":    {webui.List[story]{Load: load, Headline: webui.String[story]{Label: "T"}}, "List.Headline has no Load"},
		"a meta no label":       {webui.List[story]{Load: load, Headline: pTitle, Meta: []webui.Accessor[story]{webui.String[story]{Load: pTitle.Load}}}, "List.Meta[0] has no Label"},
		"a negative page":       {webui.List[story]{Load: load, Headline: pTitle, PageSize: -1}, "List.PageSize is negative"},
		"an action open":        {webui.List[story]{Load: load, Headline: pTitle, Open: act}, "List.Open is an Action"},
		"a url headline":        {webui.List[story]{Load: load, Headline: webui.URL[story]{Label: "T", Load: pLink}}, "List.Headline is a URL"},
		"open and a link":       {webui.List[story]{Load: load, Headline: pTitle, Open: openStory(detail), Link: pLink}, "List has an Open and a Link"},
		"a badge no load":       {webui.List[story]{Load: load, Headline: pTitle, Badge: webui.Select[story]{Label: "K"}}, "List.Badge has no Load"},
		"a badge with a store":  {webui.List[story]{Load: load, Headline: pTitle, Badge: webui.Select[story]{Label: "K", Load: pKind.Load, Store: func(*story, []string) {}, Options: pKind.Options}}, "List.Badge has a Store"},
		"a bad origin":          {webui.List[story]{Load: load, Headline: pTitle, Image: func(story) string { return "" }, TrustedOrigins: []string{"http://x.example"}}, `List.TrustedOrigins[0], "http://x.example", is not an origin`},
		"origins and no image":  {webui.List[story]{Load: load, Headline: pTitle, TrustedOrigins: []string{"https://x.example"}}, "List has TrustedOrigins and no Image"},
		"cards, no load":        {webui.Cards[story]{Headline: pTitle}, "a Cards has no Load"},
		"an unknown layout":     {webui.Cards[story]{Load: load, Headline: pTitle, Layout: "carousel"}, `Cards.Layout is the unknown "carousel"`},
		"cards, a bad headline": {webui.Cards[story]{Load: load}, "Cards.Headline is nil"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
}

// The link of an Open is checked as any is: it must name a page that is mounted, with the arguments that page takes.
func TestAnOpenIsCheckedLikeAnyLink(t *testing.T) {
	t.Parallel()

	elsewhere := webui.Page[webui.NoArgs]{Path: "/elsewhere", Body: webui.Stack{}}
	lost := webui.Link[story, webui.NoArgs]{Page: elsewhere, Args: func(context.Context, story) webui.NoArgs { return webui.NoArgs{} }}
	got := editorErrors(t, webui.List[story]{Load: (&listLoader{}).load, Headline: pTitle, Open: lost})
	assert.Contains(t, got, `a link targets "/elsewhere", which is not mounted in this app`)
}
