# Library Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reshape this module so `pkg/webui` is glue, `internal/echo` holds echo behaviour, and `test/` is integration-only against `pkg/webui`.

**Architecture:** `pkg/webui` is the composition root and the only import path for other modules. Each `internal/<capability>` exports concrete funcs/types; `internal/` never imports `pkg/`. Public `Echo(string) string` stays; internals drop `Service` / DTO / JSON tags. Characterization tests in `test/` stay green while the move happens — public behaviour does not change.

**Tech Stack:** Go 1.27, `make test` / `make lint`, GitHub Actions CI.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-09-09-library-layout-design.md`
- Module: `github.com/Olian04/webui`
- Public API stays `func Echo(message string) string` in package `webui`
- No Go `interface` for echo
- No colocated `*_test.go` for echo (too thin)
- `test/` must not import `internal/`
- Do not change `README.md`
- Do not add pages/fields/actions; echo stays the stub
- Index may already have unrelated staged edits to `Makefile`, `README.md`, `docs/AGENTS.md`. Implement from this plan + spec, not those staged diffs. Leave `README.md` as HEAD’s one-liner. Commit only files listed in each task.
- Commit messages: Conventional Commits, capital after colon (repo style: `chore: Apply template`)
- Sequential: Task 2 needs Task 1’s `test/` + Makefile paths. Task 3 needs the new layout to exist. Do not parallelize.

## File map

| Path                                            | Role                                                       |
| ----------------------------------------------- | ---------------------------------------------------------- |
| `test/echo_test.go`                             | Integration tests, `package webui_test`                    |
| `internal/echo/echo.go`                         | Concrete `Echo(string) string`                             |
| `pkg/webui/echo.go`                             | Public glue calling `internal/echo`                        |
| `Makefile`                                      | `SOURCE_CODE` covers `./internal/... ./pkg/... ./test/...` |
| `docs/AGENTS.md`                                | Library layout law                                         |
| `.cursor/rules/agents-md-alignment.mdc`         | Same words as AGENTS.md                                    |
| `.cursor/agents/go-quality-reviewer.md`         | Test placement + library structure                         |
| `.cursor/rules/go-quality-reviewer-routing.mdc` | Review `pkg/` not `cmd/`                                   |
| `.github/workflows/ci.yml`                      | Drop broken `make test-race` step                          |
| `test/unit/pkg/webui/echo_test.go`              | Delete                                                     |
| `internal/domain/echo/service.go`               | Delete                                                     |

---

### Task 1: Rehome integration tests

**Files:**

- Create: `test/echo_test.go`
- Modify: `Makefile`
- Delete: `test/unit/pkg/webui/echo_test.go` (and empty `test/unit/` tree)

**Interfaces:**

- Consumes: existing `webui.Echo(message string) string`
- Produces: `test/echo_test.go` with `TestEcho`; Makefile `SOURCE_CODE ?= ./internal/... ./pkg/... ./test/...`

The public API already exists. This test is a characterization test — it must **pass** on the first run, then keep passing.

- [ ] **Step 1: Write `test/echo_test.go`**

```go
package webui_test

import (
	"testing"

	"github.com/Olian04/webui/pkg/webui"
)

