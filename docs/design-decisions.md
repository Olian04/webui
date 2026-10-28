# Design decisions

Why the library is shaped the way it is: the decisions that are not obvious from the
code, with their costs. Read this before changing the shape of the public API. The
standing preferences behind them are a small public surface, as much as possible
checked by the compiler, behaviour the library owns and infers instead of hooks the
user implements, a static top-level declaration style, and everything working with
JavaScript switched off.

HTML is the state of the application, and the only thing the server answers a browser with (hypermedia as the engine of application state, as in
[HATEOAS](https://htmx.org/essays/hateoas) and [hypermedia-driven applications](https://htmx.org/essays/hypermedia-driven-applications), whose thinking is a big
inspiration here): a page, a panel that script asks for by `X-Webui-Leaf`, and whatever a script-made request is answered with are HTML, which holds the links, forms and
buttons that a visitor can use in that state, and the script only asks for it and puts it in the page. There is no JSON endpoint, and a script does not learn what the
server's states mean or build markup from data. What this costs: a script feature needs a fragment that the server renders, which is more work than a JSON answer, and a
client that is not a browser has no data format to read.

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
- The rest of a path is a list. The last segment of a path may be `{name...}`, which
  takes every segment from there on, and its field is a `[]string`, one element for
  each. It was once a `string` of the segments joined by "/", which made every user
  split it and made a slash inside a name ambiguous; a slice says what the segments
  are, and the codec refuses what a slice cannot say (an empty element, or one that
  holds a slash, which would be read back as two). The address holds the joined
  segments, escaped one by one. A path segment is one value, so a slice is refused for
  `{name}` and for any `{name...}` that is not the last segment (a query parameter that repeats
  is the other list, below). It is the last segment only because `net/http.ServeMux`, which
  routes the pages, allows a rest wildcard only there; a route of our own to lift that
  would have had to re-implement what the mux gives for nothing (cleaning, escaped
  segments, `HEAD`, `405` with `Allow`).
- A slice that is not the rest of a path is a query parameter that repeats. It was refused ("a URL
  carries one value per name") for as long as the only list was the rest of a path, but a repeated
  key (`?tag=a&tag=b`) is how URLs, HTML forms and `URLSearchParams.getAll` have always carried a list, and
  the machinery was there already: a table's multi-select filter keeps every value of a parameter, joined
  by the unit separator in the request's arguments, and the address builder writes such a value back as
  repeated parameters. The spec says which fields are lists (`ArgSpec.List`, with the kind of one element, so
  `[]string`, `[]int`, `[]int64`, `[]float64`, `[]bool` and slices of defined types of them all work), the
  parser keeps every value of those keys, and the codec parses each element on its own and reports each that
  fails (a 400, with the value).
- **What a repeated parameter leaves out on purpose.** An empty string, which a URL cannot tell from none (so `[]string{""}` is
  as absent as `nil`), and an empty list, which is as absent as a nil one; and what it refuses: an element holding U+001F (the
  separator, which would be read back as two), and a slice of anything but the five types (a `[]byte` is a mistake, not a list
  of numbers). A slice for `{name}` is a compile error with a fix that says to write `{name...}` or to drop the segment so that
  it is a query parameter. The one limit is the address's own: a long list is a long URL.

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
- **What the `PageID` constant gives, and costs.** `PageID[A]` carries the argument type, so `Page[NoArgs]{Path: DetailsPath}`
  and `Open(ctx, DevicesPath, DeviceArgs{})` do not compile. `Path: "/device"` still does: a literal or an untyped constant
  converts. `Open` and `Link.Page` take a page or its `PageID`; only a `string` variable can no longer be a `Path`.

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
- The browser says what the server says. The browser checks the constraint attributes before it sends a form, and
  says what it found in words of its own: "Please match the format requested.", "Please fill out this field." A refused
  value reads the same whoever refuses it, so the words are written once, in `internal/rules`, which `Check`
  (the server, beneath the field) and `Validity` (the browser) both use, and a test keeps them equal. `Validity` is a map
  by the names of the properties of the control's `ValidityState` (`valueMissing`, `tooShort`, `tooLong`,
  `patternMismatch`, `rangeUnderflow`, `rangeOverflow`, `badInput`, `stepMismatch`), the standard
  vocabulary of the Constraint Validation API, put in an attribute for each, named for the property in kebab case (`data-msg-value-missing`, `data-msg-pattern-mismatch`), on each editable input and select. They were one JSON object in `data-validity`, which the script parsed; one attribute is a plain value that the script reads and the HTML says.
- **With script the browser's words are the server's, and are taken back when the value changes.** When a control
  reports itself invalid (the `invalid` event), the first reason that is true, in the order the server checks, goes to
  `setCustomValidity`, which the browser then shows in its own bubble; it is taken back when the value changes, since a custom
  message keeps a control invalid until it is. Nothing is drawn by the page and nothing is reinvented: the bubble, the focus and
  the blocking of the submission are the browser's. Without script the browser says its own words, and a pattern's message
  follows them as the control's `title`, which is what the attribute is for. Left as they were: a moment's own parse errors, and
  the range slider, which cannot leave its bounds. What it costs: a `tooShort` or `tooLong` message is only given for a value
  the visitor edited, which is the browser's rule for those two, so a value that was loaded too long is caught by the server.

- `Store` is pure assignment with no error return, and no `ctx`. The ordering
  problem came from `Store` seeing `*M`; rules see only their own value, so
  inter-field dependencies are not expressible and need no ordering.
- A `Pattern` runs twice, in Go on the server and in the browser as the `pattern` attribute, and the two regular
  expressions are not one: RE2 is not "the intersection" of Go's and JavaScript's, and it is not. Measured, in Go 1.27 and in V8 with the `v` and the `u` flags (a corpus that is kept with the tests): a browser **ignores** a `pattern` it cannot compile, with no error, so the field is not checked
  in the page; both accept some patterns and read them differently (`.` and `\s` match more characters in a
  browser; `[a&&b]` is an intersection under `v`); and Go reads a great deal that a browser refuses (`\pL`, `\A`, `\z`,
  `\Q..\E`, `(?i)`, `(?P<n>)`, `\_`, `\-`, `[a-]`, `[(]`, `[/]`, a loose `]` or `}`, `\x{41}`, POSIX classes).
- **`Compile` accepts only a subset of patterns, and writes the rest as the browser reads it.** So `Compile` reads the pattern
  itself and accepts only a subset, listed in "Text rules and patterns" in the package documentation, each part of which was
  held against both engines: the corpus, 380 patterns by 110 inputs, is recorded as data and the Go tests hold the library to it
  without a browser; a random search of 60,000 accepted patterns against V8 found no difference. What the engines read
  differently is written for Go as the browser reads it, not refused: `.` becomes `[^\n\r\x{2028}\x{2029}]` and `\s` the white
  space of a browser, tested against every code point. Left out on purpose and listed in the test: named groups, flags in a
  group (not in the browsers that have `v`), the long names of a property, and the C categories (what is unassigned is the
  Unicode version of the engine, which is also why a category added to Unicode lately may differ; they agree today).
- **What the subset of patterns costs.** An app with a pattern outside the subset fails to compile with a message that says what to write; the subset has no
  lookahead, so "has a digit somewhere" is not a pattern; Firefox and Safari have engines of their own, and the corpus holds
  only what the specification settles. The expression is anchored on both sides, because an HTML `pattern` attribute must match
  the whole value and the server has to reach the same verdict as the browser, and the pattern is compiled alone first so that
  `a)|(b` cannot close the group it is put in.

- **Text rules** (`StartsWith`, `EndsWith`, `Contains`, `NotStartsWith`, `NotEndsWith`, `NotContains`) are plain
  strings, since their zero value is no constraint and their message is derivable. The server tests them with the
  `strings` package; the browser has no attribute for them, so each is a JavaScript expression that the whole value
  matches when it holds the rule, and the input's one `pattern` is all of them and the field's own as lookaheads
  (`(?=(?:...)$)`), which is the browser's alone: the server never runs them as patterns, which is why a
  pattern may have no lookahead and a text rule may. When the field has more than one condition on its text, the
  browser can only say that the pattern does not match, so `data-part-N` carries each condition, with `data-part-N-msg` its words, and
  `enhance.js` gives `setCustomValidity` the first that the value fails (the order the server tests them), which
  keeps the page and the server saying the same thing; without script the browser says its own words and the title,
  which holds every message. Case is exact. A text no value can hold (a line break, `StartsWith` with a
  `NotContains` in it, a text longer than `MaxLen`) is a compile error. Not done: case-insensitive rules, a list
  of texts for one rule, and a rule on a number or an enum.
- Any struct a user fills with an unkeyed literal from another package trips
  `go vet`'s composites check, so nested literals in the API must expect keyed
  fields; `Reject` takes `webui.Field[Device](IP, "...")` messages, which are keyed.

## Accessors

- An accessor says how its value reads with `Format`, a function the application writes, on `Int`, `Float`, `OptionalInt`, `OptionalFloat`, `Datetime` and `Timestamp`. It changes only what is read (a table cell, a
  field with no Store, and what the search looks in): sorting, filters, the range bounds, an
  editable input and the export keep the value, so a size can read "1.5 GiB" and sort as
  bytes.
- **The library ships no formats.** Considered and dropped: formats as constants (`FormatBytes`, `FormatDuration`), which is less API to write and less to get wrong, but a list that grows with every unit someone needs, and
  a way for an application to say anything else; and a second accessor that shows what another sorts by, which is a second
  declaration of one column. What it costs: each application writes its bytes formatter; the range filter of a number is typed
  in the number and not in what Format shows; a moment's Format is not handed an empty or unparsable value, which shows as it
  did. A Format that panics is logged and the value shown as it is, so one bug is a plain number and not every page that has the
  column.

- **A column can leave itself out of sorting or filtering** (`NoSort`, `NoFilter` on each accessor that is a column).
  The default stays "every column does both", since that is what a table of rows is for and an app that says nothing
  gets it. A column that is left out has no sort link, or no filter menu, in its header, and a header with neither is
  only its name; and the address cannot ask for what it left out: a sort by it is dropped like a sort by a column the table
  does not have, and its filter or range never reaches `Load` (nor the library's own filtering, for `Rows`), so the
  address, which is the visitor's to edit, is no way round it. The parameters of a column that left a part out
  are not kept on the table's other links either. The two are separate because they are: free text is worth
  filtering and not ordering, a list of words the reverse. It is a flag on the accessor, with the `Label` that names
  the column, since an accessor already describes its own column; the same accessor in a form says nothing of it.
  Not done: a rule for the whole table (`Table.NoSort`), which a table of nothing but filterable columns does not need.

- **A field may say what it is for, with `Help`.** A label names a field and its rules say what a value may be, and neither says what
  it is for or what to put in it ("0.25 keeps one event in four"). `Help` is a field of every accessor, beside `NoSort` and
  `NoFilter`, and so is one method on the accessor interface (`meta`) in place of one for each. It goes in the popover that is
  already beside the label, the information icon that lists the rules, as a section of its own: the heading "Help:" and the
  text, above "Rules:" and its list. A second icon was tried first (a question mark beside the information icon) and dropped: two
  icons on one label is more to see and to tell apart for what is one question, what the visitor should know about the field, and
  one popover has one place for a keyboard and a screen reader to land and read all of it. The popover is the panel's
  `Desc` popover, so it needs no script. A `Placeholder` was not stretched to it: it disappears when the visitor types and is only for text. A read-only field
  shows it too, since what a value is for is as true when it is only read. A table does not: a column header is a name and
  a sort, and a `Placeholder` is not there either. It is text, not markup, so a popover is never a way into the page. The
  cost is that the help and the rules are in one popover that the visitor must hover for, which is the reason to keep a `Help` to a line or two.

- `Group` listed as a column is rejected by `Compile` rather than flattened —
  flattening would mean defining what a nested group is in a cell.

- A nested `Table` inside a form's `Fields` stays out of scope. If it turns out
  to be wanted, it is solved then.

- **A `Select`'s options may be the app's, read when a request is answered** (`Select.LoadOptions`, in place of
  `Options`; both is a compile error). A select that is edited chooses from what there is now, and a submission is held
  to it, so a value that someone added is accepted at once and one that was taken away is refused, and a form that
  was opened before the change is told so (the 422 draws it again with the options as they are). The same
  options give a table with `Load` its filter (the source filters, so the library can only offer what it is told) and
  say, in every table, form, export and page search, what a value reads and the tone it is drawn in. With `Rows` the
  filter stays the values the rows hold, as for `Options`.
- **A select reads its options when a request is answered, by a function that the field carries.** The field carries a
  function, and each request that needs the options asks for a copy of the form or the table with them read; nothing is kept between requests, since fields are shared, and the app knows how
  long its options may be old, so a slow source is cached by the app, as for `NavEntries`. A source that fails fails the panel
  as a failed `Load` does (a POST is answered with the toast); a value that is empty, listed twice or holds the separator is the
  source's mistake and fails the same way. Counted in the metrics as a load of the kind `options`, only when there was a source
  to read. Not done: letting the visitor enter a value that is not among the options (`Open`), which a `<select>` cannot take
  and which would be a combobox for one value and a tag input for several; a select of several values that grows with what was
  entered is the case that would need it.

- **A `String` can be a `Secret`** (a value that is typed and never read back, such as an API key). It is a flag on the
  accessor, not a type, so its rules and placeholder come with it. The stored value is kept out of the page at one place:
  its `Get` returns `""`, which is what the input, a table cell, search, sort, filters and the CSV and JSON files all read, so
  there is no second place to forget. The form that answers a refused submission would still echo what was typed, which
  it does for every field so that the value does not look reset, so a secret is left out of the echo too.
  An empty submission leaves the stored value (its setter ignores `""`), since the page cannot show the value and the
  visitor cannot be asked to type it again to change something else; so a `Secret` cannot be `Required`, which would say
  both things, and clearing one is not possible, which is the cost: the app offers its own button for it if it wants one. A
  secret needs a `Store`, and is a compile error in a table, an editor and a toolbar (a toolbar shows values, and there is nothing to show).
  The input is `type="password"` with `autocomplete="new-password"`, so a browser does not offer what it saved for the site.
- **A secret that has a value says that it has one, and nothing of it.** That there *is* a value is shown, since an empty box
  reads as "nothing is stored": the input's placeholder is a row of bullets, always eight, so that neither the
  value nor its length is in the page, in place of the accessor's own `Placeholder`, which is shown when nothing is stored. It
  is a placeholder and not a `value` made of bullets, which would be submitted and stored, and the standard way to show text in
  an empty input. The only thing the page learns of a secret is a yes or no, which is got by asking `Load` whether it
  returned something.

- **A `String` can be `Multiline`** (a text area for a description or some JSON), as a flag on the accessor and not a
  type, so its rules, placeholder and `Store` come with it. What a text area cannot do is refused when the app compiles:
  it has no `pattern` attribute, and the text rules reach the browser as one, so a `Pattern` or a text rule on a multiline
  field is an error (check the text in `Run`), and so is one in a toolbar, which shows one line for each value.
  `Required`, `MinLen` and `MaxLen` stay and are the browser's too. A browser sends a line break of a text area as CRLF, which
  its own `maxlength` counts as one character and `utf8.RuneCountInString` as two, so a text that the page accepted could be
  refused by the server; the form's binding reads CRLF as `\n` before it checks and stores, which is also what an app wants to
  handle. A text that wants highlighting, a language service or a schema is an `Editor`, not this. In a table the cell
  keeps the lines (`white-space: pre-line`) and shows three of them; the CSV quotes the rest; a filter, a sort and a search
  read the text as it is.

- **A value that may be absent is an accessor of its own** (`OptionalInt`, `OptionalFloat`, `OptionalSlider`, `Bool`), not a
  flag on `Int`: the Go type differs (`*int`), so `Load` and `Store` do, and a flag could not make `func(M) int` say
  "none". They are four types and not one generic, since the accessors are one type for each Go type already and a generic
  `Optional[T]` would need a constraint that names which of them to draw. Absent is not zero in any place that matters:
  the input is empty and says so, an emptied one is stored as nil (for a field that may be absent; for any other an empty number leaves the loaded value alone), `Required` is what makes one that must be there, a table
  sorts it first and leaves it out of a range as it does a missing moment (it has no number to sort by), and an export leaves a CSV cell
  empty and writes JSON `null`. `Bool` is a select of Yes and No: a checkbox cannot be three-valued without script, and an
  unchecked one is not sent at all, so "off" and "never asked" are one request. That reuses the drop-down, its check on the
  server, the badge, the filter and the CSV labels; its cost is a click more than a checkbox, which is what the third
  state costs. In a `Toolbar`'s `Form` they are values to read, computed from the model by their `Load`.

- **A kind of text that has an input of its own is an accessor of its own** (`Email`, `Phone`, `Website`, `ColorPicker`), not a
  type on `String`. The first version was a typed `String.Input`, and it needed a compile error for every field of `String` that
  it could not be combined with: `Secret` and `Multiline` (inputs of their own), and for a color `MinLen`, `MaxLen`, `Pattern`
  and the text rules. Rules that a type has to forbid by an error are rules it should not have: the accessors have only
  the fields that make sense for them, so those combinations cannot be written, and a `ColorPicker` has no `Rules` and no
  `Placeholder` at all. What they share with `String` (the rules, the placeholder, sorting and filtering as text) is one
  helper, and they are text in a table.
- **`type=` is a hint, and the server is the check.** An email is held to the HTML standard's own grammar, which is what the browser does, so the two agree (it is looser
  than the RFCs, and "a@b" is one); `net/mail` was weighed and dropped for that, since it accepts display names and quoted forms
  that an email input refuses. A web address is held to more than the browser holds it to: a browser accepts any address with a
  scheme, `javascript:` among them, and what is stored is likely to be linked, so the server wants an http or https address with
  a host, and says so in the rules popover, since the browser would not. That is a disagreement, and it is the safe one: the
  page accepts "ftp://x" and the server refuses it, with the same words beneath the field.
- **The names: `Website` and not `URL`, `ColorPicker` and not `Color`.** It is `Website` and not `URL` because `URL` is the
  link, which has no `Store`, reads site paths and `mailto:` too, and is a different thing from a web address to type; one name
  for both would have made a field with a `Store` allow what a link allows. `Color` was taken by the theme's, so the picker is
  `ColorPicker`. A phone number is checked only for being digits and the punctuation of one, since there is no grammar for them.
  A color picker cannot be empty, so an empty `Load` shows black.

- **A phone number is a country and a national number, in the HTML.** The server draws the selector of the calling code and the
  `type="tel"` input that holds the national number, and composes the number from the two when they are posted (`<name>.cc` and
  `<name>`); E.164 is stored, with no spaces. The script, in a file of its own that only a page that edits a number links, only spaces
  the digits as they are typed, by the grouping that the chosen option says; it builds nothing, holds no list of countries, has no hidden
  input, and does not guess a country. E.164 was chosen over keeping the spaces: one number then has one form, so it can be compared,
  searched and dialled, and the spacing is only how it is shown. This is the hypermedia audit's finding of the one script that built a
  control from data and decided what was sent. The costs: about 250 `<option>`s in the HTML of each page that edits a number, where it
  was one blob that the browser cached with the script; the visitor's country is guessed from `Accept-Language`, by the server, which
  makes the page vary on it, and is the region of the language and not where the visitor is; the example that the empty input showed
  for the country chosen is dropped (a field's own `Placeholder` stays); and a number pasted whole with its `+` is read by the server,
  and not moved into the selector by script.
- **The grouping of a phone number is data, and approximate.** The spacing is data, not code: groupings for about twenty
  countries where the national format is known, and threes for the rest, which is approximate and says so; `libphonenumber`
  metadata would be right for every country and is 80 to 150 kB and a licence to carry. The countries are a Go table,
  `internal/phone`, which is also what the server validates `DefaultCountry` with and what the options of the selector are drawn
  from, so what is offered is what the server accepts. The costs: the numbering plan of
  `+1` has more countries than the US and Canada, which are not listed (a number of one of them is typed as `+1 242 …`, and is
  still the right number); a number that is already stored without a code, or with an extension, is not split, since doing so
  would change it, and stays the plain input until it is cleared; and the length and pattern rules of the field are not the
  browser's, because what is typed there is the national part and the rule is about the number that is sent: the
  server holds it to them, in a page that says so beneath the field.

- **`DateOnly` makes a moment a day.** A date and time picker asks for a time of day that a date of birth, a renewal or a deadline
  does not have, and a `Timestamp` of midnight shown as "00:00" is noise. `DateOnly` is a flag on `Datetime` and `Timestamp`, not
  a type, since all else about them is the same: it is `type="date"`, shown and filtered as a date, and a day is midnight
  UTC, so what is stored is Unix seconds of that moment, or for a `Datetime` the date alone, "2026-10-09" (which `Datetime`
  already reads). A time of day that arrives by hand is dropped to the day. A filter's bounds are date inputs; the bound "to" is
  the midnight of that date and not the end of it, which is right for what a day column holds, and a column that mixes would
  want a moment, not a flag.

- **A file is a `File` accessor, and the library keeps no file.** `Store` is handed an `Upload` (a base name that is made safe, a size, the
  content type that was sniffed, and `Data`, the whole of the file, in memory) and copies it where the app
  keeps files; `Load` returns a `*FileInfo` (name, size, URL and a thumbnail: see below) for what is kept. The library stores and serves nothing: where a file lives
  (disk, a bucket, a database), who may fetch it and when it is deleted are the program's, as reloading is. Dropped: a handle that is a path (the library would own a directory), a reader over the spool of `net/http`
  (a megabyte of a file in memory and the rest in a temporary file on the server's disk, which the library did not choose and the app could not turn off),
  and a download route of the library's own (it would own serving, and caching and authorisation with it). The cost: the app writes a
  route to serve what it stored, and the docs say how to serve it safely (`attachment`, `nosniff`, the sniffed type); the demo is the
  working example. A `File` is for a file that can be held in memory, and a file that is too large to be (several gigabytes, on its way to an
  object store) is not a form value at all: it is a `FileUpload`, a transaction of its own (see below).

- **A file too large to hold is a `FileUpload` panel, whose action is handed the file as a reader.** A form field is one value in a single
  transaction (bind, rules, Guard, `Run`, all-or-nothing), which is why a file in a form is held in memory and checked with the rest: a stream
  of several gigabytes to an object store is not part of that, and three designs that put the store behind the field (a writer with `Submit` and
  `Discard`, a typed token with `Commit` and `Discard`, an `UploadSpool` that `Store` is handed a handle of) each leaked how a file is stored into
  every form with a file, and made the form's order the upload's lifecycle. A `FileUpload` is a leaf of its own with its own POST, so none of that is
  there: `Receive` is an `Action[FileHandle[M]]`, the `FileHandle` (name, sniffed content type, kind and a `Reader`) is its subject, and its `Run` is the upload,
  which reads the `Reader` wherever it goes and returns an `Outcome`; the Guard, which is the action's, decides how the panel is drawn (asked with a
  `FileHandle` that has no reader: `Disabled` disables, any other error leaves the panel out) and refuses the post. What `Run` needs of the page it reads with `ArgsOf`. One file per panel, in one request: what the visitor chooses beside it is the panel's options (below). The first 512 bytes are read before the Guard, so that it can see the kind, and the
  `Reader` is held to `MaxSize`, which is required and has no default (the cap is the app's to say, as for a `File`); a file larger than it is refused as
  "Must be at most …", whether `Run` read to the end or stopped early (what it did not read is read by the library, bounded by the limit of the body, so that the
  browser is answered and not reset). Only the kinds that the first bytes tell can be accepted (the images, PDF and any file), since the file is read once, by
  `Run`: JSON, CSV and text are told by all of the file, and are a `File`'s. Dropped: the same panel as a form with `Open`, `Commit` and `Discard`; a
  generic panel of the page's arguments; several files in one request (a result and a failure for each, and partial success); a progress bar and
  upload by script (below). The costs: a visitor that the Guard refuses has only the first 512 bytes of the file read, and a browser that was sending a
  large file may see the connection closed; a `Run` that fails after a partial write to a store leaves it for the app to clean up, which the library cannot,
  not knowing the store; and the browser's own load indicator is the only progress, since the input stays the control and the post is the browser's own
  (the earlier decision), which the script below reverses for this panel.

- **What is chosen beside the file is the panel's model: `FileUpload[M]`.** The first version had no inputs and was not generic, since a panel's type parameter is its loaded model and
  it had none; an app that moved to it needed a choice beside the file ("replace if it exists", a storage class, a folder) that was made before the file was sent, and a second step
  after the upload cannot be. So the panel has a model, the options, as a form has: `Load` gives the defaults (optional), `Fields` are the inputs, any accessor of a form with a `Store`
  and rules (a `File` among them is a compile error), and the type parameter is named `M` as on every other leaf. They are lowered, bound (`bindFunc`) and drawn (`formField`) by the
  form's own machinery, and are an `ir.Form` with a placeholder Submit, which is what makes its fields inputs. The fields come before the file in the page, so the request sends them first:
  they are bound, and checked by their rules, before any of the file is read, a refusal answers like a table action's (a toast of the messages) without reading the file, and the options
  are `FileHandle.Options` for the Guard and `Run`. A field that arrives after the file is ignored. Dropped: only page arguments (a visitor cannot choose with them on the page that
  uploads); a fixed kind of extra input (a bool or a select) of the library's (it would decide what apps choose); and options that can be read after the upload (the choice cannot
  change what is already being written). The costs: **an upload with nothing to choose says `FileUpload[struct{}]` and `FileHandle[struct{}]`**, since Go cannot default a type
  parameter, a break from the version that was not generic; the Guard is asked after the options are bound, so a visitor it would refuse sees a field's message first when they post by
  hand without the fields; and the options are the visitor's word, which `Run` checks as it does anything else.

- **A file of no bytes is a file.** A part that has a filename is a file that was chosen, whatever it holds: a chooser that was left alone sends no filename, and a post with none is
  answered "Choose a file.", where the first version answered an empty file the same, which told a visitor who had sent a folder with `installer.yaml` in it, empty, to choose a file.
  Run is handed an empty file as any other, with a reader that is at its end, and an app that has no use for one says so in its own words; a panel that takes only some kinds says "This file
  is empty." and not that a kind is to be chosen. A `File` in a form, where an empty part is what a chooser that was left alone sends in some browsers, still takes it for nothing chosen.

- **A `FileHandle` says what the browser said, apart from what the library made safe.** `Name` is the base name, made safe, and is never a path; `Path` is the folder that a
  file was sent in (a visitor who chose a folder), made safe by a rule that never lets it leave the folder the app puts it in (both slashes separate, empty, `.` and `..` elements are
  dropped and a `..` is not followed, controls are taken out, elements and depth are bounded); `RawName` is the name exactly as sent, so that an app can tell the visitor that it
  changed; `DeclaredType` is the media type the browser said, without parameters, beside `ContentType`, which is what the first bytes hold, since the visitor often knows better
  what an object is (a `.json` file sniffs as `text/plain`). Go's `Part.FileName` applies `filepath.Base`, so the raw name is read from `Content-Disposition`. The costs: three
  more fields on a type every upload sees, two of them the visitor's word and to be trusted with nothing; and there is **no test helper** in the public API: the names that a test posts (`_leaf`
  and `file`) are documented as stable instead, which `httptest` and a few lines of `mime/multipart` can use.

- **A file is of a kind by what it holds.** `Accept` lists kinds (`FilePNG`, `FileJPEG`, `FileGIF`, `FileWebP`, `FileImage`, `FilePDF`,
  `FileJSON`, `FileCSV`, `FileText`, `FileAny`), and the server reads the first 512 bytes with `http.DetectContentType` for the
  binary kinds and the whole file for JSON (exactly one value, streamed), CSV and text (UTF-8, no NUL). The type that the browser
  sends and the file's extension are the visitor's, so they are a hint (`accept=`) and nothing else: a PDF named `.png` is a PDF. There
  is no SVG kind: SVG is XML that a browser runs, so an app that takes it says `FileAny` and owns the risk, and `FileText` does not take
  HTML or XML for the same reason.
- **An upload says what it is twice: a media type and a kind.** An `Upload` says what it is twice, and they are not one value:
  `ContentType` is a media type (`image/png`, `text/csv; …`, `application/octet-stream`), which is what to serve it as and
  cannot be a `FileKind`, since a file of no named kind has one too; `Kind` is the named kind it was found to be, for an app
  that takes several and wants to branch (`FileAny` for none, which only a field that takes any file lets in, and never
  `FileImage`, which is four kinds). It is the most specific of the kinds in `Accept` that the file is, by a fixed order
  (a fixed order of the library's: the images and the PDF, which exclude one another, then JSON, then CSV, then text, then any),
  and not the first listed, which would make what an upload is depend on how the app happened to write `Accept`; a JSON file is
  also text, and `42` is JSON and CSV. The order is the library's and is not API, since an app that wants another can say it
  with the kinds it lists. The cost: a CSV is any text that parses as records (a line of words is one), which sniffing cannot
  tell apart; a text file with a byte-order mark or in UTF-16 is not text; and `FileAny` knows only what was sniffed.

- **A `FileUpload` with script sends its files one at a time, with a progress bar, and the server answers each in HTML.** Several files, a folder, a result for each file
  and a progress bar are one feature, because they all need what a plain form post cannot do: a request for each file that script makes. So `upload.js`, linked only by a page
  that has a `FileUpload` and after `file.js`, takes over the form's submit: for each chosen file, in order, it sends the form's controls and then the file (with its
  folder in its name, which the server reads as `Path`) with an `XMLHttpRequest`, for the progress events that `fetch` has none of, to the form's own address, with the
  `X-Webui-Leaf` header that a request for one panel's HTML carries. A POST with that header is answered with **HTML, never data**, and the status says how it went (below): one list item with the outcome's
  tone and text and, when the outcome goes somewhere, a link to it, written at the places where a flash and a redirect are, so every other post is unchanged. The script
  puts the item in the file's row as it is and reads nothing from it but that link, and whether a file went well is its status: when every file was a success and the last
  outcome went somewhere it follows the link, as a browser that was redirected would, and the panels around it (tables) are asked for again from the
  server by a `webui:changed` event that `enhance.js` answers, so that what was just sent is in them. A `Cancel` aborts the request in flight and marks the rest "Not
  sent". `Multiple` is the one option, and a folder is not a second: it adds `multiple` to the chooser (by script, never in the markup, so that without script a post
  never drops files that were chosen), a *Choose folder* button on a hidden `webkitdirectory` input, and the expansion of a folder that is dropped with
  `webkitGetAsEntry`. This **reverses "the input stays the control" for this panel**: its post, with script, is the script's request and not the browser's own form post,
  and the two are kept in step by sending the same controls in the same order; the post without script is still the browser's, and is what the server tests use. Dropped:
  several files in one request (a result for each, partial success, and no progress either); a JSON answer that the script would turn into markup (the server answers script with
  HTML, which the decisions log opens with); and a reload after the last file (it loses the results, and a refused file is the thing to read). The costs: a second way to send
  a form to keep in step with the first, for this panel only; a script of about 300 lines with no test harness, whose behaviour is checked in a browser by hand and its tokens by
  the tests; a progress bar per file and not of the whole queue; and a request for each file, which for hundreds of small files is slower than one.

- **A post that script makes is answered in HTML, for any panel, with the status of how it went.** The first version of the upload's answer was the upload's: a list item that
  any other post with the header would have been given with no place to go, a form that was refused was given the whole page, and the script told success from failure by the
  word in the item's tone. It is one contract now, for every leaf. A post with `X-Webui-Leaf` is answered with HTML and never with a redirect, a cookie or data: **an accepted
  action is the item with a 200** (a warning too: it was done), **a refused outcome, rule or file is the item with a 422** (understood, and refused), **a Guard's refusal is the item
  with a 403** (the visitor may not, which is not that what they sent was refused), **a failure is the item with a 500**, and a refused page Guard, a post from another site and an
  argument that did not parse are the item with their own statuses; **a form that was refused is the form**: the HTML of the leaf, drawn again with what was typed and what was
  refused, and nothing round it, as a request for the leaf is answered, with the 422. The item is `li.result` with `data-tone` for how to draw it, `.result-text` and, when the
  outcome goes somewhere, `a.result-next`: a link that is the next action, which the script follows and has not to know. So a script reads the status that HTTP has for it and the
  link that the HTML has, and never a tone, and an app's tone is for the eye. What the script still says itself is what only it can know: "Not sent." for a file that a *Cancel*
  stopped before it was sent, "Cancelled." and "The upload could not be completed." for a request that did not reach an answer, which have no response to be the server's words. The
  panel's form is told by plain attributes (`data-upload` marks it and `data-multiple` says that it takes several files), and not by JSON that the script parses. The costs: a
  status for each way it can go is more than a 200, and a script that sends a form (there is none yet) has the form's HTML to swap in and the item to read.

- **A link says whether it replaces its panel: `data-in-place`.** The script followed any link in a panel that led to the page's own path by replacing the panel,
  which it guessed from where the link led, so the markup of a link did not say what following it would do. The server declares it now, on the links that are the
  panel's own: the sort links of a column, the pager's buttons, a filter's *Clear*, the ways on from an empty table and a failed panel's *Try again*. The script follows
  those in place and every other link is a navigation, which is what a link is; it still refuses an address that is not this page's, since the panel is asked for at
  the same path. A panel that reloads says so with `aria-busy`, which the stylesheet draws as the rows pulsing and out of reach: the rows are the ones that were there
  (the script no longer builds a skeleton table, and a request that fails leaves them as they were). The search box's words are the page's own: *Searching…* is in a template that the
  server wrote beside the box, a search that could not be made is the server's fragment with a 500, and a request that got no answer at all says "The search could not
  be reached.", which is not that nothing matches. Left as it is, on purpose: the filter's submit still builds the address from its fields, dropping the empty ones, which
  a browser without script sends and the server tolerates. The costs: an attribute on each of those links, which a new kind of link has to remember; and no
  link that replaces its panel can be added without it, which is the point.

- **A file chooser has script for a drop, a picture and an early word, and the input stays the control.** `file.js`, linked only by a
  page that has a file to choose, as `phone.js` is for a number, puts the chooser in a drop zone and puts what is dropped in the input
  (`input.files = ...`, from a `DataTransfer` of the first file), so the request is the browser's own multipart post, which a `fetch` would
  have replaced with a second way to send a form to keep in step with the first. The picture of a chosen image is read with
  `FileReader.readAsDataURL`, and not `URL.createObjectURL`: the policy lets an image be `'self'` or `data:`, and a `blob:` address would
  have meant widening it for this.
- **The picture of a chosen image is a `data:` address, small, and never SVG.** Only PNG, JPEG, GIF and WebP are drawn, and only
  up to 4 MiB, since a data address is held in the page (SVG is a document that can run script, and is not drawn). The size and
  the kind are told to the script by the server in plain attributes of the input (`data-max-size`, `data-max-message`, `data-kind-message` and `data-image`, with the kinds in the input's own `accept`), in the words the server uses, through `setCustomValidity`, so
  the browser's own message appears and the form does not submit. The kind check is a hint: it goes by the type that the browser
  gives and the extension, and a file that has no type is not refused, since a wrong refusal here is worse than a right one
  late; the server is the check. The cost: a page of a form that a visitor may only read still links the script, which is the
  page's and is not asked per visitor, and has nothing to do; a drop of several files takes the first; and `accept=` now has the
  extensions as well as the media types, so that the script can tell a file by its name. A file that is chosen and not sent has a *Clear* button; the *Remove* of a stored file is a checkbox, which needs no script.

- **The stored picture is a thumbnail that the app made, and the full image is fetched only when it is clicked.** Showing the stored file
  as the picture at 32 pixels sent the visitor the whole image (a 4K photograph, on every load of the form) to show a few pixels of it.
  `FileInfo` has a `Thumbnail`, a path on this site, which the page shows (in the markup, so it needs no script) as a button that opens
  the file at full size in a native `<dialog>`, by an invoker command (`commandfor`, `command="show-modal"`), which is what `Confirm` does and so has the
  same costs: Chrome and Edge 135, Firefox 144 and Safari 26.2 without script, and `enhance.js` for an older browser. The full image is an `<img loading="lazy">` in
  the closed dialog, which has no box and so is not fetched until the dialog opens (measured: not loaded before the click, loaded after, with all script removed
  from the page). A background image on a placeholder was weighed: it defers the same way, since a closed dialog is not rendered, but it has no size of its own,
  so a 64px logo and a 4K photograph would fill the same box (a `contain` that enlarges the one and fits the other), no alt text, and the address in a style attribute.
  The image has its size, its alt and no inline style.
- **The library makes no thumbnail, and offers `webui.Thumbnail` to the app that does.** The library keeps no files, so it
  cannot make the thumbnail itself: the app does, when its `Store` is handed the image, and `webui.Thumbnail(up, size)` is there
  for it (PNG, JPEG and GIF, to a PNG that keeps the picture's proportions and is never enlarged, with the picture of no more
  than 25 million pixels, since it is held whole in memory and a small PNG can be a vast picture). Dropped: showing the file
  itself, which is what was wrong; the library keeping a thumbnail beside every upload, which is keeping files; and a WebP
  reader, which is an image dependency for every program, so a WebP has no `Thumbnail` from the helper, and the name stands
  alone unless the app scales it with its own. The cost: an app that wants a picture makes the small one and a route to serve
  it; there is no picture without one, which is the library's choice and not a gap; and the picture of a file that is chosen is
  made in the page, in a canvas, from a data: address of the whole file, which is held only as long as it takes to draw it, and
  read again when the dialog is opened.

- **A form that has a file is multipart, read after the page's Guard, once, into memory.** The form's `enctype` is `multipart/form-data` only when it has
  a file field, so no other form changes. The page's body limit is the sum of its files' `MaxSize` and the usual megabyte (the library sets no ceiling on a `MaxSize`: how large a file may be is the app's to say, and an early version that refused more than 256 MiB was dropped for that; the sum saturates at the largest size there is, so that an app that says "as large as it likes" for more than one file has no limit, and not a negative one), for a
  multipart body only; an urlencoded post to the same page keeps the usual one. The page's `Guard` runs before the body is parsed,
  so a visitor it refuses has nothing read from them, and a body over the limit is "That submission was too large." The body is read once, in
  order, with `MultipartReader`, and not `ParseMultipartForm`, which keeps a megabyte in memory and writes the rest to a temporary file: the values go to
  the form, and each file is read into memory (its first 512 bytes first, so that a file of the wrong kind is refused before the rest is held) and
  checked, so nothing of a submission is written to disk. A file that is refused is not held, and the rest of it is read and thrown away, since a
  browser that is not read to the end resets the connection and the visitor sees that instead of the reason; the other parts are still read, so the form
  is drawn again with what was typed. An action's own `Guard` runs after `Bind` (it is handed the model), so for a refused action the file has been received
  and is dropped with the request. The costs: memory for each upload that is arriving is up to its `MaxSize`, so the app sets one that it can hold
  for as many visitors as send at once; `_leaf` names the form that the files are of, so it must come before them, as the markup of the library has it, and a
  body that has it after is "not understood". A program that takes uploads sets `ReadTimeout`, which the demo does, since `ReadHeaderTimeout` does not bound a body.

- **A file is stored only when the whole submission is accepted.** `Bind` checks every field and applies each value that passed,
  but a file is the one value that is held back until nothing is refused: a form that another field refuses has stored no file, and
  shows the stored one again. A browser never fills a file chooser in, so the file that was chosen is lost either way; the field says
  "Choose the file again". Dropped: keeping the upload on the server between the two requests (state to expire, and to clean up
  behind a load balancer), which is what the 422 design avoids for everything else. `Required` is met by a stored file that is not
  being removed or a new one; a new file with a removal is a replacement. A `File` is a form input and a table column, and not a toolbar item: a toolbar shows values, and a file has only a name to show.

## Guards and refusals

- **Every state that stops a visitor is HTML with the way on in it.** The hypermedia audit found states that were a line of plain text or a message with nothing to follow, and each is
  now the page of the state with the links that the state offers, written by the server. A method an address does not take is a 405 in the shell, with what it does answer and a link to open
  it; a post from another site is a 403 in the shell with a link to the page (and a fragment of the same words for script); the 400 for an argument that did not parse says so and leads on to
  the page without its query, and no longer tells the visitor to edit an address; a 500 leads on to the same address, to try again; a page whose render failed so badly that no shell could be
  drawn is the smallest page of HTML with a link, not a line of text. A panel that failed has a *Try again* link to the page's address, and a table that shows nothing has the way on that the
  reason offers: *Go to the first page* when it is past its end, *Clear the filters* when some are set. A script's post that a page's Guard refused is answered with the refusal's words
  and its `Href` for any status, and not only for the 403. A breadcrumb parent and a form's *Cancel* and *Back* are asked of the Guard of the page they lead to, as a toolbar link and a row
  link are, so that none leads the visitor to a 403 that nothing on the page had said: a refused parent is its words and no link, and a way back that is refused is not drawn. A control that
  its Guard disabled gives its reason as text and not only as a tooltip on hover: the wrapper takes the keyboard focus (which shows the tooltip as hover does), and a screen reader reads
  "Not available: reason" after the button; and a form that is only read says why in its footer when its Guard said `Disabled`. Left as they are, on purpose: the pager's disabled *Previous*
  and *Next* stay drawn (the buttons keep their places, which "nothing moves under the pointer" asks for, and a disabled button is announced as one), and a source that does not know its
  total can still show a *Next* that leads to an empty page, which the table then says and leads on from. The costs: more markup in the states, and the in-process Guard check that a link
  asks is paid for each breadcrumb parent on every page that has one.



- **What a visitor may be told is a `Refusal`, a typed error with a status.** A `Guard`'s error was always a 403 with its text, and a
  `Load`'s was "Could not load" with the cause hidden, since either may name what a visitor should not see; so an app that
  wanted a 404 for a record that is not there, or a 401 with a link to set up a credential, had no way to say so. `NotFound`,
  `Unauthorized` and `Forbidden` make a `Refusal`, `WithLink(label, target)` gives it a link (the `Target` that `Open` builds, read
  inside `pkg/webui`, since it has no parts to read outside; not called `Then`, which on an `Outcome` redirects the user
  and here would only offer a link that they choose to follow), and it is found with `errors.As`, so a wrapped one counts.
- **A refusal is the page's status in a `Guard`, and is said in place in a panel.** A refusal is read where the page's `Guard`
  and the loads of a table, a form, a diff, a markdown and an editor are lowered, which is the one place a type assertion
  belongs. In a `Guard` the status is the page's, and the page draws the state of that status (the sidebar, the search and the
  links only ask whether there is an error, so they are unchanged, and a POST to a refused page is redirected to the page that
  says it, as before). In a panel's `Load` the page has its status by then (a page that streams has sent it), so the panel says
  the message and its link in place of "Could not load", and the page stays 200: said in the documentation and not hidden. A
  refusal is the visitor's matter and not the app's failure, so it is not logged; an error that is anything else is as it was.
  Not done: a status for a panel's refusal, which would make a page that has begun to be sent a lie, or buffering every page to
  be able to.

- Row actions are decided at render, per row, by the same `Guard` that authorises the POST. A bulk action's `Guard` receives the
  selection, which does not exist yet: it is asked of each row as a selection of one, and the button is drawn if it accepts any (see
  *A Guard's result decides how its control is drawn*).

- **A Guard's result decides how its control is drawn.** An action's `Guard` returns an error to refuse, and what the error is
  says the rest: an ordinary one leaves the control out; `webui.Disabled(message)` draws it disabled, with the message in a
  tooltip. Either way the same `Guard` refuses the POST, and for a request made by hand its message is the toast. It applies to a table's row
  actions, its bulk actions, a toolbar's buttons and a `Link` (which is left out when the page it leads to refuses the visitor, the way the
  sidebar leaves out an entry).
- **What was dropped beside the Guard's result, and what it costs.** Dropped: a field on `Action` (`HideWhenGuarded`,
  `DisabledMessage`) for what the `Guard` already knows, since a field is a second place to say it and cannot depend on the row;
  one rule for every control, hiding or disabling (hiding is right for a viewer who has no business with an editor's buttons,
  disabling for "Already granted", and no rule is right for both); and a separate "disabled when" condition on `Action`, which
  is the same `Guard` said again. The costs: a left-out control cannot be discovered, and its reason is seen only on a hand-made
  request or a refused page; a bulk `Guard` must accept a selection of one, and is called for each row of the window, for each
  bulk action, whenever the table is drawn, as a row action's is (a `RowClick` `Link` asks the destination page's `Guard` once
  for each row, which for a `Guard` that reads a database is a read for each); the column of row buttons is there when some row
  of the window has one, so it comes and goes between the pages of a table; a `Link` is left out whichever error the
  destination's `Guard` returns, since `Disabled` has no meaning for a page; a `RowClick` action that is refused leaves the row
  shown and doing nothing, since it has no button to leave out or to disable, whichever error says so; and a bulk action whose
  `Guard` refuses every row is disabled with the first message that was given, or left out if none was. The Save of a form is
  read-only for either (a form that cannot be sent would only invite typing). A `Guard` that panics is an application bug, which
  leaves the control out and is logged, since a page must not fail for it. A table with no row actions and no bulk action drawn
  has no form, no checkbox column and no bar, and a toolbar with nothing drawn is not there.

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
- An accepted POST is answered with a `303`, so that a reload of the page it leaves the visitor on
  never sends it again (the browser would ask, and for an action that already ran, a yes
  would run it twice). Its toast rides a short-lived `HttpOnly` cookie. The same goes for what
  is not about the form's fields: an error in `Run`, an action `Guard` that refuses, a request
  that is not understood and a body that is too large each go back to the page with a toast, and
  a page `Guard` or argument error on a POST goes to the same address, whose GET draws the real 400
  or 403. The one exception is a cross-origin POST, which stays a plain 403: a forged request is
  never redirected. POSTs are checked with `http.CrossOriginProtection` (Fetch metadata and
  `Origin`); behind a proxy that rewrites `Host`, wrap the handler and allow the public origin.
