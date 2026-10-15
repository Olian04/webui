# Agent context: webui

## Layout

| Path               | Role                                                                              |
| ------------------ | --------------------------------------------------------------------------------- |
| `pkg/webui`        | Public API. Declaration AST, split by file. `Compile` lowers to internal runtime. |
| `internal/ir`      | Handoff between declaration and runtime. No transport, no generics.               |
| `internal/args`    | Path/query encode-decode, `webui` tags, optionality.                              |
| `internal/rules`   | RE2 compile, constraint check, HTML attrs from `Rules`.                           |
| `internal/favicon` | Scale `Brand.Logo` to the favicon PNGs (stdlib only).                             |
| `internal/markdown` | Markdown to safe HTML (goldmark, GFM). Link allowlist, no raw HTML, images become links unless from a trusted origin; the only place that decides what text may do on a page. |
| `internal/textdiff` | Unified diff (Myers, bounded) for `Diff`'s no-script view. |
| `pkg/monaco/*`     | Optional language-service packs for the editor: each `go:embed`s one worker and registers it in `internal/render/assets` from `init`. |
| `tools/monaco`     | Node build of the embedded Monaco (`make monaco`, esbuild from the ESM package). Output is committed: `internal/render/assets/monaco`, `pkg/monaco/*/worker`, `pkg/webui/language_gen.go`. Do not edit the output. |
| `internal/tablequery` | Filter, sort and page rows held in memory, from the columns' accessors (`Table.Rows`). |
| `internal/runtime` | Compiled app in `ctx`, prefix, routes, request pipeline, POST actions, flash, `Open` resolve. |
| `internal/render`  | IR + loaded data → HTML; asset routes. `templates/components` is the design language. |
| `test/`            | Integration tests against `pkg/webui` only. External test package (`webui_test`). |
| `test/util/assert` | Test assertions (`got`, `want`). No import of `pkg/` or `internal/`.              |
| `.claude-plugin`, `.cursor-plugin` | Packaging of the skill. The Claude Code marketplace installs only `skills/webui` (a `git-subdir` source, so users do not pull the Go source); the Cursor plugin takes the repo root. No `version` is set, so installs follow the default branch. |
| `skills/webui`     | Skill for agents building *with* the library. Method and judgement, no API list; points at `go doc` for the pinned version. `test/skill_test.go` guards it. |
| `cmd/demo`         | Runnable demo app. Not part of the public API.                                    |

`pkg/webui` files: `app`, `page`, `table`, `form`, `editor`, `diff`, `markdown`, `language`, `layout`, `accessor`, `action`, `outcome`, `link`, `open`, `compile`, `compile_error`, `validate`, `lower`. One package, not one package per type. A type's `validate` and `lower` methods live beside it; `validate.go` and `lower.go` hold the entry points and shared state.

The runnable `Example` functions in `pkg/webui` are the documentation's tests, and are what pkg.go.dev shows. Unit tests are `*_test.go` next to the source they cover, same package. Internal packages export concrete funcs so those tests can call them. Prefer integration tests for the public API.

## Design

| Path                     | Role                                                                          |
| ------------------------ | ----------------------------------------------------------------------------- |
| `docs/design.md`         | The visual language: tokens, metrics, components, states.                     |
| `docs/design-mapping.md` | Which declared type produces which design element, and where the two disagree. |
| `docs/design-decisions.md` | Why the API is shaped as it is, and what each decision costs. |

