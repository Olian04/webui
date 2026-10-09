# Design decisions

Why the library is shaped the way it is: the decisions that are not obvious from the
code, with their costs. Read this before changing the shape of the public API. The
standing preferences behind them are a small public surface, as much as possible
checked by the compiler, behaviour the library owns and infers instead of hooks the
user implements, a static top-level declaration style, and everything working with
JavaScript switched off.

The user-facing behaviour is documented in the package documentation
(`go doc github.com/Olian04/webui/pkg/webui`, or pkg.go.dev). `design.md` is the
visual language, and `design-mapping.md` says which declared type produces which
design element.

## Declaring pages and arguments

- Arguments are one struct per page. `Open` is compiler-checked and `Guard` is
  typed. Cost: reflection enters the user-facing path for encode/decode, with
  `Compile` in front of it so failures land at startup;
  `Pages` goes back to an interface slice since `Page[A]` differs per `A`;
  cross-page reuse becomes struct embedding rather than shared vars; and a zero
  field cannot be distinguished from an absent one without a pointer.
- Non-string arguments: built-in decoding for `int`, `int64`, `bool` and
  `float64`; a type that is none of these is a compile error, with a fix.
- Optionality: a zero field means absent and stays out of the URL. A field
  where zero is meaningful needs a pointer — the same trade as `Rules`.
- Argument names come from the lower-cased field name, overridable with a
  `webui:"..."` tag. `Compile` rejects duplicates, unsupported field types, and
  path placeholders with no matching field.
- `ArgsOf` returns one value and panics on a mistake, because the only ways it
  can fail are a wrong type or a `ctx` from outside a page: neither is something
  a caller can handle, and an error to check at every call site buys nothing. The
  runtime recovers any panic in a closure into the "Something went wrong" page
  and a log line with the stack, instead of a dropped connection.
- The runtime keys `ctx` on an unexported type, which `ArgsOf` and `Open` read. A string key would collide with
  any other package using the same string, and `go vet` does not catch it.
- Pages that link to each other are still static `var`s, with one rule. A list
  whose rows `Link` to a detail page, whose form action redirects back with
  `Open(ctx, List, ...)`, is a cycle Go rejects at build time:
  `initialization cycle for List`. Go counts any mention of a package-level
  variable inside an initializer, function literals included, so splitting the
  `Link` and the `Action` into their own `var`s only lengthens the chain. A
  constant is not a variable, so a page that is linked back to declares its path
  as a `webui.PageID[A]` constant, and the link back names the constant:

  ```go
  const DevicesPath webui.PageID[webui.NoArgs] = "/device"

  var Devices = webui.Page[webui.NoArgs]{Path: DevicesPath, ...} // links forward: Page: Details
  var SaveDevice = webui.Action[Device]{
    Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
      return webui.Success("Deleted").Then(webui.Open(ctx, DevicesPath, webui.NoArgs{})), nil
    },
  }
  ```

  `PageID[A]` carries the argument type, so `Page[NoArgs]{Path: DetailsPath}` and
  `Open(ctx, DevicesPath, DeviceArgs{})` do not compile. `Path: "/device"` still
  does: a literal or an untyped constant converts. `Open` and `Link.Page` take a
  page or its `PageID`; only a `string` variable can no longer be a `Path`.
- `Open` can fail at render time (a zero path argument), so `Target` carries an
  error, which it keeps to itself: a `Target` is opaque and only `Outcome.Then`
  reads it, so a redirect can only be to a page of the app. A row link that cannot
  be built is a plain row and is logged rather than panicking, and `Outcome.Then`
  with one is an error.
- `Open` is `webui.Open(ctx, page, args)`, not a method, so `Page` carries no
  behaviour at all. Type inference still gives a compile error on the wrong
  argument struct: `type OtherArgs does not match inferred type DetailsArgs`.
  Resolving through the runtime means a `Target`'s address includes the mount prefix,
  so it is a real href and the renderer no longer prepends anything; and an
  unmounted page is caught at render rather than producing a dead link.
  `Link` declares its destination page and an `Args` function instead of
  building the `Target` itself, which moves target checking from render time to
  `Compile`. `Open` remains for programmatic navigation, such as
  `Outcome.Then`.