func TestEcho(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims surrounding space", in: " hello ", want: "hello"},
		{name: "leaves inner space", in: "hello world", want: "hello world"},
		{name: "empty stays empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webui.Echo(tt.in); got != tt.want {
				t.Fatalf("Echo(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

Do not import `internal/` from this file.

- [ ] **Step 2: Run the new test**

Run: `go test -race -count=1 ./test/`

Expected: PASS (`ok` for `github.com/Olian04/webui/test`, three subtests)

- [ ] **Step 3: Point Makefile at `./test/...`**

Replace `Makefile` with:

```makefile
.PHONY: format lint test help

# Trees this library owns. A missing path fails the whole target.
SOURCE_CODE ?= ./internal/... ./pkg/... ./test/...
REV := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_OUTPUT_DIR := ./dist

help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

format: ## Run go fmt and gofmt
	go fmt ./...
	gofmt -w .

lint: ## Run go vet, module verify, vuln scan, golangci
	go vet ./...
	go mod verify
	go tool govulncheck $(SOURCE_CODE)
	go tool golangci-lint run $(SOURCE_CODE)

test: ## Run tests
	go test -race -shuffle=on -timeout 180s $(SOURCE_CODE)
```

Dropped: `test-race` PHONY (no target). Dropped: `./test/unit/...`. Comment no longer talks about template modes.

- [ ] **Step 4: Delete the old mirrored tests**

```bash
rm -f test/unit/pkg/webui/echo_test.go
rmdir test/unit/pkg/webui test/unit/pkg test/unit 2>/dev/null || true
```

Confirm `test/unit` is gone and `test/echo_test.go` remains.

- [ ] **Step 5: Run `make test`**

Run: `make test`

Expected: PASS. `go test` covers `./internal/... ./pkg/... ./test/...`. No package under `test/unit`.

- [ ] **Step 6: Commit**

```bash
git add test/echo_test.go Makefile
git rm -f test/unit/pkg/webui/echo_test.go
git commit -m "$(cat <<'EOF'
test: Move echo coverage to test/

Integration tests target pkg/webui only. Makefile must not
point at the old test/unit tree.
EOF
)"
```

---

### Task 2: `internal/echo` + `pkg/webui` glue

**Files:**

- Create: `internal/echo/echo.go`
- Modify: `pkg/webui/echo.go`
- Delete: `internal/domain/echo/service.go` (and empty `internal/domain/` tree)
- Test: `test/echo_test.go` (already in tree from Task 1)

**Interfaces:**

- Consumes: `test/echo_test.go` calling `webui.Echo(message string) string`
- Produces: `echo.Echo(message string) string` in package `echo`; `webui.Echo` delegates to it

No `Service`, `Request`, `Response`, or JSON tags. No Go `interface`. No `internal/echo/echo_test.go`.

- [ ] **Step 1: Add `internal/echo/echo.go`**

```go
// Package echo holds echo capability behaviour.
//
// pkg/webui consumes this package as glue. Do not import pkg/.
package echo

import "strings"

// Echo returns message with surrounding whitespace removed.
func Echo(message string) string {
	return strings.TrimSpace(message)
}
```

- [ ] **Step 2: Point `pkg/webui` at `internal/echo`**

Replace `pkg/webui/echo.go` with:

```go
// Package webui is the public library surface for this module.
//
// This package is glue: it composes internal capabilities and exports the
// API other modules import. Consumers cannot import internal/. Keep
// behaviour in internal/<capability>; keep public types here.
package webui

import "github.com/Olian04/webui/internal/echo"

// Echo returns message with surrounding whitespace removed.
func Echo(message string) string {
	return echo.Echo(message)
}
```

- [ ] **Step 3: Run tests (both impls still in tree is OK)**

Run: `make test`

Expected: PASS. `pkg/webui` now calls `internal/echo`. `internal/domain/echo` may still compile until the next step.

- [ ] **Step 4: Delete `internal/domain`**

```bash
rm -f internal/domain/echo/service.go
rmdir internal/domain/echo internal/domain
```

Grep the module (exclude `docs/superpowers/`) for `internal/domain` — only the spec/plan should mention it.

- [ ] **Step 5: Run tests and lint**

Run: `make test && make lint`

Expected: both PASS. `go vet` / golangci have no `internal/domain` path.

- [ ] **Step 6: Commit**

```bash
git add internal/echo/echo.go pkg/webui/echo.go
git rm -f internal/domain/echo/service.go
git commit -m "$(cat <<'EOF'
refactor: Move echo behaviour to internal/echo

pkg/webui stays glue. Capability packages export concrete
funcs; no DTO or Service leftover from the template.
EOF
)"
```

---

### Task 3: Docs, agent rules, CI

**Files:**

- Modify: `docs/AGENTS.md`
- Modify: `.cursor/rules/agents-md-alignment.mdc`
- Modify: `.cursor/agents/go-quality-reviewer.md`
- Modify: `.cursor/rules/go-quality-reviewer-routing.mdc`
- Modify: `.github/workflows/ci.yml`
- Do not modify: `README.md`

**Interfaces:**

- Consumes: layout from Tasks 1–2 (`pkg/webui`, `internal/echo`, `test/`)
- Produces: docs/rules that match the spec; CI with no `make test-race`

- [ ] **Step 1: Replace `docs/AGENTS.md`**

```markdown
# Agent context: webui

## Layout

| Path                    | Role                                                                                               |
| ----------------------- | -------------------------------------------------------------------------------------------------- |
| `pkg/webui`             | Public API. Glue only. Compose internal packages.                                                  |
| `internal/<capability>` | Implementation `pkg/` consumes. Not importable outside this module. Echo lives at `internal/echo`. |
| `test/`                 | Integration tests against `pkg/webui` only. External test package (`webui_test`).                  |

Unit tests, when they exist, are `*_test.go` next to the source they cover, same package. Prefer integration tests. Echo is too thin for a colocated unit test.

## Dependency direction

`pkg/webui` → `internal/<capability>`. Never reverse: `internal/` packages do not import `pkg/`.

`internal/` packages may import each other. `pkg/` is the composition root.

Consumers outside this module cannot import `internal/`. No side effects on import.

Keep behaviour in `internal/` packages. `pkg/` wires them and owns public types.

## Tests

`test/` must import `pkg/webui` only, not `internal/`.

---

## Go proverbs ([source](https://go-proverbs.github.io/), caveman compress)

Concurrency channels coordinate mutex serializes · not parallelism · small interface sharp · zero value useful · `any` untyped tame it · gofmt settles bikeshed · tiny copy beats dep hairball syscall/cgo build tags isolate · cgo not Go · `unsafe` no contract · clarity beats wit · reflection stay cold path · errors values inspect wrap once · architecture name docs users · panic stays in `main` / hard startup.

## Uber style distill ([guide](https://github.com/uber-go/guide/blob/master/style.md), caveman compress)

Rare `*Iface` · `var _ I = (*T)(nil)` at export boundary · defer unlock pairs · chan buffer zero or one usually · slice/map copy exported API boundaries · typed errors `%w` chain handle once · assert comma-ok · goroutine bounded ctx/waitgroup · no zombie `init()` · globals inject not mutate · exits from `main` only · strconv hot paths · structs field-named literals · table tests sub `t.Run`.

---

Canon links above beat bullet memory when tradeoff unclear.
```

- [ ] **Step 2: Replace `.cursor/rules/agents-md-alignment.mdc`**

```markdown
---
description: Align all code changes with docs/AGENTS.md (product terms, layout, naming, anti-patterns).
alwaysApply: true
---

# Align with `docs/AGENTS.md`

For every code change in repo (new code, refactor, tests, wiring), follow `docs/AGENTS.md`.
Goal: keep product words, package boundaries, repo conventions aligned.

Before big edits, skim/re-read `docs/AGENTS.md` (and deeper refs it points to). Treat as source of truth for:

- Layout/responsibility: `pkg/webui` is public glue; `internal/<capability>` is implementation; `test/` is integration vs `pkg/` only; rare unit tests are colocated `*_test.go`.
- Naming/imports: `internal/` must not import `pkg/`; `test/` must not import `internal/`; do not invent catch-all packages.
- Errors: wrap with `%w`.
- Anti-patterns: no catch-all `util` / `helpers`; do not put unit tests under `test/`.

If generic advice conflicts with `docs/AGENTS.md`, pick `docs/AGENTS.md`.
If public behavior or big package boundaries change, update `docs/AGENTS.md` in same effort.
```

- [ ] **Step 3: Patch `.cursor/agents/go-quality-reviewer.md`**

Replace these three bullets under “Core Go habits”:

```markdown
- **Structure:** `pkg/` is public glue; `internal/<capability>` holds implementation; no `util`/`helpers` junk-drawer packages; package names singular lowercase.
- **Test placement:** unit tests are rare `*_test.go` next to the source they cover (same package). Integration tests live under `test/`, import `pkg/` only, use the external test package (`webui_test`). Do not import `internal/` from `test/`. Do not mirror paths under `test/unit`.
```

Replace this bullet under “Principles and patterns”:

```markdown
- **Dependency direction:** `pkg/` is the composition root and depends on `internal/<capability>`; `internal/` must not import `pkg/`. Prefer constructor injection over implicit globals.
- **Boundaries:** keep `pkg/` thin glue and behaviour in `internal/`; flag leakage of internal types into the public API.
```

Leave the rest of the reviewer file unchanged (errors, Must\*, workflow, report format).

- [ ] **Step 4: Patch `.cursor/rules/go-quality-reviewer-routing.mdc`**

Replace the second bullet:

```markdown
- Substantive Go changes are made in `pkg/` or `internal/`, especially around concurrency or error handling, before merge.
```

- [ ] **Step 5: Drop the CI race step**

In `.github/workflows/ci.yml`, delete:

```yaml
- name: Race tests
  run: make test-race
```

Leave the existing `Test` step (`run: make test`). That target already uses `-race`.

- [ ] **Step 6: Verify**

Run: `make test && make lint`

Expected: PASS.

Grep (exclude `docs/superpowers/`): no remaining `test/unit`, `internal/domain`, `make test-race`, or “every mode” in `docs/AGENTS.md`, `.cursor/`, `.github/`, `pkg/`, `internal/`, `test/`, `Makefile`.

- [ ] **Step 7: Commit**

```bash
git add docs/AGENTS.md .cursor/rules/agents-md-alignment.mdc .cursor/agents/go-quality-reviewer.md .cursor/rules/go-quality-reviewer-routing.mdc .github/workflows/ci.yml
git commit -m "$(cat <<'EOF'
docs: Align agents with library layout

AGENTS.md is the law for pkg glue, internal capabilities,
and test/ integration. CI already races in make test.
EOF
)"
```

Do not stage `README.md`.
