package webui

import (
	"context"
	"fmt"
	"slices"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
)

// Feed is a leaf that lists items of a model, newest first or in whatever order the source gives them: each has a headline, a line of small facts beneath it
// (its age, its author, a count, where it came from), and, when it has them, a picture and a badge. It is for the things a page says "here is what is new" about,
// where a [Table] would be a column of cells and a reader wants the headlines.
//
// It shows the first of what Load gives it, and, when there are more, a Show more link that shows PageSize more: a real link, so it works without
// script, which with script replaces the list in place. How many are shown is in the address as the leaf's name and ".limit", as a table keeps its page. Nothing is kept
// on the server. It only reads: a headline leads to where the item is, with [Feed.Open] or a [URL] headline, and a feed has no row actions.
//
// It is not [Table.Feed], which is a source for a table that pages by a cursor: a Feed is a way to show items, and its source is a plain list.
type Feed[M any] struct {
	// Title is the panel's heading, and names the leaf's view state in the address: "latest" for a Feed titled "Latest", and "feed" when it has none.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the title.
	Desc string

	// Load returns at most limit items, in the order they are shown. The limit is what is on screen plus one, so that the leaf can tell whether there
	// are more: a source that has more than it was asked for is not asked again to count. Return no items, not a failure, for a feed that is empty. If Load returns
	// [Failed] the panel says "Could not load", the cause is logged and the rest of the page stands. A feed of something that is still going returns [Loaded.RefreshAfter].
	//
	// Load is called on every request that needs the leaf's items, and the library keeps nothing between requests: if asking is costly, cache it in your app.
	Load func(ctx context.Context, limit int) ([]M, Loaded)

	// PageSize is how many items are shown at first, and how many more each Show more adds. Five when it is left zero.
	PageSize int

	// Headline is the item's title: an accessor for text, such as a [String]. It is required. It is text, and the address it leads to is Open's or Link's: a [URL],
	// whose text is the same on every row, is not a headline.
	Headline Accessor[M]

	// Open is where an item leads in the app, as a [Link] built from the item. The headline is a real anchor to it. An item whose page refuses the
	// visitor is plain text. It cannot be an [Action]: an item is read, and a link can be opened in a new tab. An item leads in the app or elsewhere, and so has Open or Link and not both.
	Open RowClick[M]

	// Link is where an item is on another site, or a path on this one: the headline is a link to it, which opens in a tab of its own, with the mark that says so
	// and rel="noopener noreferrer". An address that is not a path on this site or an http, https or mailto address, such as a script, is not a link, and the headline is text, so
	// data in the model cannot make one. Left nil, no item has one.
	Link func(M) string

	// Meta are the small facts under the headline, in order, separated by a dot: an item that has no value for one (an empty text) is shown without it.
	Meta []Accessor[M]

	// Image is the address of the item's picture, when it has one: a path on this site, which the app serves, or an https address from one of TrustedOrigins.
	// Any other address is not drawn, and the item has no picture, as one with none has not: never a broken one. It is decoration, with no text of its own, since the
	// headline names the item. Left nil, no item has one.
	Image func(M) string

	// Badge is a [Select] drawn beside the headline, such as Live or the kind of an item. Left empty, no item has one.
	Badge Select[M]

	// TrustedOrigins are the origins that serve the pictures of Image besides this site, each written as https://host or https://host:port, with nothing after it, as
	// they are for a [Markdown]. The content security policy of a page with a leaf that names origins has them in its img-src, and no other page does.
	TrustedOrigins []string
}

// CardLayout is how the cards of a [Cards] are laid out.
type CardLayout string

const (
	// CardsGrid wraps the cards into rows, as many to a row as there is room for. It is the default.
	CardsGrid CardLayout = ""

	// CardsScroll puts the cards in one row that scrolls sideways, with each card snapping into place. It can be reached and moved by the keyboard.
	CardsScroll CardLayout = "scroll"
)

// Cards is a [Feed] whose items are cards: a picture over a headline, a badge and the line of small facts, in a grid that wraps or a row that scrolls. It has the same
// source, the same Show more, and the same fields, and is for items that are looked at as much as read, such as videos or products.
type Cards[M any] struct {
	// Title, Desc, Load, PageSize, Headline, Open, Link, Meta, Image, Badge and TrustedOrigins are those of a [Feed].
	Title    string
	Desc     string
	Load     func(ctx context.Context, limit int) ([]M, Loaded)
	PageSize int

	Headline Accessor[M]
	Open     RowClick[M]
	Link     func(M) string
	Meta     []Accessor[M]

	Image          func(M) string
	Badge          Select[M]
	TrustedOrigins []string

	// Layout is a grid that wraps, which is the default, or a row that scrolls.
	Layout CardLayout
}

// ASSERT: Feed and Cards implement PageBody
var (
	_ PageBody = Feed[struct{}]{}
	_ PageBody = Cards[struct{}]{}
)

// itemsSpec is what a Feed and a Cards share, so that they are validated and lowered in one place.
type itemsSpec[M any] struct {
	what     string // "Feed" or "Cards"
	cards    bool
	scroll   bool
	title    string
	desc     string
	load     func(ctx context.Context, limit int) ([]M, Loaded)
	pageSize int

	headline Accessor[M]
	open     RowClick[M]
	link     func(M) string
	meta     []Accessor[M]
	image    func(M) string
	badge    Select[M]
	origins  []string
	layout   CardLayout
}

func (f Feed[M]) spec() itemsSpec[M] {
	return itemsSpec[M]{
		what: "Feed", title: f.Title, desc: f.Desc, load: f.Load, pageSize: f.PageSize,
		headline: f.Headline, open: f.Open, link: f.Link, meta: f.Meta, image: f.Image, badge: f.Badge, origins: f.TrustedOrigins,
	}
}