## Validation and rules

- Validation is declarative. `Rules` is a per-value-type struct of data, not
  closures, so the same set renders as HTML constraint attributes (`required`,
  `minlength`, `pattern`, `min`, `max`) and re-runs on the server before
  `Submit`. A rule is a plain scalar when its zero value already means "no
  constraint" and its message is derivable (`MinLen`); a struct pointer when
  either fails — `Pattern` because a regex explains nothing to a user, `Min`
  because a zero bound is a real constraint and Go will not let you write `&0`.
  The set is closed: a rule the framework does not define cannot be added
  without writing client code, so anything beyond it goes in `Run`.
- `Store` is pure assignment with no error return, and no `ctx`. The ordering
  problem came from `Store` seeing `*M`; rules see only their own value, so
  inter-field dependencies are not expressible and need no ordering.
- `Pattern` is not portable: Go's RE2 rejects the lookahead and backreference
  syntax people copy from JavaScript examples. `Compile` compiles every
  `Pattern` at startup and the error says RE2, not "regex". The expression is
  anchored on both sides, because an HTML `pattern` attribute must match the
  whole value and the server has to reach the same verdict as the browser.
- Any struct a user fills with an unkeyed literal from another package trips
  `go vet`'s composites check, so nested literals in the API must expect keyed
  fields; `Reject` takes `webui.Field[Device](IP, "...")` messages, which are keyed.
- `Group` listed as a column is rejected by `Compile` rather than flattened —
  flattening would mean defining what a nested group is in a cell.
- A nested `Table` inside a form's `Fields` stays out of scope. If it turns out
  to be wanted, it is solved then.

## Actions and outcomes

- `Run` says how it ended with an `Outcome`, by intent, not by assembling a
  toast and a status: `Success` and `Warning` are accepted (the action was done and
  the user is sent back, or on with `.Then(target)`), and differ in that a
  `Warning` says something to be aware of. `Failure` and `Reject` are not accepted
  (nothing was done): the form is shown again with what was typed kept, and they
  differ in that a `Failure` speaks for the whole form and a `Reject` for fields,
  by `webui.Field[Device](IP, "already in use")`, variadic. The toast, its colour
  and the status code follow from the constructor, and there is no default
  message: a zero `Outcome` is a quiet success. An `error` is for something the
  user cannot fix by editing the form, and shows only that something went wrong.
  `Outcome` stays non-generic by erasing the model inside `Reject`, so bulk
  actions do not inherit a meaningless `[]FieldError[[]Device]`.
- Pipeline: parse -> rules -> `Store` -> `Guard` -> `Run`. `Store` applies to a
  copy of the model `Load` returned, not a zero one, so read-only fields (an ID
  with no `Store`) are intact for `Guard` and `Run`. Parse failures
  short-circuit, so a form with both a malformed number and a duplicate value
  shows the parse error first and the duplicate only on the next submit.
- After a successful action the browser gets a `303` and the toast rides a
  short-lived `HttpOnly` cookie, so a reload does not repeat the POST and the
  URL carries nothing. POSTs are checked with `http.CrossOriginProtection`
  (Fetch metadata and `Origin`); behind a proxy that rewrites `Host`, wrap the
  handler and allow the public origin.
- The re-render after a rejection must echo the raw submitted input, not the
  model value, or the user's bad input disappears and the form looks like it
  reset.
- Bulk-action gating: `Action[[]Device].Guard` receives the selection, so
  gating happens at execution rather than per-row at render. Row actions are
  gated at render, per row, by the same `Guard` that authorises the POST.

## Tables, filters and view state

