# Agent context: webui

## Layout

| Path | Role |
| --- | --- |
| `internal/domain/echo` | Domain logic only. Identical in every mode; no IO imports. |




| `pkg/webui` | Public API: exported facade delegating to the domain. |



| `test/unit/...` | Unit tests beside mirrored paths. |

## Dependency direction

`internal/domain` is the fixed point. Mode-specific IO adapters depend on it; it
depends on nothing but the standard library. Put behaviour in the domain and
keep adapters to translation only.

`pkg/webui` → `internal/domain/echo`. The facade is the public surface
(consumers cannot import `internal/`); no side effects on import.




## Mode notes


- **Logging**: standard library `slog` defaults (no `observability/logging` package).


- **Metrics**: off for this mode.


## Commands (`make`)

`lint`, `test`, `test-race`.

---

## Go proverbs ([source](https://go-proverbs.github.io/), caveman compress)

Concurrency channels coordinate mutex serializes · not parallelism · small interface sharp · zero value useful · `any` untyped tame it · gofmt settles bikeshed · tiny copy beats dep hairball syscall/cgo build tags isolate · cgo not Go · `unsafe` no contract · clarity beats wit · reflection stay cold path · errors values inspect wrap once · architecture name docs users · panic stays in `main` / hard startup.

## Uber style distill ([guide](https://github.com/uber-go/guide/blob/master/style.md), caveman compress)

Rare `*Iface` · `var _ I = (*T)(nil)` at export boundary · defer unlock pairs · chan buffer zero or one usually · slice/map copy exported API boundaries · typed errors `%w` chain handle once · assert comma-ok · goroutine bounded ctx/waitgroup · no zombie `init()` · globals inject not mutate · exits from `main` only · strconv hot paths · structs field-named literals · table tests sub `t.Run`.

---

Canon links above beat bullet memory when tradeoff unclear.
