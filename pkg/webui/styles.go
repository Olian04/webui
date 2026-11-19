package webui

import "github.com/Olian04/webui/internal/render"

// Styles are the addresses of the stylesheets that make a page of your own look as the app does, for a page that the app makes itself, such as the page in an [Embed]: a classless stylesheet,
// and the app's theme when it has one.
//
// Classless means that it styles elements and nothing else: headings, paragraphs, lists, links, quotes, code, tables and forms, the page's ground (the panel's colour, since the page is shown in a panel) and text, and the app's scrollbars and focus
// ring. A page writes plain semantic HTML and looks like the app. No class name is promised, because there is none in it, so the library is free to change how an element looks. What is promised
// is that, and the names of these custom properties, which the page may read in its own CSS (an SVG's fill, a border of its own) and which follow the visitor's colour scheme and the app's theme:
// --surface-color --field-color (the grounds: a panel's, and an input's), --border-color --border-strong-color --control-border-color (lines), --text-color --text-muted-color --text-faint-color (text),
// --hover-color, --accent-color --accent-text-color (the accent as a fill and as text), --success-color --warning-color --danger-color each with a -bg-color and a -border-color (status: text, tint
// and edge), --focus-ring-color, --radius --radius-pill and --control-height. Each is named by what it colours, and is in the stylesheet because the stylesheet itself draws with it or a page of
// plain markup would want it; the app's own pages have more, which are not offered.
//
//	for _, href := range app.Styles("/admin") {
//		fmt.Fprintf(w, `<link rel="stylesheet" href="%s">`, href)
//	}
//
// The page follows the visitor's system colour scheme (it can read the choice that the visitor made with the app's switch, which is kept in localStorage under "webui.theme", and set it as
// data-theme on its html element).
//
// prefix is the one that the app was compiled with, since an App does not know where it is mounted: "" or a path such as "/admin". The addresses are the ones that the app serves, with a hash of
// the contents in the name, so a browser keeps them for good. They are under the app's mount, so a visitor that the mount's sign-in refuses cannot load them. The page's own policy must allow
// them: style-src 'self'. The icons are not here: the font is cut for the icons that the app names, which is not a font for any other page. It returns nothing for a prefix that [App.Compile]
// would refuse.
func (a App) Styles(prefix string) []string {
	if len(validatePrefix(prefix)) > 0 {
		return nil
	}
	return render.StyleHrefs(prefix, lowerTheme(a.Theme))
}
