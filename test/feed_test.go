package webui_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type asked struct {
	after string
	limit int
}

// feedApp is a table of ten rows handed out four at a time by a cursor, which is the
// index of the first row of the page, and the cursors it was asked for.
func feedApp(seen *[]asked, act *[]string) http.Handler {
	tbl := webui.Table[Device]{
		Title:    "Objects",
		PageSize: 4,
		Feed: func(_ context.Context, after string, limit int) ([]Device, string, error) {
			*seen = append(*seen, asked{after, limit})
			start, _ := strconv.Atoi(after)
			var rows []Device
			for i := start; i < min(start+limit, 10); i++ {
				rows = append(rows, Device{Id: fmt.Sprintf("o%d", i)})
			}
			if start+limit >= 10 {
				return rows, "", nil
			}
			return rows, strconv.Itoa(start + limit), nil
		},
		Key:     func(d Device) string { return d.Id },
		Columns: []webui.Accessor[Device]{formID},
		Actions: []webui.Action[Device]{{Label: "Open", Run: func(_ context.Context, d Device) (webui.Outcome, error) {
			*act = append(*act, d.Id)
			return webui.Success("done"), nil
		}}},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: tbl}}}.MustCompile("")
}

// pagerHref is the address a footer button of a page leads to, "" when it is disabled.
func pagerHref(body, label string) string {
	if strings.Contains(body, `disabled>`+label+`</button>`) {
		return ""
	}
	m := regexp.MustCompile(`href="([^"]*)"[^>]*>` + label + `</a>`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return html.UnescapeString(m[1])
}

func TestAFeedIsHandedACursorAndAPageSizeAndPagesByNextPreviousAndFirstPage(t *testing.T) {
	t.Parallel()

	var seen []asked
	var act []string
	h := feedApp(&seen, &act)

	first := serve(h, http.MethodGet, "/f").Body.String()
	assert.DeepEqual(t, seen[0], asked{"", 4})
	assert.Contains(t, first, ">o0<")
	assert.Contains(t, first, ">o3<")
	assert.False(t, strings.Contains(first, ">o4<"))
	assert.Contains(t, first, "Showing rows 1–4")       // which rows, not only how many
	assert.Equal(t, pagerHref(first, "First page"), "") // on it already
	assert.Equal(t, pagerHref(first, "Previous"), "")   // nothing before it

	// Next carries the cursor, where the next page starts, and the page it leaves.
	toSecond := pagerHref(first, "Next")
	assert.Equal(t, toSecond, "/f?objects.after=4&objects.back=1%3A&objects.row=5")
	second := serve(h, http.MethodGet, toSecond).Body.String()
	assert.DeepEqual(t, seen[1], asked{"4", 4}) // the source is handed the cursor and the size, and no more
	assert.Contains(t, second, ">o4<")
	assert.Contains(t, second, "Showing rows 5–8")
	assert.Equal(t, pagerHref(second, "First page"), "/f")
	assert.Equal(t, pagerHref(second, "Previous"), "/f") // back to the first page, which has no cursor

	third := serve(h, http.MethodGet, pagerHref(second, "Next")).Body.String()
	assert.Contains(t, third, ">o9<")
	assert.Contains(t, third, "Showing rows 9–10")
	assert.Equal(t, pagerHref(third, "Next"), "") // no cursor came back: the last page

	// Previous goes to the page before, and from there on again, as far back as it goes.
	backToSecond := pagerHref(third, "Previous")
	assert.Equal(t, backToSecond, toSecond)
	assert.Contains(t, serve(h, http.MethodGet, backToSecond).Body.String(), "Showing rows 5–8")
}

func TestAFeedAddressWithNoRowOrTrailStillPagesAndSaysHowManyRows(t *testing.T) {
	t.Parallel()

	var seen []asked
	var act []string
	h := feedApp(&seen, &act)

	// An address made by hand has a cursor and nothing else: which row it starts at and
	// the way back are not known.
	body := serve(h, http.MethodGet, "/f?objects.after=4").Body.String()
	assert.Contains(t, body, "4 rows")
	assert.Equal(t, pagerHref(body, "Previous"), "")
	assert.Equal(t, pagerHref(body, "First page"), "/f")
	assert.Equal(t, pagerHref(body, "Next"), "/f?objects.after=8&objects.back=0%3A4")
	// Garbage in the trail or the row is no trail and no row.
	junk := serve(h, http.MethodGet, "/f?objects.after=4&objects.row=-3&objects.back=x&objects.back=NaN:y").Body.String()
	assert.Contains(t, junk, "4 rows")
	assert.Equal(t, pagerHref(junk, "Previous"), "")
}

func TestAFeedForgetsTheOldestPagesWhenTheWayBackIsTooLongForTheAddress(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("c", 1000)
	tbl := webui.Table[Device]{
		Title:    "Objects",
		PageSize: 1,
		Feed: func(_ context.Context, after string, _ int) ([]Device, string, error) {
			n := 0
			if after != "" {
				n, _ = strconv.Atoi(after[:3])
			}
			return []Device{{Id: "x"}}, fmt.Sprintf("%03d", n+1) + long, nil
		},
		Columns: []webui.Accessor[Device]{formID},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: tbl}}}.MustCompile("")

	at := "/f"
	var hops int
	for range 12 {
		next := pagerHref(serve(h, http.MethodGet, at).Body.String(), "Next")
		assert.True(t, len(next) < 8000) // an address that browsers and servers accept
		at = next
		hops++
	}
	body := serve(h, http.MethodGet, at).Body.String()
	assert.True(t, pagerHref(body, "Previous") != "")         // it still goes back
	assert.True(t, pagerHref(body, "First page") != "")       // and the start is always there
	assert.True(t, strings.Count(at, "objects.back=") < hops) // but not all the way
}