- A refused form is answered with the page and a `422`, not redirected. A `Reject`, a `Failure` and
  a rule that the input fails mean that nothing was done, so a reload that sends the POST again
  does no harm, and the browser's own question about it is the right one. The response draws the
  form with the model as bound so far, the values as typed, the field messages and the toast, as
  the answer to the POST, so nothing is kept on the server or put in a cookie. Every other way to carry
  them across a redirect is a convention of its own, and was weighed and dropped: the address (it
  would put what was typed in history and logs), a cookie (it goes with every request to the mount, is
  limited to a few kilobytes while an editor takes 16 MiB, and sends what was typed to the server again
  and again), and a store on the server, which is what this replaced: it is per process, so behind a load balancer without stickiness the second request could reach another instance, it needed a bound and a timer, and it made a refusal show as a 303 in the metrics.
- **What a refused form that is not redirected costs.** The browser asks "send the form again?" when the page it answered with is reloaded,
  which is its standard behaviour for a response to a POST, and with script there is no workaround for it on purpose. A table
  action that is refused has no fields to draw again, so it keeps the redirect and a toast. A `Then` or a `Back` that leaves the
  form still takes the user there, with the message as a toast, and what was typed is not kept, as before.

- The re-render after a rejection must echo the raw submitted input, not the
  model value, or the user's bad input disappears and the form looks like it
  reset.

