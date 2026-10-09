package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

// Handbook is a text the application has, which is shown as it is written.
type Handbook struct{ Text string }

// A Markdown shows one text of a model, set as markdown, and only reads: its accessor has no
// Store. Raw HTML in it is never rendered, a link goes only to an http or https address or a
// mailto, and an image is a link to the picture, so a text from outside is safe to show.
// A fenced block that names a language is coloured by the editor the page then loads;
// without script it is plain text in a block.
var (
	HandbookText = webui.String[Handbook]{
		Label: "Collector handbook",
		Load:  func(h Handbook) string { return h.Text },
	}

	Handbooks = webui.Page[webui.NoArgs]{
		Path: "/handbook",
		Nav:  webui.Nav{Label: "Runbook", Icon: "book-open", Section: "Operations"},
		Body: webui.Markdown[Handbook]{
			Title:   "Handbook",
			Desc:    "A Markdown: text set as markdown, read only. Raw HTML is never rendered.",
			Load:    func(context.Context) (Handbook, error) { return Handbook{Text: handbookSource}, nil },
			Content: HandbookText,
		},
	}
)

const handbookSource = "# Collector handbook\n" +
	"\n" +
	"What to do when the collector **misbehaves**. Anything not covered here goes to the\n" +
	"[on-call channel](https://example.com/oncall).\n" +
	"\n" +
	"## Is it down?\n" +
	"\n" +
	"1. Open **System** and read the health badge.\n" +
	"2. Check the queue depth. Above 5 000 the sinks are not keeping up.\n" +
	"3. Look at the **Audit log** for a change in the last hour:\n" +
	"   - a configuration saved by someone else\n" +
	"   - a sink that was restarted without a reason\n" +
	"\n" +
	"| Health | Meaning | First step |\n" +
	"| --- | --- | --- |\n" +
	"| `healthy` | Everything is flowing | none |\n" +
	"| `degraded` | A sink is slow or refusing | restart the sink |\n" +
	"| `down` | Nothing is being collected | page the on-call |\n" +
	"\n" +
	"## Restarting a sink\n" +
	"\n" +
	"```bash\n" +
	"systemctl restart collector-sink@file\n" +
	"journalctl -u collector-sink@file --since '5 min ago'\n" +
	"```\n" +
	"\n" +
	"The configuration it reads is on the **Config** page:\n" +
	"\n" +
	"```json\n" +
	"{ \"sample\": 0.25, \"sinks\": [{ \"kind\": \"stdout\" }] }\n" +
	"```\n" +
	"\n" +
	"### Who to call\n" +
	"\n" +
	"Start with the person on call, and go on to the owner of the site only when the queue\n" +
	"is still growing after *ten minutes*.\n" +
	"\n" +
	"#### Contacts\n" +
	"\n" +
	"Numbers are kept in `/etc/collector/oncall.yml` on every host.\n" +
	"\n" +
	"## Checklist\n" +
	"\n" +
	"- [x] Page acknowledged\n" +
	"- [ ] Cause found\n" +
	"- [ ] ~~Rollback~~ not needed\n" +
	"\n" +
	"> Do not edit the configuration during an incident without telling the channel.\n" +
	"\n" +
	"<script>alert('raw HTML is never rendered')</script>\n"