- Pagination and sorting are library-owned view state. A table always pages, 25
  rows a page unless `Table.PageSize` says, since one that listed every row would
  grow without bound, and every column is sortable and filterable. A table says where
  its rows come from with one of three fields. `Rows` returns every row and the
  library filters, sorts and pages them from the columns' own accessors, a number
  as a number and text as text, so a table over a slice has no query code at all.
  `Load` is for a source that pages itself: it receives a `Query` and returns
  `Window{Items, Total}`, doing the work itself; `Total` below what has been shown
  means unknown. `Feed` is for a source that pages by a cursor: the rows come in the
  source's order, and it is handed the cursor and a page size and returns the rows and
  the next cursor. A cursor continues one sequence, so there is nothing for the library
  to reorder or filter: the table has no sort links or filters, only First page, Previous and Next, and
  says which rows it shows. The address carries the cursor, the first row's number and the pages
  before, so nothing is kept on the server; Previous goes back as far as the address
  remembers (a bounded number of characters, the oldest forgotten first). It is a source of
  its own, not a mode of `Query`, so a table can never carry both a cursor and an
  offset. Exactly one of the three is set, which `Compile` checks. Setting a filter
  returns every table to its first page.
- Column filtering is library-owned view state, like sorting. Hovering a
  header shows a filter icon; it opens a form that is a plain GET, so it works
  without script. A column with a fixed set of options — a `Badge`, whose options
  are the keys of its `Kinds` — gets a multi-select of exactly those, a numeric
  column (`Int`, `Float`, `Slider`) a minimum and a maximum, and every other
  column a text input. `Load` receives them in `Query.Filters` (the options
  chosen, or the one text typed) and `Query.Ranges` (a `Range{Min, Max *float64}`,
  inclusive, either end optional), keyed like `Query.Sort`. A number is compared
  as a number, never as text: "5" is not a way to ask for more than 5. A chosen
  option that is not an option, an empty text, a bound that is not a finite
  number and a column the table does not have never arrive, and a typed filter is
  cut at 200 characters. A multi-select is repeated parameters
  (`devices.filter.status=a&devices.filter.status=b`), which is also what a form
  of checkboxes submits; a range is `devices.min.occurrences=300&devices.max.occurrences=500`. The page has no row of argument controls:
  its query arguments come from the address (a link, `Open`) and have no UI.
- A link inside a panel that changes only that panel's arguments
  fetches the *same URL* with an `X-Webui-Leaf` header and replaces that panel;
  without script it is an ordinary navigation. The page `Guard` runs on every
  request, leaf addresses are derived from the `Body` tree by position, and
  framework parameters never touch the URL. `Vary: X-Webui-Leaf` is set. After
  a leaf moves, sibling panels that embed the old address are fetched again and
  forms keep what was typed (only their `action` is patched). After an action
  the whole page re-renders (a leaf-level refresh after an action is not done).
- Transport parameters travel as headers, never in the URL; *view state* is in
  the URL, because it is what a copied address has to reproduce. The URL
  carries the page's arguments and the view state of its tables and tabs,
  headers carry read-side transport (`X-Webui-Leaf`), form bodies carry
  write-side routing. View state is named `<id>.<param>` (`devices.offset`,
  `devices.sort`, `devices.desc`, `tabs.tab`), and an argument name may not
  contain `.`, so the two can never collide; `Compile` rejects one that does.
  An ID is a lower-case word, unique within its page (two pages may reuse one),
  and defaults to the component's name. Parameters for a leaf the page does not
  have are ignored. Consequence: an iframe cannot set headers, so embedding a
  leaf later needs its own path and a standalone document, rather than reusing
  the refresh mechanism.
- Why the library owns view state instead of the argument struct: the struct
  would have to carry `Offset`, `Sort`, `Desc` and `Tab` fields for the
  framework's benefit, and the table would name them by string. The cost is
  that nothing outside the leaf can read or set that state — `Guard` and
  sibling leaves cannot depend on the selected tab, and `Open` cannot link to a
  sorted table.
