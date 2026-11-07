# Review 0003: Gemini review of Claude's M2 code

| | |
|---|---|
| Date | 2026-11-08 |
| Roadmap item | M2, cross-model review before merge |
| Reviewer | Gemini 4 Argon (High), through agy's `adversarial_review` (session `3f952139-a8fd-41f6-a40c-7af9a5eb06bc`) |
| Scope | M2.1–M2.5 by Claude: internal/ir, dsl, engine, internal/gen, internal/loader, cmd/cfgx |
| Result | 13 findings (6 high, 4 medium, 3 low). Claude checked each against the code; all were accepted and fixed before merge. Claude also reviewed Gemini's M2.6 work and changed the e2e harness to derive its go.mod from the repository's module graph. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | High | `Load` parsed every layer, so a bad env var failed startup even when a flag overrode it, and a key could get two errors. | Only the winning layer is parsed. Test: `TestOverriddenBadValueIsIgnored`. |
| F02 | High | A schema with no commands generated an unused `context` import, which doesn't compile. | The import is conditional. `TestGeneratedCodeCompiles` vets five edge-case schemas. |
| F03 | High | Receiver names came from re-splitting the Go name, so `smtp_tls` became `smtptlsGroup` and `db_1` became `dB1Group`, against interface §5.3. | Names are derived from the schema's words (`smtpTLSGroup`, `db1Group`); command key lists are `keys<Name>`. |
| F04 | High | SIGQUIT went to the whole `go run` process group, so the go command dumped its own stacks, and survivors weren't killed. | The temporary main is built with `go build -overlay` and run directly; only the schema program gets SIGQUIT, and os/exec kills it after a delay. |
| F05 | High | Parsing `go list -e` as text mishandled missing packages, zero matches, `.go` file arguments and `package main`. | `go list -e -json`, with clear errors for each case. Script: `resolve_errors.txtar`. |
| F06 | High | Running from the schema's directory lost the caller's module context (vendor, replace), wrote `GOFLAGS=` over `go env -w` settings, and broke relative paths in a user's overlay. | The temporary main lives in the working directory, inside the caller's module. GOFLAGS is only set when a user overlay is merged, with its relative paths made absolute. |
| F07 | Medium | `Command.Uses` matched groups by name, so a lookalike group with an App group's name was silently accepted. | `Export` compares by identity and records orphans; the generator reports them with every other problem. |
| F08 | Medium | Names allowed trailing or double underscores (`host_`, `a__b`), producing broken flags and env vars. | Segments must be lower-case words joined by single underscores. |
| F09 | Medium | Non-canonical defaults in the IR (`TRUE`, `+007`) would generate code that doesn't compile. | Defaults are canonicalized. |
| F10 | Medium | An invalid package name (from `GOPACKAGE`) wasn't checked before writing a file named after it. | Checked in `gen.Build`, before anything is written. |
| F11 | Low | A key with no flag name would register an empty flag. | Guarded in `BindFlags` and `Load`. |
| F12 | Low | An invalid env prefix derived from the app name gave an unclear message. | The message says it was derived and how to set one. |
| F13 | Low | `Problem` and `Errors` lived in `engine`, which has no compatibility promise; interface §2 puts errors in `cfgx`. | Moved to package `cfgx`. |

The review also listed choices the code makes that the interface doesn't
spell out. Claude decided these as owner:
- An empty env var or flag gives the zero value even when the key has a default.
  That follows A4 as you approved it.
- Empty groups, apps without groups, and commands without groups are allowed.
- Integers are base 10, booleans accept what `strconv.ParseBool` accepts, and
  whitespace around them is ignored.

One choice is an API design decision, so it went to you: how a command's run
function gets its positional arguments.