- **An action may ask first, with a native `<dialog>`.** `Action.Confirm` puts a question between the click and the
  request. A `<dialog>` opened by an invoker command (`commandfor`, `command="show-modal"`) needs no script, which is the rule
  for everything here, and gives focus handling, Escape, the top layer and a backdrop for free; the confirming button is
  a submit button inside the dialog, which sits in the form that the action's button was in, so the request is the one
  the button would have made, with its `_act` and nothing else new on the server. `window.confirm()` was weighed and
  dropped: it needs script, cannot be styled, and is blocked in some embeds. The costs, in the docs: invoker commands
  need Chrome and Edge 135, Firefox 144, Safari 26.2, so `enhance.js` adds a small click handler for an older browser, and an
  older one with no script cannot open the dialog, and so cannot do the action (it fails closed, which is the safe way; a page of its own to confirm on, reached by a link, was weighed in the hypermedia audit and dropped, since it would serve only a browser that is both without commands and without script, and would add a page for each confirming action and a way to carry a bulk selection in an address);
  a form whose Save asks is not submitted by Enter in a field, because Enter clicks the form's first submit button, which
  would be the one in the closed dialog, so a disabled one is put ahead of it; and it asks the visitor, not the server, so
  a hand-made request is not asked and a Guard is what refuses. A row action's dialog is named by the row's position and the
  action's, never the key, so nothing a row says is in an id. A `RowClick` action has no button and may not have one.

