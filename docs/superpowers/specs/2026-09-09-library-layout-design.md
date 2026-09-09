# Library layout

Date: 2026-09-09

This repo is a Go library, not a service template. Layout, tests, and agent docs
must match that.

## Goal

Sharpen package boundaries so:

- `pkg/` is the public API (glue).
- `internal/` holds implementation `pkg/` consumes, hidden from other modules.
- `test/` holds integration tests against `pkg/` only.
- Unit tests, when they exist, sit next to the code they cover.

Echo stays as the example capability until real library types land.

## Decisions

| Topic | Choice |
| ----- | ------ |
| Scope | Rehome tests, rewrite conventions, reshape `pkg`/`internal` |
| Consume-surface | Concrete types and funcs. Go `interface` only if `pkg` must mock, defined at use site |
| Internal layout | `internal/<capability>` — drop `internal/domain/` |
| Echo API | One function each side. No `Service`, no DTO, no JSON tags |
| Tests | Prefer integration. Unit tests rare |

## Layout

```
pkg/webui/          public API. Glue only. Compose internal packages.
internal/echo/      one capability. Concrete export. Not importable outside module.
test/               integration vs pkg/webui only. No test/unit.
```

Rules:

- `pkg/` does not hold behaviour beyond wiring and public types.
- `internal/<capability>` does not import `pkg/`.
- `internal/` packages may import each other. `pkg/` is the composition root.
- Delete `internal/domain/`. Delete `test/unit/`.

## Consume-surface (echo)

`internal/echo`:

```go
package echo

func Echo(message string) string {
	return strings.TrimSpace(message)
}
```

`pkg/webui`:

```go
func Echo(message string) string {
	return echo.Echo(message)
}
```

Public types live in `pkg/` when consumers need them. Internal types stay in
`internal/` (the module already hides them).

Package comments on `pkg/webui` and `internal/echo` describe a library, not
template “modes” or IO adapters.

No Go `interface` for echo. Add one later only if `pkg` must mock, and define it
at the use site (`pkg` or the test). Implementers may add
`var _ I = (*T)(nil)` per `.cursor/rules/go-interface-assertions.mdc`.

## Tests

`test/` is integration only. Black-box `pkg/webui`. External test package.

```
test/echo_test.go    package webui_test
                     import github.com/Olian04/webui/pkg/webui
```

Keep the current table cases: trim surrounding space, keep inner space, empty
stays empty. Move them out of `test/unit/pkg/webui/`. Delete that tree.

Unit tests are `*_test.go` next to source, same package. Echo is too thin for a
colocated unit test — skip it.

`test/` must not import `internal/`.

## Docs and tooling

Rewrite `docs/AGENTS.md`:

- Layout table as above.
- Dependency direction: `pkg/webui` → `internal/<capability>`; never reverse.
- Test law as above.
- Drop `cmd/`, template modes, HTTP/observability, `test/unit` mirrors.
- Keep the Go proverbs and Uber distill already in that file.

Align agents to the same words:

- `.cursor/rules/agents-md-alignment.mdc`
- `.cursor/agents/go-quality-reviewer.md` — unit = colocated `*_test.go`;
  `test/` = integration vs `pkg/` only

Makefile:

```
SOURCE_CODE ?= ./internal/... ./pkg/... ./test/...
```

`make test` already runs with `-race`. CI calls `make test-race`, which has no
target. Drop that CI step and the unused `test-race` PHONY in the Makefile.

Leave `README.md` as the existing one-liner.

## Out of scope

- Real admin-panel types (pages, fields, actions). Echo remains the stub.
- Nested `template/` or `poc/` trees (not part of this pass).
- New README content, extra design docs beyond this spec.
- Introducing Go interfaces “for the future”.
