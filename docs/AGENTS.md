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