- **A job is followed by the panel that shows it, which says that it is not final, in the value its `Load` returns.** A button that starts something long
  was first thought of as an action result that says "ask me again" (a `RetryAfter`), which the client would answer by posting the same action again. That makes
  the action something that must be safe to repeat, a POST used for what a GET does, and a cookie that has to carry the form of the retry; it was dropped for
  the view saying it: the action only starts the job and returns `Success`, and the panel that shows the job returns `webui.Ready().RefreshAfter(d)` from its
  `Load` while it is not done.
  A `Load` now returns a `Loaded` beside the model, in place of the `error` it returned, as an action's `Run` returns an `Outcome`: `webui.Ready()`,
  `webui.Failed(err)` (a `Refusal` in it is still the visitor's to be told, and a failed load does not refresh), and `.RefreshAfter(d)`. The error was a
  native return value that could say only that it failed; a value of our own can say more as the library does, and `Loaded` is where the next thing a load can
  say goes without another break. It is the return of `Table.Rows`, `Table.Load`, `Table.Feed`, `Form.Load`, `Diff.Load`, `Markdown.Load`, `Editor.Load`
  and `FileUpload.Load`, and of the rows that `SearchRows` takes, so that one function serves a table and its search. A source of options, an action's
  `Run` and an accessor's projection keep what they return. The panel (only its own, since panels are built at once and each has its own hint in the
  context) says so in its head and in `data-refresh-after`, the delay in whole seconds; with script `enhance.js` asks for that panel again after that time, with
  the header that asks for one panel, and replaces it, and the answer says so again or does not, which is what ends it. Without script the head says
  "Updating, reload in 5 s" and the visitor reloads. This is the standard polling idiom of hypermedia applications, and the loop that was wanted: the
  request that asks again can say it again. The delay is at least a second. The script waits while the tab is hidden and while focus is inside the panel,
  which a reader or someone typing would lose, and a failed request ends it with the panel as it was. The costs: every `Load` of every application is
  changed once (`return x, nil` is `return x, webui.Ready()`, and an error is `webui.Failed(err)`), where `return m, err` of a store no longer passes through
  as it is; the server is asked for as long as a page is open and visible (visitors × panels ÷ interval), with no cap of its own, so a `Load` that never stops
  saying it is a fault of the application; a panel with a form in it is replaced when nothing in it has focus, so it is for panels that show state; a
  a toolbar says it through a `Form` in it; and a job that no panel shows has nothing to poll, so the application adds one.

## Toolbar

