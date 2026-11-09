# Review 0022: Gemini review of M6.2 (gen -check, AssertGenerated)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M6.2, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `f642531e-74d9-4227-a90c-5debbc91c386`), about 80k tokens, on the diff alone |
| Scope | M6.2 by Claude: `cfgx gen -check` and `cfgxtest.AssertGenerated` |
| Result | 4 findings: 3 accepted and fixed, 1 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | `AssertGenerated` skips `_test.go` files, though `go generate` reads their directives too. | Accepted. |
| F02 | Major | A quoted argument with a space (`-artifacts "a dir"`) is split at the space. | Accepted: words are split as `go generate` does, with Go-quoted strings. |
| F03 | Major | `-run` or `-skip` before the command in a directive becomes the command. | Rejected: those are flags of `go generate`, not part of a directive. |
| F04 | Minor | A tab after `//go:generate` isn't recognised. | Accepted. `TestGenerateLine` covers F01, F02 and F04. |
