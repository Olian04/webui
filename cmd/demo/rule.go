package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

// Rule is a text the application keeps that is code: the rule that decides which events page
// someone. It is TypeScript, so an editor that can be edited is given the language service for it.
type Rule struct{ Source string }

// An Editor whose Language has a language service gets it only if the program imports the package
// that carries it (see main.go: pkg/monaco/typescript), and only for a visitor who can edit: a
// viewer reads the same text, highlighted, and is sent no service. The service is TypeScript's
// own, so it checks the types as you type: the errors below are not marked by the library, and
// fixing one removes it.
var (
	RuleSource = webui.String[Rule]{
		Label:    "Paging rule",
		Load:     func(r Rule) string { return r.Source },
		Store:    func(r *Rule, v string) { r.Source = v },
		Required: true,
		Rules:    webui.StringRules{MaxLen: 64 << 10},
	}

	RuleEditor = webui.Editor[Rule]{
		Title: "Paging rule",
		Desc:  "An Editor in TypeScript. With the language service imported, the types are checked as you type: this text has three errors to find and fix, and completion and hover work as in an IDE. Signal, Rule, rules and pageOnCall are not declared here: they come from the editor's Context, as do the compiler options (strict, no DOM).",
		Load:  func(context.Context) (Rule, webui.Loaded) { return Rule{Source: service.Rule()}, webui.Ready() },

		Content:  RuleSource,
		Language: webui.LangTypeScript,
		Context:  editorContext,
		Submit: webui.Action[Rule]{
			Guard: canEdit[Rule],
			Run: func(_ context.Context, r Rule) (webui.Outcome, error) {
				service.SetRule(r.Source)
				return webui.Success("Rule saved"), nil
			},
		},
	}
)

var PagingRule = webui.Page[webui.NoArgs]{
	Path: "/rule",
	Nav:  webui.Nav{Label: "Paging rule", Icon: "code", Section: "Configuration"},
	Body: RuleEditor,
}

// seedRule has three type errors, on purpose, for the language service to find: a kind that no
// signal has, a string returned as a number, and an urgency that pageOnCall does not take. Fix them
// and the red goes. Signal, Rule, rules and pageOnCall are declared in context/typescript/rule.d.ts,
// which is what the editor's Context gives the service.
const seedRule = `// Which signals page someone. The TypeScript service checks this as you type, against the
// types the collector hands a rule (hover over a name to see them) and strict options.

export const slow: Rule = {
  name: "slow flush",
  // Error 1: no signal has the kind "timeouts" (it is "timeout"), so this can never be true.
  matches: (signal) => signal.kind === "timeouts" && signal.ageMinutes < 15,
};

export function lagSeconds(signal: Signal): number {
  // Error 2: detail is a string, so it cannot be returned as a number of seconds.
  return signal.detail;
}

export function shouldPage(signal: Signal): boolean {
  const hit = rules.some((rule) => rule.matches(signal));
  // Error 3: urgency is "low" or "high". Type "signal." for completion, and delete the
  // quotes to see the signature.
  if (hit) pageOnCall(signal, "urgent");
  return hit;
}
`
