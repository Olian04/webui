package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

// An Editor shows one text of a model in a code editor, with highlighting for its
// language. With a Store on its Content it is edited and saved, like a Form; without
// one it is a viewer of the text. A language service (JSON's, here, which checks the
// syntax as you type) is only offered to an editor that can be edited, and only because
// this program imports pkg/monaco/json; without that import the editor still highlights.
var (
	ConfigText = webui.String[Config]{
		Label: "Configuration",
		Load:  func(c Config) string { return c.Current },
		Store: func(c *Config, v string) { c.Current = v },
		// The text is bounded, as any field is: Rules apply to what is saved.
		Rules: webui.StringRules{Required: true, MaxLen: 64 << 10},
	}
	// A viewer's accessor has no Store, so an Editor or a Diff built on it only reads.
	DeployedText = webui.String[Config]{
		Label: "Deployed",
		Load:  func(c Config) string { return c.Deployed },
	}
	SavedText = webui.String[Config]{
		Label: "Saved",
		Load:  func(c Config) string { return c.Current },
	}
)

var ConfigEditor = webui.Editor[Config]{
	Title: "Collector configuration",
	Desc:  "An Editor with a Store on its Content. JSON, so its language service checks the syntax: break a bracket to see it.",
	Load:  func(context.Context) (Config, error) { return service.Config(), nil },

	Content:  ConfigText,
	Language: webui.LangJSON,
	Submit: webui.Action[Config]{
		Guard: canEdit[Config],
		Run: func(_ context.Context, c Config) (webui.Outcome, error) {
			service.SetConfig(c.Current)
			return webui.Success("Configuration saved"), nil
		},
	},
}

// A Diff shows two texts of a model and what changed between them. It only reads.
var ConfigDiff = webui.Diff[Config]{
	Title:    "Changes since the last deploy",
	Desc:     "A Diff: the deployed text on the left, the saved text on the right.",
	Load:     func(context.Context) (Config, error) { return service.Config(), nil },
	Original: DeployedText,
	Modified: SavedText,
	Language: webui.LangJSON,
}

var Configuration = webui.Page[webui.NoArgs]{
	Path: "/config",
	Nav:  webui.Nav{Label: "Config", Icon: "file-code", Section: "Configuration"},
	Body: webui.Stack{ConfigEditor, ConfigDiff},
}
