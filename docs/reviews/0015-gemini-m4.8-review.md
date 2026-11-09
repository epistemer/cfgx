# Review 0015: Gemini review of M4.8 (constraints and function references)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.8, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `f1001dbc-d7c6-441d-81ec-856bbcfb7e2e`), about 316k tokens, run from a folder holding only the diff and short spec excerpts. Argon High, meant for one review per milestone, is out of quota until about 2026-11-14 |
| Scope | M4.8 by Claude: `Range`, `Min`, `Max`, `Len`, `Pattern`, `Validate`, `DefaultFunc`; `FuncForPC` names; the generator's `keyExtras` and `ResolveRefs`; the engine's `Check` hook |
| Result | 8 findings: 5 accepted and fixed, 3 rejected. Fixing F03 exposed a parsing bug that hid packages from the checks, also fixed. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical (as filed) | A `Pattern` without functions leaves `regexp` unimported, since `ResolveRefs` returns early. | Rejected: `Build` already sets the imports from the importer; a probe rendered `"regexp"`. |
| F02 | Critical | An enum or `Text` type declared in the generated package makes it import itself. | Accepted: such a type is now a problem at the key's line; `funcrefs.txtar` covers it. |
| F03 | Major | A package `go list -e` can't load is accepted silently. | Accepted: it's a problem now. Fixing it showed that a package without dependencies at the end of `go list`'s output lost its trailing tab to trimming and was skipped, so it went unchecked; lines are no longer trimmed. |
| F04 | Major | `internalOK` lets any package import a standard library `internal/` package. | Accepted, though the schema itself couldn't import one: a top-level `internal` is never importable now. |
| F05 | Major | Through a symlinked directory (macOS's `/tmp`), the generated package isn't recognised in `go list`'s output, since `Dir` resolves symlinks and `filepath.Abs` doesn't. | Accepted: the package is found by `Match`, the patterns it matched (`.`), and a missing match is an error. |
| F06 | Minor | `internalOK` only checks the last `internal` element of a path. | Rejected: the go command checks the final `internal` element too (`findInternal`). |
| F07 | Minor | A generic function instantiated with a package-qualified type misparses, as the last `/` falls inside the type argument. | Rejected: `FuncForPC` names generic instances `F[...]`, with no type arguments (ADR 0004). |
| F08 | Minor | `--help` shows `(default from v2.Int)` for `math/rand/v2`. | Accepted: `ResolveRefs` uses the package's real name; `constraints.txtar` checks `rand.Int`. |
