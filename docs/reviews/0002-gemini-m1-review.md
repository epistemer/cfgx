# Review 0002: Gemini review of Claude's M1 work

| | |
|---|---|
| Date | 2026-11-08 |
| Roadmap item | M1, cross-model review before merge |
| Reviewer | Gemini 4 Argon (High), through agy's `adversarial_review` (session `181b6368-cbf5-4ef1-9bdd-e61cb69b644a`) |
| Scope | ADRs 0002–0004, Claude's edits to ADRs 0005 and 0006, the decisions recorded in implementation.md and roadmap.md, and the spike code as evidence |
| Result | 12 findings (5 high, 7 medium). Claude checked each against the spike code; all were accepted and fixed before merge. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | High | The spike's slice flag split CSV inside pflag's `Set`, so pflag could reject a malformed value before cfgx saw it, against ADR 0003's own invariant. Empty occurrences were dropped. | ADR 0003, implementation §3.4: `Set` records each raw occurrence and the engine splits it; an empty occurrence provides an empty list. Roadmap M4.6. |
| F02 | High | A validator in the generated package itself would make generated code import its own package. A function in a schema-local `internal/` package passes the rules but can't be imported by the generated package. | ADR 0004, implementation §2.2: a function in the generated package is referenced without an import, and importability is checked against the generated package's path. Roadmap M4.8. |
| F03 | High | The stand-in generator put its raw argument (`./schema`) into the import and ran `go list` without `-e`, so a schema with a syntax error would fail in `go list` instead of reporting the compiler's error. | ADR 0002, implementation §2.1: resolve with `go list -e`. Roadmap M2.3. |
| F04 | High | v4 returns an error wrapping `io.EOF` for an empty or comment-only file, and a null root for `---`; anchors without aliases weren't rejected. | ADR 0005, implementation §3.4: both become an empty map, and anchors are rejected too. Roadmap M3.1. |
| F05 | High | The planned surrogate-pair rewrite would corrupt `"\\uD83D…"` (an escaped backslash), lone surrogates would still be rejected, and the shorter escape would shift every later column on the line. | ADR 0005, implementation §3.4: a string-aware scanner rewrites pairs and pads the saved bytes after the closing quote, so columns stay exact; lone surrogates become `�`, as `encoding/json` does. Roadmap M3.1. |
| F06 | Medium | ADR 0002 says `cfgx gen` never writes to the tree, but the computed-method pass wrote a stub `zz_settings.go` to disk. | Implementation §2.3: the stub goes through `go/packages`' in-memory overlay. Roadmap M5.6. |
| F07 | Medium | ADR 0002 overstated coverage (not every placement ran in every mode) and the warm figures: editing the schema invalidates the executable cache, and the `go list` call wasn't timed. | ADR 0002 corrected: what was tried, and the realistic cost after a schema edit. |
| F08 | Medium | Overlay edge cases: an `-overlay` already in `GOFLAGS`, symlinked paths, and an `init` that prints to stdout would corrupt the IR. | ADR 0002, implementation §2.1: merge the user's overlay, canonicalize with `filepath.EvalSymlinks`, write the IR to a file. Roadmap M2.3. |
| F09 | Medium | For custom value types, pflag judges a default "zero" by its text, hiding a string default of `0` or `false` and showing `(default 0s)`; slice type labels read badly. | ADR 0003, implementation §3.4: cfgx writes `(default …)` into the usage itself and leaves `DefValue` empty; labels `strings`, `ints`, `durations`. Roadmap M2.4. |
| F10 | Medium | ADR 0006 said both "exact methods from the spike" and "SHA-256 digests" for `Equal`, and the early `false` for an unset secret leaks through timing whether it's set. | ADR 0006 made consistent; `spikes/s5-secret/secret.go` now hashes both values and combines the result with set-ness without branching. |
| F11 | Medium | One `go list -deps` per referenced package would cost 60–100 ms each; implementation §2.2 omitted the `main` rule; `Resolve` didn't check `token.IsIdentifier`. | ADR 0004, implementation §2.2: one batched call, the `main` rule, and both identifier checks. |
| F12 | Medium | The roadmap claimed the spike code was already deleted; M2.3's done-when used `gen --check`, which arrives in M6.2; the generator timeout wasn't scheduled; M4.8 and M4.10 lacked the new function-reference checks. | Spike code deleted before merge (except S5, which waits for M4.3 and doesn't run in CI until then); M2.3, M4.8 and M4.10 updated. |