- **A `Toolbar` is a row of controls for the page, with no type parameter, and its values are a `Form` that it loads.** It is a leaf and not
  a panel (no frame), in a `Stack` with what it is for. It was `Toolbar[A]`, typed by the page's arguments, with the arguments handed to its `Action`s and
  `Link`s and read by its accessors, but a type parameter of a leaf is the model that the leaf's own `Load` fills and the leaf then reads, and a toolbar had
  no `Load`: the arguments were passed through it, which is what `ArgsOf(ctx)` is for, and the values it showed were computed from them or from a global, with
  nothing to say that they were still changing. Now the buttons are the existing `Action[NoArgs]` and `Link[NoArgs, B]` (no `ToolbarAction`, no
  `ToolbarLink`: a button with no model has the empty one, and reads the page with `ArgsOf`), and the values are a plain `Form[M]` in the toolbar's item list,
  which already has a `Load`, fields and a title. The item list is a sealed interface, `ToolbarItem`, whose method takes `NoArgs`: `Action[M]`, `Link[M, B]`
  and `Form[M]` have such a method, and `Action[M]` and `Link[M, B]` match it only when `M` is `NoArgs`, so a button with a subject of its own does not
  compile (the accessors have no such method any more, so an accessor is not an item). The toolbar is the leaf, and so what is refreshed: a `Form`'s
  `Load` returns `Loaded`, and the shortest `RefreshAfter` of the forms makes the toolbar `data-leaf` and `data-refresh-after`, without a note: a panel says "Updating…" in its head, but a toolbar's row has little room
  and an inline note moves what is in it, so a visitor without script is not told to reload one. A form in a toolbar is a group (`role="group"`, named by its `Title`, which is required), is read-only (no `Submit`),
  has no `Desc` or `Help` of its own and no field that is multiline, secret or a file; the compile says so, with what to do. A form whose `Load` fails is
  its title and "Could not load" (or a refusal's words) in its place, and the rest of the toolbar stands, where a failed read of a select's options still
  fails the whole toolbar. The costs: every toolbar was changed once (the type parameter, the accessors, the subject of the buttons); a `Form` in a
  toolbar is a restricted use of a general type, checked when the app compiles; a slow `Load` holds the whole toolbar, as a panel's holds its panel; and
  the toolbar now has a `data-leaf`, so `syncPanels` asks for it again after a panel or tab moves, which also keeps the address its buttons post to right.
  Writable fields, a `Form`'s `Submit` drawn in the toolbar, are not here.

- **A toolbar shows values and has no inputs.** It once had inputs that wrote the page's arguments, and they were removed outright: an input that changes
  the page needed what a `Form` already is (a form and a request path of its own, a way of reporting a refused value, and script to apply it, or a
  button without), and a toolbar of values with no button still carried a form. What remains is what a toolbar is good for: buttons, links, and values to read. An accessor in a
  toolbar's `Form` is its label and its value as a read-only form field shows it; its `Store` is ignored, as a table ignores it, so
  the one accessor declaration serves a form, a table and a toolbar's form (a compile error for a `Store` there was weighed and dropped for that: it
  would have forced a second declaration of any accessor shared with a form). A page whose arguments the visitor sets is for a `Form` panel,
  or for links to the page with other arguments. The cost is that a page's argument that only a link sets, such as a window, cannot be
  chosen from the toolbar of the page. Because a value is only read, the accessors that cannot be a form's input (`Optional*`, `Bool`, `URL`) fit a toolbar's form too,
  computed from the model by their `Load`; `Secret`, `Multiline` and `File` are refused (nothing to read, one line of text, and an input).
  A toolbar with a button has one form, which posts the buttons, and one with only links and values has none.

- **A toolbar has two groups, `Left` and `Right`, and they are one row.** A gap between the groups takes what the page has
  to spare, so the right ones are at the right edge. When there is no room for every control in a row the gap goes and
  the controls flow from the left in rows, the left ones first, as one group; the right group does not wrap as a unit onto a line
  of its own. CSS cannot ask whether the content fits, so a few lines of script measure the controls as they are (their widths
  and the gaps between them, which do not depend on the layout, so the answer does not flip with it) against the toolbar's width, and
  set a class that removes the gap; the toolbar is watched as it resizes. Without script the gap stays, which wraps a little less
  well and works. Not done: a breakpoint in CSS, which would be right for the toolbars of one app only.

- **A toolbar's buttons post, as every action does.** The request names the toolbar and the position of the item, and
  the same step runs the `Guard` and the `Run` as for any action, so a button that the `Guard` refuses is left out or disabled with the reason, and the request is
  refused with it. A failure, or a reject, is a toast, as for a table action, since there are no fields to point at; a
  success returns to the address it was at, with the toast. Not done: buttons that take input of their own (a popover with fields over a new model),
  which `Form` on a page of its own already is, and which the toolbar's `Link` leads to. Also not done: a `Toolbar` field of
  `Table`, which would put the same items in a table's header beside its title.

## Tables, filters and view state

- Pagination and sorting are library-owned view state. A table always pages, 25
  rows a page unless `Table.PageSize` says, since one that listed every row would
  grow without bound, and every column is sortable and filterable unless its accessor says `NoSort` or
  `NoFilter` (see Accessors).
- **A table's rows come from one of three fields.** `Rows` returns every row and the library filters, sorts and pages them from the columns' own accessors, a number as a number and text
  as text, so a table over a slice has no query code at all. `Load` is for a source that pages itself: it receives a `Query` and
  returns `Window{Items, Total}`, doing the work itself; `Total` below what has been shown means unknown. `Feed` is for a source
  that pages by a cursor: the rows come in the source's order, and it is handed the cursor and a page size and returns the rows
  and the next cursor. A cursor continues one sequence, so there is nothing for the library to filter: the table has no filters,
  only First page, Previous and Next, and says which rows it shows.

- **A feed sorts only the page in hand.** Its columns sort the page in hand and no more: the library orders the rows `Feed`
  returned, with the same comparison `Rows` uses, in the query that built the page, and the source is not told. The sort is view
  state in the address like any other, so Next and Previous keep it, and each page is ordered within itself; the sort link stays
  on the page, since the cursor is the way back to it, and the footer says "sorted within this page" so that the order is not
  mistaken for the sequence's. What this costs: a sort by a column does not find that column's smallest value in the sequence,
  only on the page, which is all a cursor can honestly offer; a source that can sort should be a `Load`. The export of a feed
  ignores the sort, since it is the sequence from its first page to its last. The address carries the cursor, the first row's
  number and the pages before, so nothing is kept on the server; Previous goes back as far as the address remembers (a bounded
  number of characters, the oldest forgotten first). It is a source of its own, not a mode of `Query`, so a table can never
  carry both a cursor and an offset. Exactly one of the three is set, which `Compile` checks. Setting a filter returns every
  table to its first page.

- Column filtering is library-owned view state, like sorting. Hovering a
  header shows a filter icon; it opens a form that is a plain GET, so it works
  without script. A column with a fixed set of options — a `Select`, whose options
  are the ones it lists, in that order — gets a multi-select of exactly those, a numeric
  column (`Int`, `Float`, `Slider`) or a moment (`Datetime`, `Timestamp`) a minimum and a maximum, and every other
  column a text input.
- **What a `Load` is handed of the filters.** `Load` receives them in `Query.Filters` (the options chosen, or the one text
  typed) and `Query.Ranges` (a `Range{Min, Max *float64}`, inclusive, either end optional), keyed like `Query.Sort`. A number is
  compared as a number, never as text: "5" is not a way to ask for more than 5. A chosen option that is not an option, an empty
  text, a bound that is not a finite number and a column the table does not have never arrive, and a typed filter is cut at 200
  characters. A multi-select is repeated parameters (`devices.filter.status=a&devices.filter.status=b`), which is also what a
  form of checkboxes submits; a range is `devices.min.occurrences=300&devices.max.occurrences=500`. Nothing on the page sets its
  query arguments: they come from the address (a link, `Open`), and a `Toolbar` can show their values (a `Form` that reads them with `ArgsOf`) but not change them.

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
- `Rows` is filtered and sorted by the library from the accessors' own values (`internal/tablequery`); `Load` is the escape hatch. A column is known by its `Label`
  in the address and in `Query`, lower-cased with dashes in the address; there is no
  key to declare, and `Compile` refuses labels that collide.

- Export is the table's, and opt-in. `Table.Export` puts two links in the footer, CSV and
  JSON, that download every row the sort and filters keep. It is a declaration and not
  a default because it is a way to take all of the rows at once, and a table that
  does not say so offers nothing and ignores the parameter, as for any leaf the page
  does not have. The address is the page's own with `<table>.export=csv|json`, so
  it runs the page's Guard like any request, and it never appears in the links of the
  page the visitor then sees. The parameter is read from the request and is not one
  of the table's view keys, which is what keeps it out of them.
- An export reuses the table's own `Load`, which is the one thing the library can rely
  on. `Rows` is asked for everything, with no limit. `Load` is asked for 500 rows at a
  time from `Offset` 0, with the table's sort and filters, until a window comes back
  empty or the `Total` is reached, or a cap of 100,000 rows.
- **An export stops a source that does not end.** A source that ignores `Offset` and has no `Total` would hand out its first
  batch for ever, so a batch that is the same as the one before it, row for row, ends it. It is the whole batch and not its
  first row that is compared: a table whose rows are not unique has batches that begin with equal rows, and cutting those short
  would drop data without a word. `Feed` is followed from the first page to the last, ending at a cursor seen twice. An export
  that may be incomplete says so in the page, not in a header that nothing reads: the table's footer says *cut at 100,000 rows* beside the export links whenever the table has more rows than the cap, or does not say how many it has (a note in the file would be data). It is a note of the table and not of the download, since a download is a link whose response the page never sees; a source that repeats itself is a fault of the source and is logged, and the table does not know of it. The whole file is built in memory before it is
  written: a failure half way could not change the status, and the cap bounds it.

- What a cell holds is decided once, for both files. CSV is the text the table shows, so
  it is what the person saw, except that a number is a number, an address is the address
  and not the words that link to it, and any other text that starts with `=`, `+`, `-`,
  `@`, a tab or a carriage return gets a quote before it, as spreadsheets run such a
  cell as a formula. A number is left alone, so that a negative one is not turned into
  text. The file starts with a byte order mark, which Excel needs to read it as UTF-8
  (a tool that does not want one has to strip it). JSON keeps the types: a number is a
  number, a moment is an RFC 3339 time in UTC or `null` when there is none, a `Select` is
  the list of its values (what the model stores, not what they say), and anything else is
  a string. A `Select` in CSV is the labels of its values joined by `; `, since that is
  what the cell shows and a comma in a quoted cell reads as a list of one. Its keys are the columns' labels, in the columns' order. Row actions,
  row links and the selection column are not columns and are not in the file.

## Search

- Search belongs to pages, not tables. A `Page` has a `Search`, handed the text and a
  limit, that returns `Hit`s: the arguments of that page that each result leads to, a
  title and a line beneath. It was a table's, which could be searched only with a `RowClick` to a `Link`, and a page with path arguments only while the visitor was on it. A hit is the thing and the page that shows it, so the
  question "who provides the results" has one answer, the page the result opens.

- Each hit is checked against the page it leads to, with the page's own `Guard` and the
  hit's arguments, so the search never offers what following it would refuse; the page's
  `Guard` is not asked with no arguments to decide whether to search at all, since
  one that looks the thing up refuses an empty id for the wrong reason. The
  search itself runs for every visitor, so one that is costly or private looks at the
  context. A search that fails or panics leaves that page out, and the rest answer.
  Results are grouped under the page's name in the navigation, or the entry it
  borrows from, which is what the visitor knows the page as.

- `SearchRows` is the in-memory case: it makes a `Search` from rows, a `Hit` builder and
  the fields to look in, so an app that lists its rows in a `Rows` table pays one
  declaration on the detail page and no query code, as it did with `Table.Search`.
  A table-level `Search` was removed and not kept beside it: two ways to say the same
  thing, with different limits, would have been the first thing a reader had to learn
  to choose between.

- **The search finds things, ranks them, and is only there when there is something to find.** It first also listed the app's own
  pages, filtered from the sidebar by script. That was dropped: the sidebar is already the list of pages, a search that lists
  it again puts every page above the things the visitor typed for, and an app with no `Search` had a box that could only
  filter the sidebar. Now the box is drawn only when some page has a `Search`, empty it shows nothing, and the results are those of
  the pages' Searches. They are ordered by `internal/rank`, one scale for every app, from the title (and the description, at 0.6):
  exact 1, a word that starts the title 0.95, a start 0.9, a word that ends it 0.85, an end 0.8, a word of its own 0.75, the start
  of a word 0.65, inside a word 0.4, characters in order with others between 0.2, and a floor of 0.05 for a result a source found for
  a reason neither shows. A clean match is above a sequence in the middle of a word, which is what a person typing means. The library
  scores and the source does not, so there is no `Hit.Score` to set wrong and a source that cannot rank (a database's `LIKE`)
  ranks like any other; a source with a relevance of its own cannot say so, which is the cost, and it is the one place an app
  would want a field. Results are grouped by page and the
  groups ordered by their best result, so the context that tells two things of one name apart stays, rather than one flat list.
  `SearchRows` ranks every row that matches before it keeps the limit, since the first rows that match are not the best.

- **The search asks every page at once, answers whole, and answers in HTML.** The pages' searches were asked one after the other under one
  shared deadline, so one slow search held the others back and the answer took the sum of them. Each page now runs on a goroutine of its own
  with its own deadline (two seconds), so the answer takes as long as the slowest, and a page that is late is left out and the rest answer.
  A deadline works through the context, so a `Search` that does not look at it would still block its goroutine; the answer no longer waits
  for it (the slot it writes to is buffered, and nothing it does after the request ends is logged), which is the cost of not trusting
  every `Search` to behave. Results are **not** streamed as each page answers: they would arrive in the order the pages happened to be
  fast, move under the pointer as they came, and the visitor would click the wrong one; the answer is sent once, ranked.
- **The search answers in HTML, with one renderer for the box and the page.** The same endpoint answers in HTML, not JSON: for
  script, which asks with the `X-Webui-Leaf` header (named `search`, as a request for one panel is, and not with a parameter of the address; the answer varies by it and says so with `Vary`), the box's own list (the labels and the links, with `role="option"`), which it puts in the box; and
  without it, a page of results in the shell, with the sidebar and a breadcrumb, which is what the box's button submits to when
  there is no script to list the results as it is typed in. One renderer makes both, so they cannot disagree, and the script has
  no markup of its own to keep in step with the server's. The cost is that the script inserts server HTML (escaped by the
  template) where it used to build nodes from data. The box is a form, so without script it has a button, joined to its right
  edge as one control, and with script the button is hidden and Enter follows the highlighted result; while the next results are
  asked for, the last are dimmed and out of reach, and Enter then submits the form instead.

