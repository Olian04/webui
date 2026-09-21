# Agent context: webui

## Layout

| Path               | Role                                                                              |
| ------------------ | --------------------------------------------------------------------------------- |
| `pkg/webui`        | Public API. Declaration AST, split by file. `Compile` lowers to internal runtime. |
| `internal/args`    | Path/query encode-decode, `webui` tags, optionality.                              |
| `internal/rules`   | RE2 compile, constraint check, HTML attrs from `Rules`.                           |
| `internal/runtime` | Compiled app in `ctx`, prefix, leaf ids, handler, `Open` resolve.                 |
| `internal/render`  | IR → HTML.                                                                        |
| `test/`            | Integration tests against `pkg/webui` only. External test package (`webui_test`). |

`pkg/webui` files: `app`, `page`, `table`, `form`, `layout`, `accessor`, `action`, `link`, `open`, `compile`. One package, not one package per type.

Unit tests are `*_test.go` next to the source they cover, same package. Internal packages export concrete funcs so those tests can call them. Prefer integration tests for the public API.

## Dependency direction

`pkg/webui` → `internal/<phase>`. Never reverse: `internal/` packages do not import `pkg/`.

`internal/` packages may import each other. `pkg/` owns public types and `Compile` glue.

Consumers outside this module cannot import `internal/`. No side effects on import.

Keep algorithms in `internal/`. `pkg/` owns the declaration AST.

## Tests

`test/` must import `pkg/webui` only, not `internal/`.

---

## Go proverbs ([source](https://go-proverbs.github.io/), caveman compress)

Concurrency channels coordinate mutex serializes · not parallelism · small interface sharp · zero value useful · `any` untyped tame it · gofmt settles bikeshed · tiny copy beats dep hairball syscall/cgo build tags isolate · cgo not Go · `unsafe` no contract · clarity beats wit · reflection stay cold path · errors values inspect wrap once · architecture name docs users · panic stays in `main` / hard startup.

## Uber style distill ([guide](https://github.com/uber-go/guide/blob/master/style.md), caveman compress)

Rare `*Iface` · `var _ I = (*T)(nil)` at export boundary · defer unlock pairs · chan buffer zero or one usually · slice/map copy exported API boundaries · typed errors `%w` chain handle once · assert comma-ok · goroutine bounded ctx/waitgroup · no zombie `init()` · globals inject not mutate · exits from `main` only · strconv hot paths · structs field-named literals · table tests sub `t.Run`.

---

Canon links above beat bullet memory when tradeoff unclear.
