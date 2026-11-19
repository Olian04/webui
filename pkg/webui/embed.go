package webui

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
)

// Embed is a panel that shows a page of this site in a frame: a map, a scene, a visualisation, anything that the library has no view for and that the app can make as a page of its own.
// It is the way out for what a panel cannot be, and it asks nothing of script: the frame is plain markup, and whether the page in it uses script, or has a version for a reader with
// none, is the app's to say. It is only for pages of this site; it does not show another one.
//
//	webui.Embed{Title: "Site map", Load: func(ctx context.Context) (string, webui.Loaded) {
//		return "/frame/device/" + webui.ArgsOf[DeviceArgs](ctx).ID, webui.Ready()
//	}}
//
// Load is asked for each request and returns the address, which may be built from the page's arguments with [ArgsOf] and from who the visitor is: a page for each device, at
// "/frame/device/" and its id. A page that does not change is a Load that returns its path. Its [Loaded] says, as for any panel, that it failed, that it is not final, or that the visitor
// may not be told. An address that is not a path on this site is a failure of the app's, which is logged and not shown.
//
// The page in the frame is the app's own code, from the app's own site, and is as trusted as any route of the app is: the frame is not sandboxed, so the page has the visitor's cookies and
// storage, can ask the app's other routes for what it draws, and can reach the page that holds it. It is a route of the app's own, outside the pages of this app: those are not for a frame, since
// every page of the app says that no page may frame it. A route of the app's own says so in its own words, and should say that only this site may: Content-Security-Policy: frame-ancestors 'self'.
// It is a request like any, so it checks who the visitor is as any route does: the Guard of the page that holds the frame does not protect it. What the page loads, a library from another
// site or a worker, is allowed by its own policy and not by this app's, which is the reason for a page of its own.
//
// A page that is replaced when the panel is, by a refresh or a tab, is loaded again, and so is the frame, with what it held.
type Embed struct {
	// Title names the panel, and the frame to a reader.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the title.
	Desc string

	// HeadActions are buttons and links in the panel's head, at the right of its title, for the panel as a whole: an [Action] or a [Link] of [NoArgs], as in a
	// [Toolbar], and with the same Guard, Confirm and Role. They are not a menu: each is drawn, and an app that has more than a few has a Toolbar.
	HeadActions []ToolbarItem

	// Load returns the path of the page to show, such as "/frame/status", for the request being answered.
	Load func(ctx context.Context) (string, Loaded)
}

// ASSERT: Embed implements View
var _ View = Embed{}

func (e Embed) validateView(v *viewValidator) {
	validateHeadActions(v, "Embed", e.HeadActions)
	if e.Title == "" {
		v.add("an Embed has no Title", "Set Title: it is the heading of the panel and the name of the frame to a reader.")
	}
	needLoad(v, "an Embed", e.Load != nil)
}

func (e Embed) lowerView(at ir.Pos) ir.Node {
	out := &ir.Embed{At: at, Title: e.Title, Desc: e.Desc, Load: eraseLoad(e.Load)}
	out.HeadActions = lowerHeadActions(at, e.HeadActions)
	return out
}

// samePath is whether an address is a path on this site: it begins with one slash, and is not an address on another site or a path that a browser reads as one.
func samePath(s string) bool { return ir.SamePath(s) }
