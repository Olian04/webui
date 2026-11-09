package components

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// Icon and NavItem are Go and not templ components, for the cost of a component that has nothing to say that a string does not. What they say is read here as a
// browser reads it: an icon is an element that has the classes of the font and of the caller and is hidden from a reader or says what it is, a link is where
// its address says and is called what its label says, and nothing that is written in a title, a label or an address is anything but text.

func rendered(t *testing.T, render func(context.Context, *bytes.Buffer) error) *page.Page {
	t.Helper()

	var b bytes.Buffer
	assert.NoError(t, render(context.Background(), &b))
	return page.Parse(t, b.String())
}

func drawnIcon(t *testing.T, props IconProps) *page.Page {
	t.Helper()
	return rendered(t, func(ctx context.Context, b *bytes.Buffer) error { return Icon(props).Render(ctx, b) })
}

func classes(e page.El) []string { return strings.Fields(e.Attr("class")) }

func TestAnIconHasTheClassesOfTheFontAndOfTheCaller(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		props IconProps
		want  []string
	}{
		"the icon":          {IconProps{Name: "house"}, []string{"fa-solid", "fa-house"}},
		"and a class":       {IconProps{Name: "house", Class: "ni"}, []string{"fa-solid", "fa-house", "ni"}},
		"and two":           {IconProps{Name: "house", Class: "ni menu-icon"}, []string{"fa-solid", "fa-house", "ni", "menu-icon"}},
		"spaced out":        {IconProps{Name: "house", Class: "  spaced   out "}, []string{"fa-solid", "fa-house", "spaced", "out"}},
		"a hostile name":    {IconProps{Name: "a<b"}, []string{"fa-solid", "fa-a<b"}},
		"a hostile class":   {IconProps{Name: "house", Class: `x"y <z>`}, []string{"fa-solid", "fa-house", `x"y`, "<z>"}},
		"one of the font's": {IconProps{Name: "house", Class: "fa-solid"}, nil}, // the class of the font is the font's, and the caller's is not drawn twice
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			icon := page.One(t, "the icon", drawnIcon(t, tc.props).ByAttr("class", ""))
			got := classes(icon)
			if tc.want == nil {
				assert.Equal(t, strings.Count(strings.Join(got, " "), "fa-solid"), 1)
				return
			}
			assert.DeepEqual(t, got, tc.want)
		})
	}
}

func TestAnIconIsHiddenFromAReaderUnlessItHasATitle(t *testing.T) {
	t.Parallel()

	hidden := page.One(t, "the icon", drawnIcon(t, IconProps{Name: "house"}).ByAttr("class", ""))
	assert.True(t, hidden.Hidden())

	for _, title := range []string{"Home", `A "quoted" <b> & 'x'`} {
		named := page.One(t, "the icon", drawnIcon(t, IconProps{Name: "house", Class: "ni", Title: title}).ByRole("img", title))
		assert.Equal(t, named.Attr("title"), title) // and whatever it holds is text
		assert.False(t, named.Hidden())
	}

	// With no name there is no icon.
	assert.Equal(t, len(drawnIcon(t, IconProps{Class: "ni", Title: "t"}).ByAttr("class", "")), 0)
}

func navOf(t *testing.T, props NavItemProps) page.El {
	t.Helper()

	p := rendered(t, func(ctx context.Context, b *bytes.Buffer) error { return NavItem(props).Render(ctx, b) })
	return page.One(t, "the entry", p.ByAttr("href", ""))
}

func TestANavItemIsALinkThatIsCalledWhatItsLabelSaysAndLeadsWhereItsAddressSays(t *testing.T) {
	t.Parallel()

	for _, label := range []string{"Devices", `A "q" <b> & 'x'`, "Ünï", ""} {
		entry := navOf(t, NavItemProps{Label: label, Href: "/admin/a?x=1&y=\"2\"", Icon: "bell", Letter: "A"})
		assert.Equal(t, entry.Attr("href"), "/admin/a?x=1&y=\"2\"")
		assert.Equal(t, entry.Attr("title"), label)
		assert.Equal(t, entry.Name(), label)
		assert.False(t, entry.Has("aria-current"))
	}
	assert.Equal(t, navOf(t, NavItemProps{Label: "Devices", Href: "/admin/device", Active: true}).Attr("aria-current"), "page")
}

func TestANavItemHasAnIconOrItsLetterAndNeitherIsItsName(t *testing.T) {
	t.Parallel()

	withIcon := navOf(t, NavItemProps{Label: "Devices", Href: "/x", Icon: "display", Letter: "D"})
	assert.True(t, len(withIcon.ByAttr("class", "")) > 0) // the icon is drawn
	assert.Equal(t, withIcon.AllText(), "Devices")        // and the letter is not

	withLetter := navOf(t, NavItemProps{Label: "Sites", Href: "/x", Letter: "S"})
	assert.Equal(t, withLetter.AllText(), "S Sites") // the letter stands where the icon would
	assert.Equal(t, withLetter.Name(), "Sites")      // and a reader is not told it

	plain := navOf(t, NavItemProps{Label: "Plain", Href: "/x"})
	assert.Equal(t, plain.AllText(), "Plain")
}

// An address that a browser would run, and not follow, is not one that an entry leads to.
func TestANavItemDoesNotLeadToAddressThatWouldRunSomething(t *testing.T) {
	t.Parallel()

	for _, href := range []string{"javascript:alert(1)", "data:text/html,<b>", "JaVaScRiPt:x"} {
		entry := navOf(t, NavItemProps{Label: "x", Href: href})
		assert.False(t, strings.HasPrefix(strings.ToLower(entry.Attr("href")), "javascript:"))
		assert.False(t, strings.HasPrefix(strings.ToLower(entry.Attr("href")), "data:"))
	}
}
