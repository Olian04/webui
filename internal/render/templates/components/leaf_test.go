package components

import (
	"bytes"
	"context"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// Icon and NavItem are Go and not templ components, for the cost of a component that has nothing to say that a string does not. What they write is what the templ components
// wrote, to the byte, and these are what those wrote, kept: the class of an icon is made by templ's rule (each of "fa-solid", the icon's and the extra is one item that is never split,
// and one that is the same as an earlier one, as a whole, is left out), the title, the label and the address are escaped, and an address that is not one a browser may follow is the one
// templ makes of it.

func rendered(t *testing.T, render func(context.Context, *bytes.Buffer) error) string {
	t.Helper()

	var b bytes.Buffer
	assert.NoError(t, render(context.Background(), &b))
	return b.String()
}

func TestAnIconIsWhatTheTemplComponentWas(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		props IconProps
		want  string
	}{
		{IconProps{Name: "house", Class: "", Title: ""}, "<i class=\"fa-solid fa-house\" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "ni", Title: ""}, "<i class=\"fa-solid fa-house ni\" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "ni menu-icon", Title: ""}, "<i class=\"fa-solid fa-house ni menu-icon\" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "  spaced   out ", Title: ""}, "<i class=\"fa-solid fa-house   spaced   out \" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "fa-solid", Title: ""}, "<i class=\"fa-solid fa-house\" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "fa-house x fa-house", Title: ""}, "<i class=\"fa-solid fa-house fa-house x fa-house\" aria-hidden=\"true\"></i>"}, //nolint:dupword // the case is a class that is repeated
		{IconProps{Name: "house", Class: "a a b", Title: ""}, "<i class=\"fa-solid fa-house a a b\" aria-hidden=\"true\"></i>"},                             //nolint:dupword // the case is a class that is repeated
		{IconProps{Name: "house", Class: "", Title: "Home"}, "<i class=\"fa-solid fa-house\" role=\"img\" aria-label=\"Home\" title=\"Home\"></i>"},
		{IconProps{Name: "house", Class: "ni", Title: "A \"quoted\" <b> & 'x'"}, "<i class=\"fa-solid fa-house ni\" role=\"img\" aria-label=\"A &#34;quoted&#34; &lt;b&gt; &amp; &#39;x&#39;\" title=\"A &#34;quoted&#34; &lt;b&gt; &amp; &#39;x&#39;\"></i>"},
		{IconProps{Name: "", Class: "", Title: ""}, ""},
		{IconProps{Name: "", Class: "ni", Title: "t"}, ""},
		{IconProps{Name: "a<b", Class: "", Title: ""}, "<i class=\"fa-solid fa-a&lt;b\" aria-hidden=\"true\"></i>"},
		{IconProps{Name: "house", Class: "x\"y <z>", Title: ""}, "<i class=\"fa-solid fa-house x&#34;y &lt;z&gt;\" aria-hidden=\"true\"></i>"},
	} {
		assert.Equal(t, rendered(t, func(ctx context.Context, b *bytes.Buffer) error { return Icon(c.props).Render(ctx, b) }), c.want)
	}
}

func TestANavItemIsWhatTheTemplComponentWas(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		props NavItemProps
		want  string
	}{
		{NavItemProps{Label: "Devices", Href: "/admin/device", Icon: "display", Letter: "", Active: false}, "<a class=\"nav-item\" href=\"/admin/device\" title=\"Devices\"><i class=\"fa-solid fa-display ni\" aria-hidden=\"true\"></i><span>Devices</span></a>"},
		{NavItemProps{Label: "Devices", Href: "/admin/device", Icon: "display", Letter: "", Active: true}, "<a class=\"nav-item active\" href=\"/admin/device\" title=\"Devices\" aria-current=\"page\"><i class=\"fa-solid fa-display ni\" aria-hidden=\"true\"></i><span>Devices</span></a>"},
		{NavItemProps{Label: "Sites", Href: "/admin/site", Icon: "", Letter: "S", Active: false}, "<a class=\"nav-item\" href=\"/admin/site\" title=\"Sites\"><span class=\"ni ni-letter\" aria-hidden=\"true\">S</span> <span>Sites</span></a>"},
		{NavItemProps{Label: "Sites", Href: "/admin/site", Icon: "", Letter: "S", Active: true}, "<a class=\"nav-item active\" href=\"/admin/site\" title=\"Sites\" aria-current=\"page\"><span class=\"ni ni-letter\" aria-hidden=\"true\">S</span> <span>Sites</span></a>"},
		{NavItemProps{Label: "Plain", Href: "/admin/plain", Icon: "", Letter: "", Active: false}, "<a class=\"nav-item\" href=\"/admin/plain\" title=\"Plain\"><span>Plain</span></a>"},
		{NavItemProps{Label: "A \"q\" <b> & 'x'", Href: "/admin/a?x=1&y=\"2\"", Icon: "bell", Letter: "A", Active: false}, "<a class=\"nav-item\" href=\"/admin/a?x=1&amp;y=&#34;2&#34;\" title=\"A &#34;q&#34; &lt;b&gt; &amp; &#39;x&#39;\"><i class=\"fa-solid fa-bell ni\" aria-hidden=\"true\"></i><span>A &#34;q&#34; &lt;b&gt; &amp; &#39;x&#39;</span></a>"},
		{NavItemProps{Label: "Ünï", Href: "/admin/ü", Icon: "", Letter: "Ü", Active: true}, "<a class=\"nav-item active\" href=\"/admin/ü\" title=\"Ünï\" aria-current=\"page\"><span class=\"ni ni-letter\" aria-hidden=\"true\">Ü</span> <span>Ünï</span></a>"},
		{NavItemProps{Label: "JS", Href: "javascript:alert(1)", Icon: "bell", Letter: "", Active: false}, "<a class=\"nav-item\" href=\"about:invalid#TemplFailedSanitizationURL\" title=\"JS\"><i class=\"fa-solid fa-bell ni\" aria-hidden=\"true\"></i><span>JS</span></a>"},
		{NavItemProps{Label: "Data", Href: "data:text/html,<b>", Icon: "", Letter: "D", Active: false}, "<a class=\"nav-item\" href=\"about:invalid#TemplFailedSanitizationURL\" title=\"Data\"><span class=\"ni ni-letter\" aria-hidden=\"true\">D</span> <span>Data</span></a>"},
		{NavItemProps{Label: "Empty", Href: "", Icon: "bell", Letter: "", Active: true}, "<a class=\"nav-item active\" href=\"\" title=\"Empty\" aria-current=\"page\"><i class=\"fa-solid fa-bell ni\" aria-hidden=\"true\"></i><span>Empty</span></a>"},
		{NavItemProps{Label: "", Href: "/admin/x", Icon: "bell", Letter: "", Active: false}, "<a class=\"nav-item\" href=\"/admin/x\" title=\"\"><i class=\"fa-solid fa-bell ni\" aria-hidden=\"true\"></i><span></span></a>"},
		{NavItemProps{Label: "Space", Href: " /admin/a b", Icon: "", Letter: "S", Active: false}, "<a class=\"nav-item\" href=\" /admin/a b\" title=\"Space\"><span class=\"ni ni-letter\" aria-hidden=\"true\">S</span> <span>Space</span></a>"},
	} {
		assert.Equal(t, rendered(t, func(ctx context.Context, b *bytes.Buffer) error { return NavItem(c.props).Render(ctx, b) }), c.want)
	}
}
