package webui

// Language is the language of the text in an [Editor] or a [Diff]: it decides how the
// text is highlighted, and, for an [Editor] that can be edited, which language service
// it may offer. The values are constants, one for each language the editor
// highlights, named Lang followed by the language: [LangGo], [LangJSON], [LangYAML],
// [LangMarkdown] and so on. The zero value is plain text.
type Language string

// service is the language service that covers a language, or "". There are four, each
// an optional package of its own under pkg/monaco, because each carries a worker of
// up to several megabytes: JSON, CSS (with SCSS and Less), HTML (with Handlebars and
// Razor), and TypeScript (with JavaScript). Every other language is highlighted only.
func (l Language) service() string {
	switch l {
	case LangJSON:
		return "json"
	case LangCSS, LangSCSS, LangLess:
		return "css"
	case LangHTML, LangHandlebars, LangRazor:
		return "html"
	case LangTypeScript, LangJavaScript:
		return "ts"
	}
	return ""
}

// id is Monaco's name for the language.
func (l Language) id() string {
	if l == "" {
		return string(LangPlainText)
	}
	return string(l)
}

func (l Language) known() bool { return l == "" || languages[l] }
