package webui

import "github.com/Olian04/webui/internal/render"

// Styles are the addresses of the stylesheets that the library's own pages link, in the order they are linked, for a page that the app makes itself, such as the page in an [Embed]: the
// library's stylesheet, and the app's theme when it has one. A page that links them looks as the app does, in its colours, and follows the visitor's system colour scheme (a page of the same
// site can read the choice that the visitor made with the app's switch, which is kept in localStorage under "webui.theme", and set it as data-theme on its html element).
//
//	for _, href := range app.Styles("/admin") {
//		fmt.Fprintf(w, `<link rel="stylesheet" href="%s">`, href)
//	}
//
// prefix is the one that the app was compiled with, since an App does not know where it is mounted: "" or a path such as "/admin". The addresses are the ones that the app's own pages carry and
// that it serves, with a hash of the contents in the name, so a browser keeps them for good. They are under the app's mount, so a visitor that the mount's sign-in refuses cannot load them. The
// page's own policy must allow them: style-src 'self'. The icons are not here: the font is cut for the icons that the app names, which is not a font for any other page. It returns nothing for
// a prefix that [App.Compile] would refuse.
func (a App) Styles(prefix string) []string {
	if len(validatePrefix(prefix)) > 0 {
		return nil
	}
	return render.StyleHrefs(prefix, lowerTheme(a.Theme))
}