## Navigation and links

- A tab, with script, is read alone. Following one was a navigation to the same page with another `tabs.tab`, which read
  every panel of the page again though only the tab had changed. The click now asks for the `Tabs` alone, with the leaf
  header a panel's refresh uses (`X-Webui-Leaf`, the id of the `Tabs` in the form of a panel's, which the page carries
  as `data-tabs`), and the answer, the strip and the selected tab, replaces it; the address moves with
  `pushState`, and back and forward reload the page as they do for a panel. It is the page's own request, so the Guard
  runs and an id that names nothing is a 404. What it costs, and how it is paid:
    - the other panels' links, forms and the Refresh button carry the selection in their addresses, and a panel that
    moved has the panels around it asked for again (`settle`, which asks `syncPanels`): a panel that is read and has links
    that replace it is asked for and replaced, a form keeps what was typed and takes the action of the server's copy, and the strip
    of another set of `Tabs` is replaced (its panel is not). The script does not know how an address is made: an earlier version put
    the selection (`data-tabkey`, `tabs.tab`) right in place in every link, post form and filter of the page itself, which was
    the client writing the application's addresses, and it is gone with the attribute. The cost is a request for each panel
    that has such a link or form, where there was none, for the sake of the server writing every address;
  - a tab that has an editor or a diff is a plain navigation, since the language service and the files an editor is given
    are told to the page as it is made, from the panels it was made with, and a text with code in it is too when the page
    has no editor to colour it with; a tab that does not, and any answer that is not the `Tabs`, is the same, which is
    what the link was before. Moving the editor's configuration into the panel, so that a tab could bring its own editor, was
    weighed in the hypermedia audit and dropped: the editor script already mounts an editor that arrives in a swapped panel,
    but what the panel would need to bring (its stylesheet, a module script that script has to run, the worker of each language
    service, the context files, and language services that are switched on when an editor needs one and not when the page
    loads) is a rebuilt Monaco bundle and about a hundred and fifty lines of script that has no test harness, for a request
    that was a navigation anyway, and a navigation is what a link does, not a departure from hypermedia. The page's editor
    configuration stays one JSON data island in its head, which the script reads once, and the server owns what is in it;
  - a tab that was read is read again when it is chosen again: there is no cache, which is the app's.

- A page with no `Label` lights the entry of its nearest ancestor path (the longest
  proper prefix that is a page with a `Label`, never the root). There is no field to
  declare, because the breadcrumb already derives its parents the same way.
- A page with path arguments has entries through `Page.NavEntries`, a function of `ctx`
  that returns the arguments each entry opens and what it says. It is on the page and
  not on `Nav`, because the arguments are typed by the page and `Nav` is not generic.
  `Nav` still gives the icon and the section of all of them, and must have no label,
  since each entry has its own. It is loaded when a full page is drawn, for the visitor
  it is drawn for, and not kept between pages: what a sidebar lists is as likely to
  depend on who is looking as anything, and a cache the library cannot key would be
  wrong for someone. The price is paid on the way of every page, so it is bounded: a
  deadline of two seconds, the first fifty entries, and a panic or an error leaves the
  page drawn without them and logs the cause. A fragment (a refreshed leaf), an asset
  and the search do not load it, since none of them draws the sidebar.
- An entry that lists several is left out when the page's `Guard` refuses the visitor
  with that entry's own arguments. The `Guard` is not asked with no arguments to decide
  for all of them, as it is for a page with one entry: a `Guard` that looks the thing up
  would refuse an empty name and hide every one.
- Which entry is lit is the one whose arguments fit the address best: its page is the one
  lit (the page itself or the page that borrows its place), and its path and query
  arguments are the ones in the address, with state that is not an argument (a table's
  sort, where the visitor came from) ignored. An argument the address lacks does not
  match, so a page that borrows the place lights the general entry, and an entry that
  needs a query argument only when the page carries the same one. When
  several fit, the one with the most arguments wins, so a general entry is not lit beside
  a more particular one.
- Sidebar entries whose page `Guard` refuses the visitor are left out; the `Guard` is asked with no arguments, as a visitor who opened the page bare would be. (A search result is checked with its own arguments: see Search.) The page still answers 403 to anyone who types the address. A `Guard` that panics
  when asked this way hides its entry like one that refuses, and the cause is logged
  with the page: the question is asked while every full page is drawn, the
  server-error page included, so a panic there would take down the page that
  reports the panic.
- Going back is the library's. A `Link` to a page that has a form adds the address of
  the page it is on, view state included, in the reserved parameter `webui.from`. It
  rides along through the form's own links, its re-render after a rejection and its
  POST, and a chain of pages unwinds one hop at a time, bounded so the address cannot
  grow without end. Without it Cancel goes to the breadcrumb parent and a save stays
  put. It is only ever followed when it is a path in the app, so it cannot become an
  open redirect. Expressing this as a hook the form implemented was tried and
  rejected: every link into the page had to remember to set it.
- The author can override where an outcome leaves the user, for every kind of outcome,
  with three methods that exclude each other. `Then(target)` goes to a page. `Back()`
  goes to where the user came from (else the breadcrumb parent, else stays), and
  `Stay()` stays. They exist for the cases the kind of outcome gets wrong: a `Failure`
  after which there is nothing to retry (the thing was deleted meanwhile) goes back,
  and a `Success` on a page the user goes on tuning stays. A `Failure` or `Reject` that
  goes back is not shown again, so what was typed is lost and a `Reject`'s messages
  are joined into the toast, as they already are for `Then`; the author chose to
  leave. `Stay` on a not-accepted outcome changes nothing, since keeping what was
  typed is what it does already. Cancel is navigation with no outcome to consult, so
  it always goes back.
- **The breadcrumb is derived from the path, and a page may say its own crumb.** A path has the value of an argument,
  such as an opaque id, and a person knows the thing by something else, its name. `Page.Crumb func(ctx, A) string` is
  asked for it, and not `Nav.Label`, which names an *entry* and stays refused on a page with arguments (an entry is one
  address). It names the page in its own last crumb, which is also the tab title, and as a parent in the trail of a page
  below it, where the arguments come from the path (`Decode`), so a parent whose arguments are not all in the path keeps
  the path's words, as it keeps its link. It is a function and not a field of the arguments because what it needs is
  loaded: it costs a call for each page drawn, so it has the deadline of `NavEntries` (two seconds, for the crumbs of a
  page together), and a failure, a panic or an empty answer falls back to the path's words and is logged, since a
  breadcrumb must not take a page down. It is not asked of a parent whose `Guard` refuses the visitor: what the Crumb
  knows is not theirs, and the page itself, when its Guard refuses, is drawn with the path's words. The cost is that
  the `Guard` runs once more for each page with a Crumb. A path that takes the rest, `{name...}`, has the segments for
  crumbs and may not have one (a parent above it may).

## Shell and theme

- **What script changes is said, and the focus is kept.** The audit found that a panel that was replaced (a sort, a page, a filter, a poll) dropped the focus to the page
  and said nothing, that a toast was in a live region that already held it when the page was drawn (which is not announced) and went after 3.4s including the
  ones about failures, that a refused form had the title of the page before it and no focus on what was refused, that the page's polling could not be stopped, that
  the upload's button was disabled while it held the focus and its live region was created hidden, and that the search's results were not counted aloud. Now the shell
  has an empty status region, `data-announce`, that script writes into ("Devices updated: 11–20 of 42"), the focus that was in a replaced panel goes to the new one's
  title, a panel that is replaced in the background is not from under someone typing in it, a toast is its own live region (an alert for a failure, a status for the rest) and
  is put in again once the page has loaded, a confirmation stays 8s and a warning or failure until dismissed, its dismiss control a button with script and the radio's
  label without, a refused form's title says so and the first refused control has `autofocus` (script moves it to the custom select that replaced the `<select>`, and a
  required one that was left empty says what is missing as an alert), the upload's focus goes to *Cancel* and back, its queue is out of sight and not hidden, the search says
  how many results it has in a status region, and a panel that polls has a *Pause* that stops it until *Resume*, and says when it finishes. The costs: a toast that stays
  must be dismissed, which is a step more than it was; the toolbar's polling has no pause, since the row has no room for the control (it is the one place a person cannot
  stop it); the toasts that come with a page are announced a moment after it loads, by being put in again, which moves them in the DOM; and without script nothing is
  announced, though nothing needs to be, since each action is a page.

- **A page has one h1, a way past the chrome, and landmarks for the readers who go by them.** The audit found no h1 (the page's name was only the breadcrumb and
  the title), no skip link (the sidebar, search and menus come before the content), a status bar outside every landmark, header cells with no `scope`, a column of
  "Dismiss" buttons that were the same button to a reader, a filter that was on and said nothing, a control whose help was in a popover that nothing referred to, and
  a state page title that was a paragraph. Now the shell holds an `h1` with the last crumb's label, hidden with `.sr-only` (the breadcrumb is where the page's name is
  seen, and a second, drawn, heading would repeat it; a panel is an h2, and a state page's title an h2 beneath it), a `Skip to content` link first in the body that
  leads to `main` (`tabindex="-1"` so that following it starts reading there), the status bar as a `footer` named *Page status* (the late half of a streamed bar is not
  a landmark of its own), `scope="col"` on every header cell and a word for the empty header of the selection column, a pager that is a `nav` named for its table,
  a row's buttons and selection box named by the text of its first cell, a filter that is on named *(on)*, the popover of a field's help and rules given an id that the
  input is `aria-describedby` (with the error after it), an upload result that says *Done*, *Warning* or *Failed* before its words, the confirm dialog an
  `alertdialog`, the theme button a `switch`, and a link that opens a tab saying so. The costs: the h1 is not seen, so a sighted reader does not have the page's name
  twice; the text of a row's first cell is long for a button's name when it is a long name (it is read, and not drawn); and markup that apps' own tests match changed
  (the state title, the status bar, the pager, the row buttons). Not done: a breadcrumb as an ordered list, a fixed label for the collapse button, and a focusable
  scrolling body for a table with nothing in it to tab to, which are small and wait for a reason.

- **A strip of tabs is named by its tabs, and a task box is named *Task*.** Axe found two navigations called *Views* on a page with two strips of tabs, and the boxes of a
  markdown task list with no name. A `Tabs` has no label to give (the API has no field for one, and a field for a landmark's name is a concept for the developer to
  remember), so the name is *Views* and the first three tab labels; two strips with the same first three labels still share a name, which is the cost. The
  box is named *Task* and not *Done* or *To do*, since a reader is told whether it is checked, and the item's text, which follows, is what it is about.

