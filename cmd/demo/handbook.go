package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// Handbook is a text the application has, which is shown as it is written.
type Handbook struct{ Text string }

//go:embed handbook/*.md
var handbookFiles embed.FS

// A Markdown shows one text of a model, set as markdown, and only reads: its accessor has no
// Store. Raw HTML in it is never rendered, a link goes only to an http or https address, a
// mailto, or a path in the application, and an image is a link to the picture, so a text from
// outside is safe to show. A link such as [Config](/config) is an address in the application
// whatever it is mounted at: it is given the mount's prefix, and opens in the same tab. A
// fenced block that names a language is coloured by the editor the page then loads; without
// script it is plain text in a block.
//
// The texts are files that are embedded, one per page, and link to each other and to the
// pages of the demo.
//
// A picture is a link to it unless its origin is one the author trusts. These pages trust two,
// which is why the glossary's screenshot and badge are drawn and why the browser is allowed
// to fetch them: the content security policy of these pages, and of no other, has them in
// its img-src. A picture from anywhere else, such as the tracking pixel in the glossary, is
// only a link.
var (
	HandbookText = webui.String[Handbook]{
		Label: "Handbook",
		Load:  func(h Handbook) string { return h.Text },
	}

	HandbookRunbook = handbookPage("/handbook", "Runbook", "book-open", "runbook.md",
		"A Markdown: text set as markdown, read only. Raw HTML is never rendered, and a link to a path in the application stays in it.")
	HandbookIncidents = handbookPage("/handbook/incidents", "Incidents", "triangle-exclamation", "incidents.md",
		"Links to the alerts, the devices and the config, each at an address in the application.")
	HandbookSinks = handbookPage("/handbook/sinks", "Sinks", "plug", "sinks.md",
		"A link may carry a query, as /device/dev_27c38b?minutes=15 does, and goes where it says.")
	HandbookGlossary = handbookPage("/handbook/glossary", "Glossary", "spell-check", "glossary.md",
		"A link outside the application opens in a tab of its own, and an unsafe one is not a link.")
)

// handbookPage is a page of the handbook: the text of a file, in a Markdown.
func handbookPage(path webui.PageID[webui.NoArgs], label, icon, file, desc string) webui.Page[webui.NoArgs] {
	return webui.Page[webui.NoArgs]{
		Path: path,
		Nav:  webui.Nav{Label: label, Icon: icon, Section: "Handbook"},
		Body: handbookPanel("Handbook", file, desc),
	}
}

// handbookPanel is the text of a file of the handbook, in a Markdown that any page can hold.
func handbookPanel(title, file, desc string) webui.Markdown[Handbook] {
	return webui.Markdown[Handbook]{
		Title: title,
		Desc:  desc,
		Load: func(context.Context) (Handbook, error) {
			text, err := handbookFiles.ReadFile("handbook/" + file)
			if err != nil {
				return Handbook{}, fmt.Errorf("handbook: %w", err)
			}
			return Handbook{Text: string(text)}, nil
		},
		Content: HandbookText,

		TrustedOrigins: []string{"https://raw.githubusercontent.com", "https://pkg.go.dev"},
	}
}
