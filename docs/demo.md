# The demo, and what it must use

The demo (`cmd/demo`) is **Collector**, a mock device-monitoring app: devices, sites, alerts, rules, firmware, an audit log, health checks, settings. It is the one app that has every kind of page, so it is where every feature of the library is used, looked at and tested. Two rules keep it that:

1. **Every feature of the library is used at least once in it, on a page that belongs to the Collector.** A page never exists only to show a feature: a new feature goes onto the page of the monitor where a monitor would need it (a filter on the table of devices, a rule on the form of settings), in the words of that page. There are no "layout", "loading" or "showcase" pages, and a case that has no place in the monitor goes in a test in `test/`, not in a page.
2. **The landing page (`/`) is the plain one.** It is the first thing a visitor sees and what the README shows, so it holds ordinary panels in an ordinary layout, with none of the edge cases that a feature needs to be seen in. Put those on another page.

Nothing checks this by machine (it was left out on purpose: a test that fails when a name is not used makes a feature that has no sensible use in a monitor a reason to invent one). So the table below is kept by whoever changes the library or the demo, **in the same change**.

## When you change the library

- A new or changed public name, field or option of `pkg/webui`: use it in a page of the demo, and add or change its row below. If it has no sensible use in a device monitor, put it under "Not used, and why", with the reason.
- A removed or renamed one: change the demo and its row.
- A change to what a demo page shows: keep the page about the Collector, keep its path (the browser suite and `TestEveryPageServes` read them), and keep what its tests prove.

To see what the demo does not use yet, list the exported names of `pkg/webui` and look for each in `cmd/demo` (`grep -rn 'webui\.Name' cmd/demo --include=*.go`, and the field names for options). A field is used when it is set in a literal; one that is only read (the fields of `FileHandle`, `PagedQuery`, `Span`) is used where it is read.

## Where each feature is used

| Feature | Where |
|---|---|
| `App`: `Brand` (`Name`, `Logo`, `NoFavicon`), `Description`, `Logger`, `Observer`, `Secret`, `NoConsoleMessage`, `Menu`, `Navigation` | `main.go` (`NoFavicon`, `NoConsoleMessage` and `Secret` by flags), `metrics.go` |
| `Theme`: `Accent`, `OK`, `Warning`, `Critical`, `Light` and `Dark` (`ModeColors`: `Canvas`, `Accent`, `OK`, `Warning`, `Critical`) | `main.go` (`-accent`, `-palette forest`) |
| Navigation: `NavPage`, `NavPages` (`Load`, `Label`), `NavURL`, `NavGroup` | `main.go`, `sites.go` |
| `Page`, `PageID`, `Guard`, `Crumb`, `Search` (`Hit`), `Open`, `ArgsOf`, path and query arguments, `{path...}` | `devices.go`, `sites.go`, `alerts.go`, `files.go`, `compare.go` |
| `Table` and its options: `Columns`, `PageSize`, `Export`, `Key`, `RowClick`, `Actions`, `BulkActions`, `HeadActions`; `NoFilter`, `NoSort` on an accessor | `devices.go`, `alerts.go`, `sites.go`, `system.go` |
| Loaders: `RowsLoader`, `PagedLoader` (`PagedQuery`, `PagedWindow`), `FeedLoader` | `alerts.go`, `devices.go`, `system.go` |
| `Form`, `Editor`, `Diff`, `Markdown`, `FileUpload`, `Toolbar` | `settings.go`, `config.go`, `rule.go`, `handbook.go`, `uploads.go`, `overview.go`, `devices.go` |
| `List`, `Cards` (`CardsScroll`, `CardsGrid`), `Calendar` | `activity.go`, `sites.go` |
| Layout: `Stack`, `Split`, `Columns`, `Tabs`, `When` | `overview.go`, `activity.go`, `devices.go`, `health.go` |
| Accessors: `String`, `Int`, `Float`, `Slider`, `Bool`, `Select`, `Group`, `Secret`, `Email`, `Phone`, `Website`, `ColorPicker`, `Time`, `Trend`, `File` | `settings.go`, `uploads.go`, `devices.go`, `overview.go` |
| Optional accessors: `OptionalInt`, `OptionalFloat`, `OptionalSlider`, `OptionalBool`, `OptionalTime` | `settings.go` |
| Rules: `StringRules` (`MinLen`, `MaxLen`, `StartsWith`, `EndsWith`, `Contains`, `NotStartsWith`, `NotEndsWith`, `NotContains`, `Pattern`), `NumberRules`, `PatternRule`, `ValidatePattern` | `settings.go`, `devices.go` |
| `Action` (`Role`: `RolePrimary`, `RoleSecondary`, `RoleDestructive`; `Confirm`; `Guard`), `Disabled`, `Link`, `Href`, `Control` | `alerts.go`, `devices.go`, `overview.go` |
| Outcomes: `Success`, `Warning`, `Failure`, `Reject`, `Field`, `Then` | `alerts.go`, `devices.go` |
| Refusals: `NotFound`, `Unauthorized`, `Forbidden`, `Refusal.WithLink` | `devices.go`, `auth.go` |
| `Loaded`: `Ready`, `RefreshAfter`, `Failed` | `overview.go`, `health.go` |
| `Tone`: `ToneOK`, `ToneWarning`, `ToneCritical`, `ToneNeutral` | `devices.go`, `alerts.go`, `sites.go` |
| `FileKind`: `FileImage`, `FilePDF`; `FileInfo` | `uploads.go` |
| `lang`: the editor languages | `config.go`, `rule.go` |
| Compile errors: `CompileError`, `PatternError` | the pages that `-broken` adds, `main.go` |

## Not used, and why

- `Calendar.SundayFirst`: it cannot be set with `WeekNumbers`, which the Activity calendar uses (week numbers are ISO weeks, which begin on Monday).
- `FilePNG`, `FileJPEG`, `FileGIF`, `FileWebP`, `FileCSV`, `FileJSON`, `FileText`, `FileAny`: other members of the same `Accept` list as `FileImage` and `FilePDF`; a monitor takes an image and a certificate, and the library's own tests (`test/fileupload_test.go`) take each kind.
- Names that appear only where Go infers them (`View`, `Loader`, `Control`, `Accessor`, `PageRef`, `NavTarget`, `NavItem`, `Target`, `Role`, `Tone`, `Language` as types): they are used by every page that uses a value of them.
- `FieldError`: it is what `Field` returns, and is used in that sense.