- **Controls are not enlarged to 24px for a pointer where they have the room WCAG 2.5.8 allows instead.** The accessibility audit counted the controls
  under 24px (a row's link, an information icon, the checkboxes, the header links, the splitter) and proposed a hit area for each. Measured with the icon font
  loaded and the script's classes on, axe-core finds only one that fails, a sortable header's link at phone width, 12px wide next to the filter button, and
  that is given a 24px minimum. The rest pass by the spacing exception (no other target within a 24px circle of them), which is conformance and not
  a workaround, and the dense look is the design (the density of the app), so they stay as they are. The cost: it holds only while they are not put
  closer together, so a new control beside one of them has to leave that room, and a touch screen has small targets that the rule allows. An earlier
  count of twenty-two came from running axe on pages whose icon font had not loaded, which shrinks every icon button; the method is to load it.

- **The app answers the visitor's display preferences, and its ordinary look keeps the quiet edge of a control.** There were no rules for `forced-colors`,
  `prefers-contrast` and only the spinners for `prefers-reduced-motion`, found by the accessibility audit (a checked checkbox was a background and vanished
  in Windows high contrast, a focus ring was a shadow with `outline: none`, the active tab and entry were fills). Now a forced-colors block draws what meant
  something with the system's colours or an outline, an outline that was none is a transparent one (drawn there and nowhere else), reduced motion stops
  animations and transitions and dims a busy panel in place of pulsing it, and `prefers-contrast: more` takes the edges and the quiet text up to full strength.
  Text that a reader needs moved from `--text-3` (3.2 to 3.55:1) to `--text-2`, and `--text-3` is for decoration only, so the hierarchy of the greys stays.
  **The edge of a control is not raised by default.** It was tried, as `--control-bd` at 3:1 for WCAG 1.4.11 (the quiet edge is 1.6:1), and it made the form
  fields hard to look at: the density of the app is its design. So the ordinary edge is the border's, `--control-bd-strong` is what a visitor who asks for more
  contrast gets, and the ordinary look does not meet 1.4.11 for the edge of an input, a checkbox or the switch, which is the cost; what tells a person where a
  field is, besides its edge, is its label, its fill (`--input-bg`, a step from the panel) and its position. A test holds the strong edge to 3:1 and the quiet text
  to 4.5:1 in both themes. Not checkable in the browser pane, and so not tried: forced colors and reduced motion themselves, which a run on Windows would be.

- No cross-document view transition. The shell's regions were named and the page opted in
  (`@view-transition { navigation: auto; }`), which cross-faded the content in 180ms. Its first frames are white, in dark mode too, because the browser shows its own default background while it makes the transition, which no rule of the page reaches: measured in Chrome 155, a white frame in about 3 of 8 navigations, and none with the transition skipped. Styles on the transition were tried, and none changed how often it came. The page that is on screen is held until the next has something to paint, as for any
  navigation, which does not flash, and a shell that is the same on every page does not appear to move. What it
  costs: the 180ms fade of the content, which the design had as the sign that the page moved. A browser that fixes the
  transition's first frames could have it back, with one rule and the names of the regions.

- The sidebar keeps its place when a link in it is followed. It is part of each page, so the next page draws it
  again at the top, and a link far down a long list would send the list back up. `prefs.js`, the script that
  runs before first paint, keeps how far the list was scrolled in `sessionStorage` when a page is left (`pagehide`)
  and puts it back as the sidebar is parsed, each time the parser adds to the document until the list is
  complete, so the first paint has it and a person who scrolls a page that is still arriving is not taken back.
  What it costs: a visitor without script has the list at the top as before (a script is what remembers it), the
  entry that is lit is not scrolled into view (the place is kept, not corrected), and one more value is kept on the
  device, for the life of the tab and no longer.

- The content keeps its place when a link goes to the same page: a tab, a sort, a filter or a page of a table is a
  whole-page navigation to the same path, and it would send a reader who had scrolled back to the top. `prefs.js`
  keeps `{path, top}` in `sessionStorage` when a page is left, and the next page puts `top` back only if its path is
  the same, as the content arrives (the same observer as the sidebar's, which stops at the first wheel, key, touch or
  click so that a page that is still being sent does not fight the reader, and at the end of the document). A link to
  another page starts at its top. What it costs: one more value on the device for the life of the tab, the path
  of the page it is for; and a page that is shorter than it was ends at its own end, as a scroll does.

- **An app has a sidebar when a page declares an entry, and not otherwise.** The sidebar is drawn when any page has a `Nav` entry (a `Label`, or `NavEntries`). An app
  whose pages declare none has no list to draw: the page has the width, and the mark of the app leads the top bar (the first crumb is the brand's name, and leads where the sidebar's
  brand did). The theme, which was a labelled switch in the sidebar's foot, is an icon button after Refresh in every top bar, the sun in the dark theme and the moon in the light one: a
  top bar has no room for a labelled switch, and one control in one place is the same whether the app has a sidebar or not (the switch was dropped from the sidebar, and its script with it). It is the declaration that decides, so the author of the app has the choice, as with every other part of the
  design, and not an edge case of an app with one page (the README's first example has one page and an entry, and a sidebar). Dropped: a rule by the number of entries that a
  visitor may open (a page that changes shape with who is looking, and with the data that `NavEntries` returns, so an app would have a sidebar on one request and none on the next);
  a rule by the number of pages declared (a detail page with no entry would count); and a setting on `App` (the answer is a function of the entries, which the app already
  declares, and a flag would let it disagree with them). What it costs: the sidebar's guarantee that the structure of the application never disappears (`design.md`) now has an exception, which
  is the author's; a visitor whom every Guard refuses sees an empty list; the collapse and resize preferences a visitor chose are kept but do nothing in an app that has no sidebar; and the theme's
  icon says what a press brings and not what is on, which a reader of the switch's state (`aria-checked`) has and a sighted one reads from the page. Not done: the icon of the theme that is on (a
  sun in the light theme reads as "the sun is out" as easily as "press for the sun"), and a visitor's choice of three (light, dark, the system's), which the stored value cannot hold today.

- The sidebar collapses to an icon rail from a button in its footer, and always below
  820px; the choice is a browser preference, so it stays out of the address. An entry
  with no icon shows its label's initial in the rail.

- **The sidebar's width is the visitor's to change, with script.** Script adds a handle on the sidebar's edge, a window splitter
  (`role="separator"`, focusable): it is dragged with the pointer, moved 16px by the arrow keys (64px with Shift), set to its
  smallest or largest by Home and End, and reset to 240px by Enter or a double click. It is 180px to 480px, and no more than half the
  window. The width is a browser preference like the collapse, in `localStorage` (`webui.sidebar.width`), so it is not in the address
  and not on the server, which has no visitor to keep it for. `prefs.js` puts it in the CSS variable `--sidebar-w` before the body is parsed,
  because every link is a whole page, and a sidebar that was drawn 240px wide and then moved would flash. The rail has its own widths, in
  rules that are more specific or later, so the variable never widens it, and there is no handle in it. Not done: a handle that is in the
  markup (it would do nothing without script, and every page's markup would change), a width kept in a cookie (it would ride
  every request), and a width in the address. What it costs: one more value in the browser (the README says so); a visitor without script
  has the width the stylesheet gives; a window that is narrower than the width it was chosen in holds it to what fits, which is not
  stored, so the wide one comes back with the window; and a width is for a browser, and not for a person on two.

- **A label that is wider than the sidebar is cut short with an ellipsis, and its whole is the title.** An entry, a section's caption
  and the brand are one line each (no wrapping, a cut with an ellipsis); the icon, and the initial that holds its place, keep their size. Dropped:
  wrapping, which makes rows of unequal height, so that a list jumps as entries come and go; and a wider sidebar, which is the visitor's
  to choose (see the width, above). The cost: the end of a long label is read only through the tooltip, which needs a pointer or focus, and two labels that
  start alike look alike.
- **A popover is moved back into view by script, and without script opens where the stylesheet puts it.** The popover of a description opens to
  the right of its icon and below it, and the reason of a control that a Guard disabled is centred above the control. When one opens, script
  measures it and moves it left, above or below, inside the window less the 26px status bar and the visible part of each box that scrolls or
  clips and holds it, whose scrollbar is not part of it: what is outside such a box is cut off however much room the window has. A reason that
  has no room above (the first row of a table, whose header is over it, or a toolbar) opens below, and the cell that holds it is raised over
  the next row's buttons. Dropped: a placement in CSS alone (anchor positioning is not in every browser, and cannot see a clipping parent), and a popover in
  the top layer, which needs script to open and so would be script-only. The cost: without script a popover near an edge, or the first row's reason in a
  table, is cut off; and the height of a reason is estimated from its text, not measured.
- Icons are Font Awesome Free solid, drawn with a vendored font served by the app, so
  the content security policy stays `'self'`. The class and name list are generated by
  `internal/render/assets/fontawesome_gen.go`.

- **A theme is a few colours, and may be a whole look.** The renderer derives the rest per mode with `color-mix`,
  because one colour cannot be right on both a dark and a light surface. The surfaces and the text were not themeable for that reason (the mode is the
  viewer's choice), which made the library's look the one look: six dashboards in six colours were one dashboard with a stripe. The rule is now narrower, and
  says what it was for: **a colour for the surfaces is one colour, so it is given for the mode it is for**. `Theme` keeps its four colours, which are those of both modes, and gains `Light` and `Dark`, each a
  `ModeColors`: a `Canvas`, the background in that mode, from which the panels, the borders and the text are derived by the same relations the design's own palettes have (`internal/theme`), and an `Accent`, `OK`, `Warning` or `Critical` that wins
  over the shared one in that mode. The viewer still chooses between the modes, with the button and the system's choice as before: an app gives its look in each, and does not take the choice. What it costs: a derived palette is a fixed recipe, so a canvas that the recipe does not suit is refused and is not tuned; the design's own palette is in the stylesheet and in Go (`theme.Base`), which a test keeps equal; a
  mode's own colour is one more place a colour can come from, so a problem names the field that gave it (`Accent`, `Light.Accent`); and **`Compile` now refuses a colour that cannot be read**. An accent
  that cannot carry a white label (`#2f9e8f`, 3.3:1, which the README had as its example) was accepted before and drew a button whose label was out of sight; it is an error that names the
  field, the pair and what to change. Not done: **presets** (built first, as six `Theme` values with a look in each mode, and dropped: each is a look that the library would have to keep readable, and the more it ships the more there is to maintain for accessibility; an app that wants one declares it, and `Compile` holds it to the same numbers), a theme picker for the viewer (the storage the README lists would grow by a value, and the buttons of an app are the app's), **pinning an app to one mode** (it was built first and dropped: it took the choice from the
  viewer, and an app that wants one mode can give a `Canvas` for it and leave the other to the design), a recipe that tunes the colours it is given (the author would not know what was drawn), and any surface token as a field of its own (there are twelve, and they are not independent).

## Assets and transport

- The framework's routes (`/_webui/...`) are a `ServeMux` of their own, and `Handler`
  sends to it what is under that directory and everything else to the pages' mux. On
  one mux, a page such as `/{key}/info` and the asset route `/_webui/{file}` both match
  `/_webui/info` with neither more specific, which `ServeMux` refuses by panicking when
  the app is built, so no page could begin with a placeholder. The directory is
  reserved (no page path may start with it), so a placeholder never captures it: a
  request under `/_webui` is the framework's or a 404, never a page. What it costs: one
  more mux and a prefix check per request. Two pages whose paths cross
  (`/{a}/x` and `/y/{b}`) still conflict, as `ServeMux` defines it, and Compile now
  reports that as an error naming both, instead of the router's panic.
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
- A page with a panel that is slow is sent as it is built. The panels of a layout load at the same time, and each
  is a component that waits for its own load when the layout draws it, so a layout draws its markup, then each panel
  in order. A page waits 50 ms (`streamAfter`) for all of them: if they are done, it is drawn into a buffer and sent
  in one piece, exactly as it has always been (the same bytes, a real status if something fails, one write, no
  flush). If one is not, the status and the headers are sent with the sidebar and the panels that are ready, flushed
  through the compression and the writers that wrap the response, and then each panel as it is done, in the order
  the page has them. The sidebar's own inputs (which entries the visitor may not open, where the brand leads, the
  entries that pages list) are asked for at the same time as the panels, so the first byte waits for the slowest of
  them and not for their sum. In order means no placeholders, no template and no script: it works with script off
  and on a first visit, and a layout inside a layout streams with no change to either.
- **What a page that is sent as it is built costs.** A slow panel holds back the ready ones after it, which is no slower than a page that is not sent
  early; the page shifts as panels arrive; the script (`enhance.js`, deferred) runs when the document ends, so the search waits
  for the last panel. A reverse proxy that buffers a response only costs the gain (`X-Accel-Buffering: no` asks nginx not to).
  What the status is, is decided before the first byte, so a Guard that refuses is still a 403; after it a panel that panics or
  cannot be built is drawn as a panel that could not load, and logged, where it would have been a 500. A page whose head depends
  on its panels (an editor, a diff or Markdown links its script and stylesheet) and a HEAD request are built whole. The
  `Server-Timing` header says how long the first of the page took; the status bar, drawn last, says how long
  all of it did.

## The status bar

- The bar says how the page came to be: the server's render time, the delivery time after it (the network, the assets and the browser) and, without script only, when the HTML was built, in the HTML of a page, and in the elements that follow a panel's answer (see below), with the standard `Server-Timing` header on every response so the browser's own tools read the same number. There is no header of the framework's for it: a panel that is fetched again ends with a hidden element holding the two elements of the bar that it replaces, each saying which (`data-replaces="[data-build]"`, a selector), and the script that put the panel in its place puts them in theirs, which is all it knows of it. The cost is a few hundred bytes with every panel. The render time is measured from the request's arrival at the app to the moment the document is rendered, so it includes the application's loaders and not the writing of the response.
- **JS off** is a `<noscript>` badge, so it is there exactly when script is off and costs
  nothing otherwise. It says what degrades, in its title.
- **stale page** is a guess, and says so. The badge is hidden unless the page looks stale, and its title says why. A page is built for each request and sent
  `no-store`, so one whose build time is more than 15 seconds before it was opened was
  probably not built for that request: a cache, a proxy or a DNS answer put a copy between
  the server and the viewer. The browser cache is detected from Navigation Timing (a
  transfer of zero bytes that is not a reload), and a page restored from the back-forward
  cache says so. What it costs: the comparison is between the server's clock and the viewer's,
  so a computer whose clock is a minute off shows the badge on every page, and one that is behind is never told; the title of the badge says this. A way to know without trusting
  either clock would need a round trip to ask the server the time, which the bar is not worth.
  The build time is in the page, and not taken from the `Date` header, which the script
  cannot read for the document and which is whole seconds.

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
- `Editor.Context` is the one place the service is configured: an `fs.FS` with a
  directory for each language (`typescript/`, `javascript/`, `json/`), read at compile
  time, so a mistake in the folder is a compile error and an option the editor would
  ignore is refused, not accepted and dropped. Decided against a field for each thing
  (`Schema`, `Compiler`, ...): it is an escape hatch for power users and adds nothing to
  what the others see. `tsconfig.json` is the format for the options because people
  already have one; only `compilerOptions` is read, and the enum options are sent as
  TypeScript's numbers, since Monaco's own enums stop at ES2020 and Node module
  resolution. The folder has no size limit: it is the author's own embedded code.
