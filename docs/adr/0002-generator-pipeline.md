# ADR 0002: Generator pipeline and where its temporary main lives

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-07 |
| Roadmap item | M1.1 (spike S1) |
| Deciders | Claude (owner) |

## Context

`cfgx gen` runs the user's schema to learn what it declares
([implementation.md §2.1](../implementation.md)). It writes a tiny `main`
package that blank-imports the schema and prints the IR, then runs it with
`go run`. Four questions decided whether that works:

1. Can the temporary `main` import a schema in an `internal/` package, under
   module, workspace (`go.work`) and vendor (`-mod=vendor`) modes, and from
   inside `go generate`?
2. Where can the temporary `main` live? It must be inside the module (for
   `internal/` imports), and `cfgx gen --check` must not modify the working
   tree. Review 0001 (F10) warned that `.`-prefixed directories might be
   rejected.
3. How long does it take, cold and warm? The target was warm under 2 s; the
   kill criterion was warm over 5 s or an unusable mode.
4. Does self-registration work, so the generator needs no `go/packages` pass to
   find the schema's variables?

The spike (`spikes/s1-pipeline`, code at commit `e1e3e77`) used a fixture app
whose schema imports an internal `models` package, which in turn imports a
third-party module so vendor mode has something to vendor. A stand-in generator
tried six placements; `run.sh` reproduces every number below.

## Options

| Placement | How the temporary `main` is run | Touches the tree while running |
|-----------|---------------------------------|--------------------------------|
| `overlay` | `go run -overlay=<json> ./cfgxgen_virtual`, a directory that exists only in the overlay | No |
| `overlay-file` | same overlay, file argument `./cfgxgen_virtual/main.go` | No |
| `underscore` | real directory `_cfgxgen…`, removed afterwards | Yes |
| `dot` | real directory `.cfgxgen…`, removed afterwards | Yes |
| `dot-file` | same, as a file argument | Yes |
| `plain` | real directory `cfgxgen_tmp…`, removed afterwards | Yes |

Results, Linux, Go 1.27.3 and 1.26.0 (times are the inner `go run` only):

| Measurement | Go 1.27.3 | Go 1.26.0 |
|-------------|-----------|-----------|
| All six placements, module mode, warm | all work, 253–328 ms | all work, 220–297 ms |
| Workspace mode (`go.work` above the module) | overlay 95–113 ms, plain 253 ms | overlay 87–90 ms, plain 214 ms |
| Vendor mode with a vendored third-party dependency | all work, 257–330 ms | all work, 222–296 ms |
| Inside `go generate` | overlay 93 ms, plain 253 ms | overlay 85 ms, plain 219 ms |
| Whole `go generate` step, warm, including building the generator | 0.40 s | 0.39 s |
| Cold, fresh `GOCACHE` | overlay 11.2 s, plain 10.8 s | overlay 9.7 s, plain 9.5 s |
| Warm again after cold | overlay 247 ms, plain 404–412 ms | overlay 251–257 ms, plain 393–414 ms |

Observations:

- **Every placement works in every mode.** The F10 worry about `.`-prefixed
  directories doesn't hold for `go run ./.dir` or for file arguments. Those
  placements still write to the tree while they run, though.
- **The overlay is the fastest when warm.** Since Go 1.24 the go command caches
  the executables `go run` builds. A stable virtual path with identical source
  hits that cache, while a freshly named directory is a new package path every
  time, so the link step reruns.
- **Cold runs take about 10 s**, which is compiling the standard library into an
  empty cache. Any first Go build pays it; CI keeps its cache between runs.
- **Self-registration works.** Each constructor records `runtime.Caller` and
  registers itself during the schema's `init`, so the temporary `main` needs
  only a blank import and `Export`.

## Decision

Keep the pipeline; none of the kill criteria fired. The temporary `main` runs
through `go run -overlay`, at a stable virtual path inside the schema package's
directory, with deterministic content. `cfgx gen` therefore never writes to the
working tree, `--check` included, and warm runs hit the executable cache. If a
future toolchain ever rejects the overlay, the fallback is a plainly named
temporary directory removed on exit and on signals.

## Consequences

- [implementation.md §2.1](../implementation.md) records the decision; M2.3
  implements it.
- The generator refuses to run if the virtual directory exists on disk, and
  names it unlikely to collide (`zz_cfgx_gen_main`).
- The schema package and everything it imports run their `init` functions at
  generate time. The risk is already in the roadmap; the generator runs the
  temporary `main` with a timeout.
- The first `go generate` on a fresh machine or CI cache takes about 10 s; later
  runs take well under a second.
