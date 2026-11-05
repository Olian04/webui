package webui

import "time"

// Observer is told what an app does, so that the program can count and time it: with Prometheus, by the [github.com/Olian04/webui/pkg/prom] package, or with whatever else it
// uses, by an Observer of its own. It is told, and it only observes: nothing it does changes an answer, and the app serves none of what it collects. Set it as App.Observer; left nil, an app
// counts nothing. Its methods are called from many requests at once, so an Observer must be safe for concurrent use, and they are called as the app works, so they should not block.
//
// Every page is named by its path template, "/device/{id}", and never by the address that was asked for, so that what an Observer keys by does not grow with the data. A kind or a
// result is one of the few fixed names that are listed on the method that is given it.
type Observer interface {
	// Begin is called when a request is received, and returns what ends it, which is called with the page that answered (its path template), the method, the status
	// that the handler wrote (200 for a handler that wrote none), and when the request was received. It is called for every request to a page, and not for the assets.
	Begin() func(page, method string, status int, started time.Time)

	// Load is called when one of the application's own Load functions has run: a panel of a kind (form, editor, table, diff, markdown, options, export, or when,
	// the condition of a [When]) on a page, when it was started, and the error that it ended with, nil for none. A visitor who left makes a Load fail with
	// [context.Canceled], which is not the application's fault.
	Load(page, kind string, started time.Time, err error)

	// Action is called when an action has been answered, with what it came to: success, warning, failure, reject (a field was refused), invalid (the rules refused it before it
	// ran), denied (its Guard refused the visitor) or error (its Run failed).
	Action(page, result string)

	// Export is called when a table has been exported, in a format that is csv or json.
	Export(page, format string)

	// Search is called when a search has been answered, with when it was started.
	Search(started time.Time)

	// SearchFailed is called for a page whose Search failed.
	SearchFailed(page string)

	// NavFailed is called for a page whose NavEntries failed.
	NavFailed(page string)
}