- The context files are **locked assets**. They are served from
  `/_webui/code/<hash>.<ext>?k=<key>` and a page is handed the addresses, with their keys,
  only when the visitor may edit the editor (the same Guard as the Save button). The hash
  names the file, so the browser keeps it as `private, immutable`; the key is
  `HMAC-SHA256(secret, name)`, so one file's key opens no other and nothing is stored for
  keys. A wrong or missing key is a 404, so existence is not confirmed.
- **What locking the context files costs, both ways.** Compared with inline in the page, it gives caching and a response that
  does not carry a library's types on every load, and it gives up: the key is one bearer value for every visitor and for the
  life of the process, so anyone given an address (a pasted link, an access log, since the key is in the query) can fetch that
  file until the process restarts, and a visitor who loses edit rights keeps it until then. Compared with the files being
  public, it keeps types that a person would not publish from being read by anyone who guesses a hash. The secret is random per
  process (`App.Secret` overrides it): with several instances and no shared secret, a page drawn by one hands an address that
  another rejects with a 404, and the editor then runs without its context. The library's own assets, and the logo and favicons,
  stay public: they are needed before any editor page, so a key would be handed to every visitor and protect nothing. The
  handler is written so that another user-supplied asset can be locked the same way.

- An `Editor` is lowered to the same node as a `Form` with one field, so loading,
  rules, the Guard, rejection and echo are the one pipeline and not a second one.
  Its body limit is raised (16 MiB) on pages that have an editor only, since a
  text is the one field that is expected to be large.
- Without script the editor is a `textarea`, the viewer a `pre`, and a `Diff` a
  unified diff that the server makes. The script mounts Monaco over them and keeps
  the `textarea` as the editor has it, so the form posts as it always did.
- The editor is used from the keyboard as well as it can be, and what it cannot be is said. Esc then Tab
  leaves it (Monaco's own Ctrl+M does too, for every editor on the page, and says nothing), and the line
  under it says so, for everyone and not only for those who would look for it. The costs: Esc does not leave
  the editor while a selection, a list of suggestions or the find box could use it, so a reader may press it
  twice; the hint takes a line of height from every editable editor; Cmd+K and Ctrl+K are not the search
  inside an editor, where they are Monaco's (a reader leaves the editor first); and Cmd+S presses the
  form's button, so with no button offered (a Guard) it does nothing, where it used to send the form past it. The
  required message is made in the page, with the textarea hidden, as the required select's is, and the
  server asks as well.

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
- An image is a link to the picture, named by its alt text, unless its origin is one
  the author trusts. The content security policy refuses a picture from another site,
  so drawing one would only show a broken icon, and a picture is how a text makes the
  visitor's browser fetch from a server of the text's choosing. `TrustedOrigins` is
  the author's way to say that a server is theirs, or one they answer for.
- A trusted origin is exact: `https://host` with a port only when it is not 443, in
  ASCII, and nothing else. No wildcard (a `*.example.com` trusts every site anyone can
  create there), no path, no user, no `http`. The list goes into a header, so an entry
  with a space, a `;`, a comma or a quote is a compile error, and the renderer writes
  the checked address out again rather than the text it was given. A picture is
  compared as a parsed origin, never as a string prefix: `cdn.example.com.evil.example`,
  `cdn.example.com@evil.example`, a backslash, a protocol-relative address and a
  subdomain are all other origins and stay links.
- The policy is set per page, at compile time, from the origins of the Markdown leaves and of the lists of items (`Feed`, `Cards`)
  the page declares, tabs that are not selected included. It is not computed while a
  request is served: the header is written before the body is built, and a leaf
  refresh is a fetch inside a document that already has its policy. A page whose
  Markdown names no origin has the library's policy byte for byte, and so has every
  other page. Links get no such treatment: they were never behind a warning page, so
  a trusted origin changes nothing about them, and the library does not have an
  interstitial for untrusted links.
- Code blocks are plain text in a block. When one names a language, the page links the
  editor that is already embedded and the glue colours the block by its tokenizers; a
  page whose markdown has no such block never links the editor. Colouring is
  enhancement: without script, or for a language the editor does not know, the block
  stays plain. No language service is ever offered, since the text cannot be edited.
- goldmark is a runtime dependency of the library, beside templ and the Prometheus client. Writing a markdown
  parser would have meant owning its edge cases, and its escaping, in a place where a
  mistake is a cross-site script. It is pure Go, and was already in the module graph
  through the linter, so it adds nothing transitive.

## Figures: Stat, Delta and Series

- **A figure with its change and its shape is a kind of value, and a leaf for a few of them.** The design had no statistic or chart leaf on purpose (`design-mapping.md`): a leaf
  showed a model, or a list of models, and each leaf was a kind of text a model has. A figure is that too, once it is an accessor, so the pieces are accessors first: `Delta` is a
  signed number drawn with an arrow and a colour for whether the direction is good, and `Series` is a list of numbers drawn as a line or as bars. They work wherever an accessor does:
  a column (a row of markets, with its change and its chart), a read-only field of a form or a toolbar, and a tile. `Stat` is the leaf that lays a few of them out as tiles, which
  is the one thing no existing leaf does: it has no rows and no fields to type into.
- **The change says its direction in text, and its colour says whether it is good.** A colour alone says nothing to a reader who does not see it, and an arrow that is red does not say whether
  a rise is bad or the thing it is of is. The text has the sign (and the minus sign, since a hyphen reads as a dash), the arrow repeats it, and the colour is the field's `Better`: a
  response time that falls is green, a rate of errors that falls is green, and a throughput that falls is red. A reader who does not see the colour is told "(better)" or "(worse)". A change that reads zero
  as it is shown (a rise of 0.001 with two decimals) has no sign, arrow or colour, so that the text and the colour are never two stories.
- **A series is drawn by the server, as an SVG, with its values in words as its name.** HTML is the state: the chart is markup the server made, not data that a script turns into
  markup, so it is there without script, in a table that is exported, and in a screen reader's list. Its name is *what it is, how many values, from where to where, the lowest and the
  highest*, and a file of it has the points, separated by semicolons. It has no axes and no scale: it is the shape beside the figure, and a figure that needs a scale is not this.
  It is neither sorted nor filtered by. A value that is not a number is left out, and a `Highlight` is an index of the points as they were loaded, so that dropping one does not move
  the mark.
- **Dropped:** a `Chart` leaf with axes, a legend, a time range and several lines (the dashboard the design says it is not, and a place where the app's own panel or a link to the tool that
  does it is the answer); a tile that draws a value of its own (a value is an accessor, so a badge, a bar and a formatted size are each themselves, and a tile keeps up with any new one for
  free); sparklines made by script from data in an attribute (a second source of markup); and `Stat` taking a list of models for one tile each (the markets pattern is a `Table` with a
  `Delta` and a `Series` column, which sorts, filters and pages).
- **What it costs:** a new public leaf and two accessors, and the files that go with a leaf (the declaration, the node, the loader, the view, four registries in the runtime); an SVG in the
  page, which the content security policy allows as it is inline markup and not a fetch; two more display kinds for a table's width, export and search to know about; a series that cannot
  be told apart in its detail by anyone who does not read its name (the point); and the "no chart or statistic leaf" statement of the design is now a statement about axes and legends.

## Feeds and cards

- **A list of items is not a table with the header taken off.** A `Table` is a list of rows with columns, which sort and filter and page, and have actions on a row and in a selection: every one of them is
  about finding and changing a row. A feed is about reading what is new, and has the opposite needs: a headline and a few facts, nothing to find by, nothing to change, and a place where the picture is. Making
  it a presentation of `Table` was weighed (it was the first of the two designs offered for this change, and not the one chosen) and the leaf was made instead, so that it has no sort links, filters, bulk
  actions or export to switch off, and no columns to read as a header.
- **`Feed` and `Cards` are one leaf with two drawings.** They have the same source, the same fields, the same Show more and the same view state, so they are one node in the IR (`ir.Items`) with one component,
  and two public types that differ only in that `Cards` has a `Layout`. Duplicated fields are the cost of two types, and the types are kept apart because the name says how it is drawn.
- **The source is a plain list, asked for one more than is shown.** `Load(ctx, limit)`. The library asks for `limit+1` to know that there is more without a count, and trims it. The other sources a table has
  were not offered: a cursor needs a cursor in the address and a way back, and a feed that needs one is a table (`Table.Feed`); a source that sorts and filters is a table. *Show more* adds to what is
  shown, so it is a limit in the address, and nothing is kept on the server. It is capped at 200, so that a hand-edited address cannot ask a source for a million, and the cap is the leaf's and
  not the app's: an app that wants more has a table.
- **Show more is a link.** It is the same page with a larger limit, so it works without script, a copied link shows the same view, and with script it replaces the leaf in place as the sort and pager links of a table do.
  What it costs: without script it is a navigation to the same page, which scrolls to the top, and each press asks the source for more than the last, so a long feed asks again for what it showed.
- **A headline is text, and where it leads is another field.** `Open` is a `Link` to a page of the app, built from the item, and is left out for an item whose page refuses the visitor; `Link` is an address,
  which opens in a tab of its own with the mark that says so. A `URL` accessor was not used for the headline because its text is the same on every row: it is a column of links and not a headline.
  An `Open` that is an `Action` was refused: an item is read, a link opens in a new tab, and an action is a POST.
- **A picture is drawn only if the page may fetch it, by the rule that decides it for text.** The address of a picture is data (an RSS item's, a user's avatar), so it is held to what a `Markdown` picture is (`markdown.Picture`): a
  path on this site, or an https address from an origin the author trusted, with the browser's reading of the address checked against the parser's. It is not drawn otherwise, and the item has no picture, as one that has none has
  not: never a broken one. The origins are in the page's content security policy, and in no other page's. A picture is decoration (`alt=""`), in a box of its own size so that it does not move the page, loaded late and without a referrer.
  What it costs: a remote thumbnail needs an origin named, which is a decision the author makes and not a default, and a feed of pictures from many hosts is a proxy the app runs.
- **Dropped:** row actions and selection (an item is read); sorting and filtering (a feed's order is its source's); a cursor; infinite scroll (script that fetches as the reader scrolls is a second way to page, and not a link); a
  carousel with arrows and dots (a row that scrolls, with the keyboard and the browser's own scroll, is what one is without script); and a picture of the item's own kind (a video player, an embed).

## Metrics

- The app collects and does not serve. `App.Metrics` is a `prometheus.Registerer`, the
  program's own, and the program serves its registry where it serves its other metrics.
  A `/_webui/metrics` was proposed and dropped: what is exposed, at which address and to
  whom are the host's decisions, and the metrics name every page template and how much
  each is used, which should not be public by default. What it costs: one line more for
  the program.
- **Compiling again, or sharing a registry, never fails.** Compiling the app again, or compiling a second app with the same
  registry, works: a metric the registry already has, the same in name, labels and help, is the one that is counted into (the
  `AlreadyRegisteredError` carries it), so a program that replaces its app as it changes keeps its counts, and nothing fails.
  Failing was the first design, as the safe choice, and was dropped: it made the one common case of a second compile (a program
  that recompiles) an error to work around, in exchange for telling the program about a mistake that is not one. The cost is
  that two *different* apps in one registry add their counts together where a page has the same path template, which is right
  for a replacement and wrong for two apps: the program wraps the registry with `prometheus.WrapRegistererWith` and a label of
  its own for each, to tell them apart. A metric that has the name of one of these and is not the same is still an error, since
  it would be a clash and not a second app.

- Only webui's own metrics, `webui_...`, on a registry the program supplies, so the Go
  runtime and process collectors are the program's to add or not. `prometheus/client_golang`
  is a dependency of the library for this: it was weighed against writing the text format
  by hand, and the standard client won on correctness (histograms, the exposition formats)
  at the price of its own transitive dependencies in every program that imports webui.
- Labels are the page's path template and a few fixed names, never an address, a user, a
  table's row or an error text, so the series are bounded by what the app declared. A
  request no page answered is `unmatched`, and the framework's own routes are `/_webui`.
  Loads are timed where the application's function is called, so a slow source shows up
  as itself and not as a slow page; a load that failed because the visitor left is not
  an error.
- A nil recorder does nothing, so the code that counts does not ask whether metrics are on,
  and an app with no registry pays one nil check.

## Logging

- The app logs to the `*slog.Logger` it is given (`App.Logger`), and to nothing when it is given none.
  It does not fall back to `slog.Default()`: a library that writes to a global that the program never
  handed it is output the program did not ask for, and it cannot be turned off for one app and left on for
  another. What it costs: a program that wants the lines passes `slog.Default()`, one line, and one that
  forgot hears nothing from a panel that could not load, which the page still says in words.
- The logger is reached two ways. The running app holds it, and the request's context carries it for the code that is called with no more than a value, which is a column's `Format`:
  its panic is recovered where the value is turned into text, and it is logged there. A context with no
  logger gives one that says nothing, so the code that logs does not ask whether it is on.
- The compile-failure page has no logger, since there is no app to give it one.

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
- **`Compile` can be called again, and swapping the handler is the program's.** A program that changes its pages compiles a new `App` and
  replaces the handler that it serves; the metrics that are already registered are counted into (see Metrics). A type of the library's own that
  watches for changes and swaps was dropped: it would own a lifecycle (when to compile, what becomes of the old handler and of the requests that
  are on it), which the library owns nowhere else. The cost: the program writes the swap (an atomic pointer, or a mutex around its handler), and
  nothing tells it which requests ran on the old one.
