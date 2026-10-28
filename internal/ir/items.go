package ir

import "context"

// Items is a leaf that shows a list of items of a model, each as a headline with a line of small facts beneath it, and a picture and a badge when it has
// them: a feed, which is a column of them, or cards, which are a grid or a row that scrolls. It shows the first of what its source gives and says there
// is more when there is, and it only reads.
type Items struct {
	At Addr

	// ID names this leaf's view state in the address: "<ID>.limit" is how many it shows.
	ID string

	// Cards says each item is a card, and Scroll that the cards are in a row that scrolls and not a grid that wraps.
	Cards, Scroll bool

	Title string
	Desc  string

	// PageSize is how many items it shows at first, and how many more each Show more adds.
	PageSize int

	// Load returns at most limit items, in the order they are shown. It is asked for one more than is shown, to know whether there are more.
	Load func(ctx context.Context, limit int) ([]any, error)

	// Headline is the item's title. A DisplayLink headline is a link to elsewhere.
	Headline Field

	// Open is where the item leads in the app, when it does: a link, which is built from the item. External is where it is elsewhere, as an address that
	// has been checked, or "" for an item that has none; nil for a list that has none.
	Open     *Link
	External func(model any) string

	// Meta are the small facts under the headline, in order.
	Meta []Field

	// Image is the address of the item's picture, as the field's Get returns it; nil for none. Badge is a select, drawn beside the headline.
	Image *Field
	Badge *Field

	// TrustedOrigins are the origins a picture may be fetched from besides this site, each as markdown.Origin writes it.
	TrustedOrigins []string
}

// Addr reports the node's position in the body tree.
func (i *Items) Addr() Addr { return i.At }
