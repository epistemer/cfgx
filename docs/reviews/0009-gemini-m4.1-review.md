# Review 0009: Gemini review of M4.1 (numeric and duration kinds)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.1, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `6282be43-12d9-481c-a176-00c1cb2033b3`), about 190k tokens, run from a folder holding only the diff |
| Scope | M4.1 by Claude: the generic DSL key base, five kinds, the generator's kinds table, getter imports |
| Result | 5 findings: 1 accepted and fixed, 4 rejected as false positives, two of them after running Go to check. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `ParseDuration("0")` fails with "has no unit". | Rejected: `time.ParseDuration("0")` returns 0, checked by running it; `TestParseDuration` covers it. |
| F02 | Critical | A package needed by both a getter and a computed method is imported twice. | Rejected: the getters' imports are in the computed pass's fixed set, so `q.imports()` leaves them out. |
| F03 | Major | A computed method's package named like a getter's import (`github.com/foo/time`) would reuse the name `time`. | Rejected: fixed import names are reserved in the qualifier, so it becomes `time2`. |
| F04 | Major | `time.Duration(math.MinInt64)` prints a string `ParseDuration` can't read back. | Rejected: Go parses it, checked by running it. |
| F05 | Minor | `ParseUint` rejects `+1`, unlike the other number parsers. | Accepted: a leading `+` is allowed. |