func TestAFeedHasNoSortLinksOrFiltersAndIgnoresTheirAddressParameters(t *testing.T) {
	t.Parallel()

	var seen []asked
	var act []string
	h := feedApp(&seen, &act)
	body := serve(h, http.MethodGet, "/f?objects.sort=id&objects.desc=true&objects.filter.id=zzz&objects.offset=3").Body.String()

	assert.Contains(t, body, `<th class=""><span class="th-in">ID </span></th>`) // a plain header: no sort link, no filter
	assert.False(t, strings.Contains(body, `class="filter`))
	assert.False(t, strings.Contains(body, "aria-sort"))
	assert.DeepEqual(t, seen[0], asked{"", 4}) // the first page, whatever was asked of it
	assert.Contains(t, body, ">o0<")
}

func TestAFeedsCursorIsEscapedInTheAddressAndALongOneIsAFirstPage(t *testing.T) {
	t.Parallel()

	var got []string
	tbl := webui.Table[Device]{
		Title: "Objects",
		Feed: func(_ context.Context, after string, _ int) ([]Device, string, error) {
			got = append(got, after)
			return []Device{{Id: "x"}}, "a b&c=d/é", nil
		},
		Columns: []webui.Accessor[Device]{formID},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: tbl}}}.MustCompile("")

	body := serve(h, http.MethodGet, "/f").Body.String()
	assert.Contains(t, body, `href="/f?objects.after=a+b%26c%3Dd%2F%C3%A9&amp;objects.back=1%3A&amp;objects.row=2">Next`)
	serve(h, http.MethodGet, "/f?objects.after="+url.QueryEscape("a b&c=d/é"))
	assert.Equal(t, got[len(got)-1], "a b&c=d/é") // it comes back as it went

	serve(h, http.MethodGet, "/f?objects.after="+strings.Repeat("x", 3000))
	assert.Equal(t, got[len(got)-1], "") // too long to be one of ours
}

func TestARowActionOfAFeedActsOnTheRowOfThePageItWasClickedOn(t *testing.T) {
	t.Parallel()

	var seen []asked
	var act []string
	h := feedApp(&seen, &act)

	// The page at cursor 4 holds o4 to o7; the form posts to that address.
	rec := post(h, "/f?objects.after=4", formValues("_act", "row:0:o5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, act, []string{"o5"})
	assert.Equal(t, post(h, "/f?objects.after=4", formValues("_act", "row:0:o1")).Code, http.StatusSeeOther) // not there: refused with a message
	assert.DeepEqual(t, act, []string{"o5"})
}

func TestAFailingFeedFailsThePanelAndTheCauseIsNotShown(t *testing.T) {
	t.Parallel()

	tbl := webui.Table[Device]{
		Feed: func(context.Context, string, int) ([]Device, string, error) {
			return nil, "", errors.New("bucket secret-name is gone")
		},
		Columns: []webui.Accessor[Device]{formID},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: tbl}}}.MustCompile("")
	body := serve(h, http.MethodGet, "/f").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-name"))
}

func TestATableTakesExactlyOneSourceAndAFeedCannotBeSearched(t *testing.T) {
	t.Parallel()

	feed := func(context.Context, string, int) ([]Device, string, error) { return nil, "", nil }
	problems := func(tbl webui.Table[Device]) string {
		tbl.Columns = []webui.Accessor[Device]{formID}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: tbl}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	assert.Contains(t, problems(webui.Table[Device]{}), "none of Rows, Load and Feed")
	assert.Contains(t, problems(webui.Table[Device]{Feed: feed, Load: okRows}), "more than one of Rows, Load and Feed")
	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	searched := webui.Table[Device]{Feed: feed, Search: true, RowClick: webui.Link[Device, detailsArgs]{Page: details, Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} }}}
	assert.Contains(t, problems(searched), "rows come from Feed")
}
