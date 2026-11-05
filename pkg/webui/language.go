package webui

// Language is the language of the text in an [Editor] or a [Diff]: it decides how the
// text is highlighted, and, for an [Editor] that can be edited, which language service
// it may offer. The values are Monaco's own language ids, and there is a constant for each language the editor
// highlights in the package [github.com/Olian04/webui/pkg/lang]: lang.Go, lang.JSON, lang.YAML,
// lang.Markdown and so on. The zero value is plain text. A Language that the editor does not highlight is refused when the app is compiled.
type Language string

// service is the language service that covers a language, or "". There are four, each
// an optional package of its own under pkg/monaco, because each carries a worker of
// up to several megabytes: JSON, CSS (with SCSS and Less), HTML (with Handlebars and
// Razor), and TypeScript (with JavaScript). Every other language is highlighted only.
func (l Language) service() string {
	switch l {
	case "json":
		return "json"
	case "css", "scss", "less":
		return "css"
	case "html", "handlebars", "razor":
		return "html"
	case "typescript", "javascript":
		return "ts"
	}
	return ""
}

// id is Monaco's name for the language.
func (l Language) id() string {
	if l == "" {
		return "plaintext"
	}
	return string(l)
}

func (l Language) known() bool { return l == "" || languages[l] }