- `Rows` is searched and filtered by the library from the accessors' own values
  (`internal/tablequery`); `Load` is the escape hatch. A column is known by its `Label`
  in the address and in `Query`, lower-cased with dashes in the address; there is no
  key to declare, and `Compile` refuses labels that collide.
- Search belongs to tables, not pages: a table with `Search` and a `RowClick` offers
  its rows, the page's `Guard` runs first with zero arguments, and each result is
  checked against the page it leads to with that page's own arguments and `Guard`. A
  `Rows` table is searched across every column by the library; a `Load` table is handed
  `Query.Search`. A number is searched but not shown in the result's line, since it
  says nothing without its label.

## Navigation and links

- A page with no `Label` lights the entry of its nearest ancestor path (the longest
  proper prefix that is a page with a `Label`, never the root). There is no field to
  declare, because the breadcrumb already derives its parents the same way.
- Entries and search results whose page `Guard` refuses the visitor are left out; the
  guards are asked with no arguments, as a visitor who opened the page bare would be.
  The page still answers 403 to anyone who types the address.
- Going back is the library's. A `Link` to a page that has a form adds the address of
  the page it is on, view state included, in the reserved parameter `webui.from`. It
  rides along through the form's own links, its re-render after a rejection and its
  POST, and a chain of pages unwinds one hop at a time, bounded so the address cannot
  grow without end. Without it Cancel goes to the breadcrumb parent and a save stays
  put. It is only ever followed when it is a path in the app, so it cannot become an
  open redirect. Expressing this as a hook the form implemented was tried and
  rejected: every link into the page had to remember to set it.
- The sidebar collapses to an icon rail from a button in its footer, and always below
  820px; the choice is a browser preference, so it stays out of the address. An entry
  with no icon shows its label's initial in the rail.
- Icons are Font Awesome Free solid, drawn with a vendored font served by the app, so
  the content security policy stays `'self'`. The class and name list are generated by
  `internal/render/assets/fontawesome_gen.go`.
- A theme is four colours. The renderer derives the rest per mode with `color-mix`,
  because one colour cannot be right on both a dark and a light surface. Surfaces and
  text are not themeable: they are what makes the modes differ, and the mode is the
  viewer's choice.

## Assets and transport

- Every asset is served under a name with a short hash of its contents before the
  extension (`app.410f9909.css`), found by pages through `Renderer.AssetHref`. The
  address changes when, and only when, the file does, so it is cached for a year as
  `immutable` and never revalidated. The icon stylesheet names the font by its
  hashed name, so the font is hashed first. An unversioned address is a 404: pages
  are `no-store`, so no cached page names an old one.
- Text assets are compressed once, with gzip at the best level, when the app is
  built, and served with their own validator (`"<hash>-gzip"`) and `Vary:
  Accept-Encoding`. Pages and the search are built per request, so they are
  compressed by a middleware when the client accepts it, deciding from the content
  type when the status is written. Images and fonts are left alone, as they are
  compressed already. There is no CSRF token in a page, so compressing HTML does not
  expose one.

## Code editors

- `Editor` and `Diff` are Monaco, the editor of VS Code, always embedded. About 5 MB
  of assets is little next to what a Go binary weighs, and a page that has no editor
  never links them, so the cost is the binary's and not the page's. Syntax
  highlighting is built for every language Monaco has, and a `Language` constant is
  generated for each by `tools/monaco`, so an unknown language is a compile error
  instead of an editor that silently shows plain text.
- The bundle is built from Monaco's ESM package with esbuild, with a chunk for each
  language that is loaded when it is first used, and not from the AMD build: the AMD
  build makes its workers from a `blob:` URL, which a strict content security policy
  refuses. Workers are same-origin files under `_webui/monaco`, so the existing
  policy is enough and nothing is loosened for it. The output is committed so that
  `go get` needs no node.