func (c Cards[M]) spec() itemsSpec[M] {
	return itemsSpec[M]{
		what: "Cards", cards: true, scroll: c.Layout == CardsScroll, layout: c.Layout, title: c.Title, desc: c.Desc, load: c.Load, pageSize: c.PageSize,
		headline: c.Headline, open: c.Open, link: c.Link, meta: c.Meta, image: c.Image, badge: c.Badge, origins: c.TrustedOrigins,
	}
}

func (f Feed[M]) validateBody(v *bodyValidator)  { f.spec().validate(v) }
func (c Cards[M]) validateBody(v *bodyValidator) { c.spec().validate(v) }

func (f Feed[M]) lowerBody(at ir.Addr) ir.Node  { return f.spec().lower(at) }
func (c Cards[M]) lowerBody(at ir.Addr) ir.Node { return c.spec().lower(at) }

func (s itemsSpec[M]) validate(v *bodyValidator) {
	if s.load == nil {
		v.add("a "+s.what+" has no Load", "Set Load to func(ctx, limit int) ([]M, Loaded), returning at most limit items.")
	}
	switch s.headline.(type) {
	case nil:
		v.add(s.what+".Headline is nil", "An item has a title: set Headline to an accessor for its text, such as a String.")
	case Group[M]:
		v.add(s.what+".Headline is a Group", "A group composes form fields; an item has one headline.")
	case URL[M]:
		v.add(s.what+".Headline is a URL", "A URL says the same on every row. Make the headline a String, and put where the item is in Link, or in Open for a page of this app.")
	default:
		validateAccessor[M](v, s.headline, s.what+".Headline", accessorSite{where: s.what + ".Headline"}, map[string]bool{})
	}
	validateAccessors(v, s.meta, accessorSite{where: s.what + ".Meta"})
	if s.pageSize < 0 {
		v.add(s.what+".PageSize is negative", "Use a number of items, or leave it zero for the default of five.")
	}
	if s.badge.Load != nil {
		validateAccessor[M](v, s.badge, s.what+".Badge", accessorSite{where: s.what + ".Badge"}, map[string]bool{})
		if s.badge.Store != nil {
			v.add(s.what+".Badge has a Store", "An item only reads: remove Store.")
		}
	} else if s.badge.Label != "" || len(s.badge.Options) > 0 || s.badge.Multi {
		v.add(s.what+".Badge has no Load", "Set Load to func(M) []string, or remove the Badge: one with no Load has none.")
	}
	if s.open != nil {
		if _, isAction := s.open.(Action[M]); isAction {
			v.add(s.what+".Open is an Action", "An item is read, and a link can be opened in a new tab: use a Link, or Link for another site.")
		} else {
			s.open.validateRow(v)
		}
	}
	if s.open != nil && s.link != nil {
		v.add(s.what+" has an Open and a Link", "An item leads in the app or elsewhere, and a link cannot hold a link: keep Open for a page of this app, or Link for an address.")
	}
	for i, o := range s.origins {
		if _, err := markdown.Origin(o); err != nil {
			v.add(fmt.Sprintf("%s.TrustedOrigins[%d], %q, is not an origin: %v", s.what, i, o, err),
				`Write it as "https://example.com", or "https://cdn.example.com:8443" for a port that is not 443.`)
		}
	}
	if len(s.origins) > 0 && s.image == nil {
		v.add(s.what+" has TrustedOrigins and no Image", "Origins are trusted to serve pictures: set Image, or remove them.")
	}
	switch s.layout {
	case CardsGrid, CardsScroll:
	default:
		v.add(fmt.Sprintf("Cards.Layout is the unknown %q", string(s.layout)), "Use CardsGrid, which wraps, or CardsScroll, a row that scrolls.")
	}
}

// defaultItems is how many items a feed shows when it does not say.
const defaultItems = 5

func (s itemsSpec[M]) lower(at ir.Addr) ir.Node {
	n := &ir.Items{
		At: at, Cards: s.cards, Scroll: s.scroll, Title: s.title, Desc: s.desc, PageSize: defaultItems,
		Headline: lowerAccessor[M](s.headline, "h"),
		Meta:     lowerAccessors(s.meta, "m"),
	}
	if s.pageSize > 0 {
		n.PageSize = s.pageSize
	}
	n.Load = func(ctx context.Context, limit int) ([]any, error) {
		rows, loaded := s.load(ctx, limit)
		if err := loaded.apply(ctx); err != nil {
			return nil, err
		}
		return convert.Any(rows), nil
	}
	if s.open != nil {
		n.Open, _ = s.open.lowerRow()
	}
	if s.link != nil {
		n.External = func(m any) string {
			if u := s.link(m.(M)); validURL(u) {
				return u
			}
			return ""
		}
	}
	if s.image != nil {
		f := head("i", "Image", ir.KindString)
		f.Get = func(m any) string { return s.image(m.(M)) }
		n.Image = &f
	}
	if s.badge.Load != nil {
		f := lowerAccessor[M](s.badge, "b")
		n.Badge = &f
	}
	for _, o := range s.origins {
		if clean, err := markdown.Origin(o); err == nil && !slices.Contains(n.TrustedOrigins, clean) {
			n.TrustedOrigins = append(n.TrustedOrigins, clean)
		}
	}
	return n
}

// itemsViewID is the name of a list of items's view state when it has no title: what the address calls it.
func itemsViewID(n *ir.Items) string {
	if base := args.Slug(n.Title); base != "" {
		return base
	}
	if n.Cards {
		return "cards"
	}
	return "feed"
}
