# ADR 0004: Function references from FuncForPC

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-07 |
| Roadmap item | M1.3 (spike S3) |
| Deciders | Claude (owner) |

## Context

A schema passes functions to `Validate`, `DefaultFunc` and `Complete`. The
generator must turn each function value into a reference that generated code
can import, or reject it with the schema line
([implementation.md §2.2](../implementation.md)). The spike checked whether
`runtime.FuncForPC` names support that reliably. If not, the fallback was
naming functions by string (`dsl.Func("pkg.Name")`).

The spike (`spikes/s3-funcref`, code at commit `e1e3e77`) resolved fifteen
function shapes, split raw names, and compiled a program importing every
accepted reference under a generated alias.

## Options

Raw names reported on Go 1.27.3 and 1.26.0 (`M` is the spike's module path):

| Shape | Raw name | Accepted |
|-------|----------|----------|
| Exported top-level function | `M/fns.Email` | yes |
| In an `internal/` package | `M/internal/ifns.Internal` | yes |
| Dot in the last path element (`yaml.v3`) | `M/yaml%2ev3.Dotted` | yes, unescaped to `M/yaml.v3` |
| Standard library | `strings.ToUpper`, `math.Sqrt` | yes |
| Assembly-backed standard library | `sync/atomic.AddInt32` (no ABI suffix) | yes |
| Unexported function | `M/fns.email` | no |
| Closure | `M/fns.Make.func1` on 1.27.3; inlined into the caller on 1.26.0: `M.TestResolve.Make.func1` | no |
| Closure assigned at package level | `M/fns.init.func1` | no |
| Method value | `M/fns.V.Check-fm`, `M/fns.(*V).PCheck-fm` | no |
| Method expression | `M/fns.V.Check`, `M/fns.(*V).PCheck` | no |
| Generic instance | `M/fns.Max[...]` | no |
| Function in the schema package | `M/schema.Local` | no |

Observations:

- **The `%2e` escape is real.** The package path ends at the first `.` after
  the last `/`, which makes the split unambiguous once escapes are reversed.
- **Closure names depend on inlining**, and can even move into the caller's
  package. The rule doesn't rely on their shape: anything with a further dot
  after the package path is rejected.
- **No ABI wrapper names appeared.** If one ever does, its suffix adds a dot, so
  the same rule rejects it instead of emitting a broken reference.
- **Generated code must import by alias.** A package's name can differ from its
  last path element (`yaml.v3` declares `package yaml`), and aliased imports
  compile in every case, including `internal/` packages inside the module.

## Decision

Keep `FuncForPC`; the kill criterion didn't fire. A function is accepted only
if:
- after its package path, the symbol is a single exported identifier
  (`token.IsIdentifier` and `token.IsExported`)
- its package isn't `main` or the schema package
- the generated package can import it under Go's `internal/` rule, checked
  against the generated package's own path

A function in the generated package itself is referenced without an import,
which would otherwise be an import cycle. Every other referenced package is
imported under a generated alias. (The last two rules come from review 0002,
F02.)

## Consequences

- [implementation.md §2.2](../implementation.md) records the rules; M4.8
  implements them.
- Packages that import `cfgx/dsl` must also be rejected, because generated code
  must never import the DSL. One batched `go list -deps` call checks every
  referenced package (review 0002, F11); the spike didn't test it.
- Rejections point at the schema line where the function was passed, which the
  DSL already records.