`internal/render` implements `design.md`. Every colour, radius and elevation is a
design token (the app's `webui.Theme` colours are the only ones it may override); no component reads a literal value. When the two documents and
the code disagree, `design-mapping.md` names the gap on purpose — read its last
two sections before adding a field to close one.

Icons are Font Awesome Free (solid), drawn with a vendored font. The font is
`internal/render/assets/fonts/` (SIL OFL, license beside it); `css/fontawesome.css`
and `icons.txt` are generated from a Font Awesome Free download by
`go run fontawesome_gen.go <download-dir>` in `internal/render/assets`. `icons.txt`
is what `Compile` checks `Nav.Icon` against. Do not edit the generated files.

## Dependency direction

`pkg/webui` → `internal/<phase>`. Never reverse: `internal/` packages do not import `pkg/`.

`internal/` packages may import each other. `pkg/` owns public types and `Compile` glue.

Consumers outside this module cannot import `internal/`. No side effects on import.

Keep algorithms in `internal/`. `pkg/` owns the declaration AST.

## Decided against

Things that were proposed and dropped. Do not start on them again without a reason that is not in here.

### Segments after a rest placeholder (`/file/{path...}/preview`)

**Decision:** `{name...}` stays the last segment of a path, and anything after it is a compile error. Routing stays on `net/http.ServeMux`, which allows a rest wildcard only at the end. Decided 2026-10-12, twice: first the general form, then the narrow one below.

**What was considered**

1. Any position, with a matcher of our own. Dropped: it re-implements what the mux already gives (path cleaning, escaped segments, `HEAD`, `405` with `Allow`), and it needs overlap analysis between pages.
2. Static segments after the rest only, mux kept. Register the truncated pattern (`/file/{_rest...}`) and suffix-match `/preview` ourselves. This is the version worth remembering, because it looks cheap.

**What option 2 would cost**

- A dispatcher shared by every page that has the same truncated pattern. A wrapper per page is not enough: `/file/{p...}` and `/file/{p...}/preview` become the same mux pattern, and `ServeMux` panics on a duplicate. The dispatcher tries the longest suffix first and falls back to the page with none.
- The same grouping in four registrations: `GET`, `POST`, `probe` and `gate`. The winner is chosen by address before the method, so a `POST` to a suffix page with no actions is a `405` and not a fall-through to the end-rest page. The `probe`/`allow` lookup has to ask the matcher too, or a `404` turns into a `405`.
- Matching on `r.URL.EscapedPath()`. The mux hands over the unescaped remainder, so `a%2Fb` cannot be told from `a/b`; it would have to be a 400, like the encoder's refusal of a slash inside a segment.
- New compile checks: one rest per path, static segments only after it, and pages of the same shape with different placeholder names (today only identical text is caught).
- Breadcrumbs: the rest crumbs link to "this page cut at that depth", which is a different address once a suffix follows. They would link to the end-rest page if one is mounted, and be plain text otherwise.
- A rule readers must learn: `/file/x/preview` is always the preview of `x`. A file called `preview` in a subfolder is unreachable through the end-rest page. A literal-prefixed page beats the group, and a longer suffix beats a shorter one.
- A step, not a stopping point: the next request is a dynamic segment after the rest (`/file/{path...}/v/{id}`), which needs matching from the end and a real overlap analysis.
- Size: about 150 lines of dispatcher plus tests across the four muxes, in the hottest path of the runtime.

**What not doing it costs**

- A page cannot sit under an arbitrary-depth tree by path suffix. The suffix has to be something else:
  - an argument in the query, `/file/{path...}?view=preview`, or a [`Tabs`](../pkg/webui/layout.go) panel, whose selected tab is in the address too and survives a reload;
  - a literal before the rest, `/preview/{path...}`, which the mux routes with no help. It is a page of its own with its own `Nav` and `Guard`.
- The addresses are less tidy than `/file/a/b/preview`.

**Revisit when** a real app needs a suffix that cannot be a query argument, a tab or a prefix. Then build option 2 first, on the design above. If `ServeMux` ever allows a rest wildcard before the end of a pattern, the dispatcher is not needed and this entry should go.

### Fetching the next page ahead when the pointer rests on a link

**Decision:** pages are not fetched ahead. Decided 2026-10-15, after building it (PR #31, closed, not merged).

**What was built:** every full page sent a `Speculation-Rules` header naming a served rules file that asked the browser for a `prefetch` (`moderate`: about 200 ms of hover) of each link under the mount prefix, except downloads, links that open elsewhere and a panel's own sort, pager, filter and tab links. The click is an ordinary navigation that uses the response.

**What it gave** (Chrome 155, the demo behind a proxy that adds latency, median of 6): a click after a rest on the link saved one round trip plus the page's build time. At 100 ms: 137 ms became 29 ms. At 250 ms, after a 1 s rest: 292 ms became 32 ms. A click with no rest saved nothing. Parsing and painting (about 30 ms) are not saved, and on localhost the gain is 5 to 9 ms.

**Why it is not kept:** the gain is a round trip, and what it costs is a set of special cases that every later change has to keep in mind.

- A fetch ahead runs the page's Guard, `Load` and `NavEntries` for a page nobody opens. Nothing in the library says a `Load` is free of side effects, and there is no opt-out, so an app that marks a row as read in `Load`, or models "Log out" as a page under the prefix, would act on a hover.
- A page fetched ahead takes the toast out of its cookie (`webui_flash`) and is thrown away if another link is followed. Keeping the toast needed a rule of its own: answer a request made ahead (`Sec-Purpose: prefetch`) with a non-success while a toast waits.
- The metrics count a request made ahead like any other, and that non-success counts as an error (a 503 as a 5xx; a 425 would not, but is one more thing to get right).
- The status bar called a page that was old at the click "cached?", so it had a "fetched ahead" case.
- Four selectors in the rules name the markup of the table and the tabs, with nothing to say they have fallen out of step.
- Only Chrome and Edge have it. Firefox and Safari do not ship Speculation Rules, and Brave disables them.

**What else was considered**

1. A hidden iframe, swapped in on the click. Dropped: every page says `frame-ancestors 'none'`, and a swap is a second navigation system (history, scroll, title, the view transition, per-page scripts such as the editor's).
2. `prerender` instead of `prefetch`. A `no-store` page is not prerendered (measured: the request goes out as `prefetch;prerender` and the page opens as a prefetch). Pages stay `no-store`, which the status bar's "cached?" check relies on.
3. An inline `<script type="speculationrules">`. The content security policy would need a new source in `script-src`.

**Revisit when** Firefox and Safari ship Speculation Rules, and either a page's `Load` is declared safe to run on a GET or a page can opt out, or someone has measured that the wait for the next page matters to them.

## Tests

`test/` must import `pkg/webui` only, not `internal/`. `test/util/assert` is the exception: assertions only, imported by tests.

---

## Go proverbs ([source](https://go-proverbs.github.io/), caveman compress)

Concurrency channels coordinate mutex serializes · not parallelism · small interface sharp · zero value useful · `any` untyped tame it · gofmt settles bikeshed · tiny copy beats dep hairball syscall/cgo build tags isolate · cgo not Go · `unsafe` no contract · clarity beats wit · reflection stay cold path · errors values inspect wrap once · architecture name docs users · panic stays in `main` / hard startup.

## Uber style distill ([guide](https://github.com/uber-go/guide/blob/master/style.md), caveman compress)

Rare `*Iface` · `var _ I = (*T)(nil)` at export boundary · defer unlock pairs · chan buffer zero or one usually · slice/map copy exported API boundaries · typed errors `%w` chain handle once · assert comma-ok · goroutine bounded ctx/waitgroup · no zombie `init()` · globals inject not mutate · exits from `main` only · strconv hot paths · structs field-named literals · table tests sub `t.Run`.

---

Canon links above beat bullet memory when tradeoff unclear.

## The skill

`skills/webui/SKILL.md` teaches agents to build *with* the library. It is method and judgement, not an API list, so that it does not go stale: it sends the reader to `go doc` for the version in `go.mod` and to pkg.go.dev. Keep it that way. Do not add field names, signatures or code to it. If a design principle it states stops being true (how authentication works, how pages are mounted, what the library refuses to do), change the skill in the same commit. `test/skill_test.go` fails if the skill names a `webui.` identifier or a `go doc` symbol that no longer exists.
