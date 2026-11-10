# Review 0035: Gemini review of M9.2 (fuzzing in CI)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M9.2, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `a1e7da2d-bf07-44c0-9e8e-271cccd92f69`), about 207k tokens, on the diff and the roadmap row |
| Scope | New fuzz targets, the formatters' new errors, the CI steps |
| Result | 9 findings: 3 accepted and fixed, 6 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | Go 1.27 doesn't exist, so the fuzz step never runs. | Rejected: the matrix runs Go 1.27.x, and the staticcheck step already runs on the same condition. |
| F02 | Critical | The leak step would pass if its filter matched no test. | Accepted: the step fails unless a `TestLeak_` test passed. |
| F03 | Major | `FuzzParse` assumes the root is a map. | Rejected: `Parse` documents that the root is always a Map; any other root is an error. |
| F04 | Major | Splitting the input at NUL hides NUL bytes and empty lists. | Rejected: NUL isn't special to CSV, and the empty list is in the unit table. |
| F05 | Major | Rejecting every `\r` is too strict: only `\r\n` changes. | Accepted, after checking `encoding/csv`: a lone `\r` round-trips. Only `\r\n` is rejected now; the unit table has lone `\r` items. |
| F06 | Minor | The error blamed the key for a `\r` in the value. | Accepted: it names the whole item. |
| F07 | Minor | Some seeds stop at the formatter's error. | Rejected: they cover the error paths. |
| F08 | Minor | `FormatStrings` changed its signature. | Rejected: `engine` is for generated code, with no compatibility promise. |
| F09 | Minor | Crashing inputs aren't uploaded from CI. | Rejected: the fuzz targets' failure messages print the input. |