- A language service is a worker, and Monaco has four: JSON, CSS (and SCSS and
  Less), HTML (and Handlebars and Razor) and TypeScript (and JavaScript). A service
  is an opt-in import of its own package, because TypeScript alone is 6.7 MB.
  Without the import the editor switches that language's features off, and so
  highlights and nothing more: it degrades, it does not fail.
- A service goes only to an editor that can be edited, by the visitor who sees it.
  This is decided on the server from the same Guard that gates the Save button, so
  a viewer never downloads a service, and the page never carries one for text that
  cannot change.
- An `Editor` is lowered to the same node as a `Form` with one field, so loading,
  rules, the Guard, rejection and echo are the one pipeline and not a second one.
  Its body limit is raised (16 MiB) on pages that have an editor only, since a
  text is the one field that is expected to be large.
- Without script the editor is a `textarea`, the viewer a `pre`, and a `Diff` a
  unified diff that the server makes. The script mounts Monaco over them and keeps
  the `textarea` as the editor has it, so the form posts as it always did.

## Markdown

- `Markdown` is a leaf that reads one text of a model, with an accessor as `Diff` has
  one, and has no Store: editing a text is an `Editor`'s job. It is rendered on the
  server, by goldmark (CommonMark and GitHub's extensions), so it needs no script and
  its markup is the same on every request.
- The text is data, not markup the application wrote, so it is held to what any
  other text of the model is. Raw HTML is never rendered, which is goldmark's default
  and is not switched off. A link goes only to an http or https address, a mailto, or
  a path in the application; anything else keeps its words and loses its link. Links
  to other sites open in a tab of their own with `noopener`.
- A path in a link, `/device/dev_1`, is an address in the application, and the text
  cannot know where the application is mounted, so the renderer puts `Compile`'s
  prefix in front of it and leaves it in the same tab. This is the only kind of
  relative link there is: `device`, `../x` and `#section` would be read against
  whatever page the text happens to be on, so they are not links. A path is not
  checked against the pages, since the text is only known when it is loaded; one that
  is no page is the application's own not-found page. `//host` and `/\host` are
  other sites to a browser, and are refused.
- An image is a link to the picture, named by its alt text. The content security
  policy refuses a picture from another site, so drawing one would only show a broken
  icon, and a picture is how a text makes the visitor's browser fetch from a server of
  the text's choosing. The policy is not loosened for markdown.
- Code blocks are plain text in a block. When one names a language, the page links the
  editor that is already embedded and the glue colours the block by its tokenizers; a
  page whose markdown has no such block never links the editor. Colouring is
  enhancement: without script, or for a language the editor does not know, the block
  stays plain. No language service is ever offered, since the text cannot be edited.
- goldmark is the library's second runtime dependency after templ. Writing a markdown
  parser would have meant owning its edge cases, and its escaping, in a place where a
  mistake is a cross-site script. It is pure Go, and was already in the module graph
  through the linter, so it adds nothing transitive.

## Compiling and errors

- `Compile(prefix)` replaces a separate validate step and handler constructor. It is the one
  place the mount prefix is stated. `Open` resolves the prefix
  through the runtime, so a `Target`'s address is absolute. The compiled
  form is a separate value, so mutating `App` afterwards has no effect — which
  is the intent.
- `Compile` returns a handler even on failure, serving the error at every path
  under the prefix. The guarantee is therefore not "no handler without a
  passing check" but "no *silently* broken handler". `MustCompile` panics, for
  callers who want the process to refuse to start.
- The compile error page shows type and field names. Fine behind the auth
  middleware an admin panel normally sits behind; worth a conscious decision
  before anyone mounts the prefix publicly.
- `CompileError` carries `Page`, `Args`, `Detail` and `Fix`. Go cannot recover
  the file and line of a struct literal at run time, so a page is identified by
  its `Path` and its argument type name — both must therefore always appear in
  the message, since they are the only coordinates available.
- `Compile` collects every problem instead of stopping at the first. The errors
  are structured rather than parsed from a compiler, so there is no reason to
  report one at a time.
