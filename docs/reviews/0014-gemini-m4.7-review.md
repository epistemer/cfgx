# Review 0014: Gemini review of M4.7 (Text keys)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.7, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `ae0ec8dd-a43b-4eea-9551-c95c1f2b27fc`), about 299k tokens, run from a folder holding only the diff and short spec excerpts |
| Scope | M4.7 by Claude: `dsl.Text`, `engine.ParseText` and `NonZero`, the generator's `textInfo`, golden, check, unit and e2e tests |
| Result | No findings. Asked to check the reflect checks on T, the default round-trip, zero-value semantics, type inference in generated `ParseText` calls, and import naming. |

Claude found one case after the review: a non-zero default whose `MarshalText`
writes empty text would load as the zero value, since empty text means not set
(A4). `Default` now reports it, with a test.
