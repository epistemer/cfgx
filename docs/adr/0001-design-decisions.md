# ADR 0001: Design decisions D1–D11

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-07 |
| Roadmap item | M0.4 |
| Deciders | epistemer decided; Claude proposed most options |

## Context

cfgx was designed in one session before any code existed, in four phases:
feedback, agreeing on the developer interface, brainstorming the
implementation, and the roadmap. The decisions are spread across the design
conversation and [interface.md](../interface.md). This record lists them in
one place, so later ADRs can refer to them or supersede them. The interface
spec stays the source of truth for details.

## Decisions

| # | Decision | Specified in |
|---|----------|--------------|
| D1 | The source of truth is a Go DSL, executed by the generator at `go generate` time | interface §0, §3 |
| D2 | No struct tags: structs, getters and interfaces are generated output | §0, §5 |
| D3 | Libraries consume configuration only as interfaces of getters | §0, §5.1, §6.2 |
| D4 | Getters are named from the key path, Go-style, without a `Get` prefix: `SMTPHost()` | §0, §3.6 |
| D5 | Viper is replaced; cfgx merges flags, env and files itself, on top of cobra and pflag | §0, §2 |
| D6 | Command-only arguments are declared in the schema with `Command(...).Args(...)` | §0, §3.8 |
| D7 | Tag interfaces are generated only in the central settings package | §0, §5.1, §6.2 |
| D8 | `Secret` is non-generic and string-backed, with a `Bytes()` view | §0, §7.1 |
| D9 | bbolt is built in, reached through an injected `Store()`; no global variables | §0, §7.3 |
| D10 | cfgx logs through `log/slog`, replaceable at startup; apps bridge logrus | §0, §5.2 |
| D11 | MIT license | §0, [LICENSE](../../LICENSE) |

Amendments made after the interface was locked (A1–A11) are listed in
[interface §12](../interface.md). A4–A11 came from
[review 0001](../reviews/0001-gemini-doc-review.md).

## Rejected alternatives

| Alternative | Why it was rejected | Recorded in |
|-------------|---------------------|-------------|
| Struct tags as the source of truth | Tag-heavy config was painful in an earlier project (zp), and tags can't express typed defaults, Go-constant enums or tag interfaces | D2 |
| A YAML schema file instead of a Go DSL | Enum types, constants and validators could only be named by strings, so nothing is checked at compile time | D1 |
| Generating code that reads through viper | Keeps reflection and viper's case-insensitive keys, which fight generated code | D5 |
| A global `runtime.Store` with raw `DB`, `Keys` and `Bucket` | Raw writes skip validation, history and live updates; a `*bbolt.Bucket` is only valid inside its transaction; the bucket doesn't list unstored defaults; the name clashes with Go's `runtime` | A1 |
| logrus as a dependency of cfgx | Forces logrus on every user; logrus is in maintenance mode | A2 |
| Flags and env pinning a key against the store | You chose that the store wins over every other source | interface §4 |
| An env-var reset escape hatch (`ZP_SETTINGS_RESET`) | Rejected by you; recovery is `ResetX()` or an epoch bump | implementation §3.8 |
| Provenance-based rules with shadowing | Value-based rules are simpler to explain and fix the same problems | A4 |

## Consequences

- Changing any of these needs an amendment in interface §12 or a new ADR that
  supersedes this one.
- Spikes S1–S5 (roadmap M1) can still overturn implementation choices, but not
  these decisions. A spike that conflicts with one must say so in its ADR and
  propose an amendment.
