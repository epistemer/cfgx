# Review 0001: Gemini review of the design docs

| | |
|---|---|
| Date | 2026-11-07 |
| Roadmap item | M0.5 |
| Reviewer | Gemini 4 Argon (High), through agy's `adversarial_review` (session `34b60709-123e-4208-8264-145c223a0130`) |
| Scope | [interface.md](../interface.md), [implementation.md](../implementation.md) and [roadmap.md](../roadmap.md), as they stood after amendments A1–A3 |
| Result | 15 findings: 8 critical, 5 high, 2 medium. Claude checked each one against the docs and against how Go, cobra and bbolt behave, and accepted all of them. Eight changed the locked interface; you approved them as amendments A4–A11 on 2026-11-07. |

## Findings and resolutions

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A command that reaches part of a group through a tag (`TestEmail` → `auth.emailfrom`) would embed the whole group, gaining getters for keys it never loads and defeating compile-time scoping. | **A5.** Whole groups are embedded; groups reached through a tag get forwarding getters for just the tagged keys. Implementation §3.1, roadmap M5.2 and M5.8. |
| F02 | Critical | `.Default` on `Location`, `URL`, `SecretFile` and `File` takes text, not the getter type. `Test()` would read default key files from disk. | **A6.** Text defaults for text-derived kinds; `Test()` does no I/O. Roadmap M4.5, M6.5. |
| F03 | Critical | With TLS off, the default `tls.key` `SecretFile` is still read eagerly, so `serve` fails to start when the file is absent. | **A4.** `RelevantIf` skips loading and validating the dependent key when its condition isn't set. Implementation §3.3, roadmap M5.3. |
| F04 | Critical | Rules that treat "set" as "came from a non-default source" break: `--tls-enabled=false` counts as set, a file value can't be cleared from env, and a reset leaves an exclusive key set. | **A4.** Value-based rules, chosen over keeping provenance with shadowing. An empty env var or flag clears a value; exclusive rules forbid non-zero defaults. Implementation §2.4 and §3.3, roadmap M5.3. |
| F05 | Critical | `serve` never loads the `App.Store` path key, so it can't find the store. | **A7.** Every command with the store, and the `config` subcommands, load that key. Implementation §3.3 and §3.8, roadmap M7.1. |
| F06 | Critical | Computed methods: the receiver type's name was undocumented, and copying return types from syntax breaks on `/v2` paths, aliases and multiple results. | **A8.** Naming rule documented; signatures resolved by a type-checking pass. Implementation §2.3, roadmap M5.6. |
| F07 | Critical | Write pipeline: concurrent writers can lose updates; `Reset` can't restore the bootstrap value; history drops resets and collides on timestamps. | Implementation §3.2 (bootstrap snapshot) and §3.8 (one writer mutex, per-key history keyed by `NextSequence()`, history for resets). Roadmap M7.1, M7.4. |
| F08 | Critical | `config` wiring: options given to `BindRoot` can't reach the binders without a global; profiles don't know command names; whole-app `validate` would check flag-only `Args`; the `explain` example showed another command's flag. | **A9.** Implementation §3.9: a registry keyed by the root command, and `cmd.Annotations` for profile names. Roadmap M3.5, M6.4. |
| F09 | High | Roadmap dependencies: M5.8 needs M4 and M7 features; M7 needs M4 and parts of M6; the critical path left out M4, M5 and M7. | Roadmap overview, dependency graph, M5.8, lanes and critical path. Recomputed, the two-person estimate of about 18 weeks still holds. |
| F10 | High | The generator loaded types with `go/packages` just to find variable names; a `.`-prefixed temporary directory may be rejected by the go command and modifies the tree during `--check`; the "module graph pruning" claim was wrong. | Implementation §2.1 (DSL self-registration; where the temporary `main` lives moves into spike S1, with `-overlay` first) and §4 (wording fixed). Roadmap M1.1, M2.3. |
| F11 | High | `FuncForPC` references can point into the schema package, at unexported functions or at method expressions, and path splitting needs care. | Implementation §2.2 (acceptance rules) and §2.4 (check). Roadmap M1.3, M4.8. |
| F12 | High | Secrets: mutable secrets weren't rejected; the claim about unexported fields was inaccurate; secret values could appear in error messages; file reads were unbounded; `X` together with `X_FILE` was undefined; the bbolt file mode and history and value limits were unspecified. | **A10.** Implementation §2.4, §3.4, §3.5, §3.6 (closure-based `Secret`) and §3.8 (mode `0600`, 1000 history entries per key, 64 KiB values). Roadmap M1.5, M4.4, M7.1. |
| F13 | High | `SecretFile` and `File` paths weren't resolved like `Path`; a missing `--config` was silently ignored; an unset `$HOME` turned a search path into `/.zp`. | **A11.** Implementation §3.4, roadmap M3.2 and M4.5. |
| F14 | Medium | `engine.Key` had no value formatter or metadata; the binary needs the example config and JSON Schema embedded; provenance must keep every layer; the `CONFIG.md` location differed between docs. | Implementation §1, §2.5, §3.1 and §3.5; `docs/CONFIG.md` everywhere. Roadmap M3.4, M6.1, M9.4. |
| F15 | Medium | Repeated flags would overwrite each other; the default slog output didn't match the documented `warning:` lines; `config` subcommands could create the store file or block on it; env names are case-insensitive on Windows. | Implementation §3.4, §3.5 and §3.8. Roadmap M3.3, M3.5, M4.6, M7.8. |

## Notes from checking

- **F10.** How the go command treats `.`- and `_`-prefixed directories named
  explicitly, and whether `-overlay` can add a file in a directory that doesn't
  exist, weren't verified here. Spike S1 now settles both instead of the docs
  assuming either way.
- **F11.** Go is expected to escape dots in the last path element of symbol names
  as `%2e`, which makes the split between package path and function name
  unambiguous. Spike S3 confirms this rather than the docs relying on it.
- **F12.** The old text said a `Secret` "stays masked" in an unexported field.
  In fact `fmt` prints an address there: the secret isn't revealed, but it isn't
  masked either. The interface now says exactly that.
- **F04.** Value-based rules have one cost: a zero value counts as unset (a port
  of 0, an empty list). It's documented in interface §3.5 and listed as a low
  risk in the roadmap.

## Follow-ups

None outside the docs. Every finding is either applied to the docs or scheduled
as part of a roadmap item.
