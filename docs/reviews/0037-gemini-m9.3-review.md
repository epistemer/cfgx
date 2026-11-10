# Review 0037: Gemini review of M9.3 (API review checklist)

| | |
|---|---|
| Date | 2026-11-11 |
| Roadmap item | M9.3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `5f7971df-ed7b-4576-ba44-199bc6ed57f8`), about 281k tokens, on the diff and the godoc text of `cfgx`, `store` and part of `dsl` |
| Scope | docs/api-review.md, doc-comment fixes |
| Result | 10 findings: 3 fixed in code or docs, 4 turned into or sharpened proposals for the owner, 3 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `Secret.Equal` takes a string, against Go's `Equal(T)` convention, and the checklist implied it compares Secrets. | Partly accepted: the checklist row is corrected, and the naming is proposal P8 (recommendation: keep). Not critical: comparing a candidate string is the method's purpose. |
| F02 | Major | The checklist missed `StringKey.Len` and `Pattern`. | Accepted: added, after listing every key method from godoc to make sure nothing else is missing. |
| F03 | Major | `EnumKey.Example` accepts a value outside the enum. | Accepted and fixed, with a test. `StringMapKey.Example` had the same gap for map keys, also fixed. |
| F04 | Major | Unknown-key errors can't be matched and differ between `Get` and the rest. | Accepted as proposal P6, now recommended before v0.1.0. |
| F05 | Major | `store.Open` and `store.New` have internal types in their signatures. | Rejected as a change: generated code needs a public way to build `*store.Store`, and can't reach an internal package. P4's reasoning now says so. |
| F06 | Major | `WithArgs`, `WithEnv` and `WithFile` do nothing when passed to BindRoot. | Accepted: folded into proposal P2. |
| F07 | Minor | P3 is cheapest to do before v0.1.0. | Accepted in part: P3 now says so, and leaves the choice to the owner. |
| F08 | Minor | dsl's new doc said SecretKey and PathKey have Min and Max, and didn't give G for the keys where it differs from T. | Accepted: rewritten, with a table of every key type where T and G differ. |
| F09 | Minor | `Store.Get` builds the whole list. | Rejected: microseconds for the key counts apps have. The checklist notes it. |
| F10 | Minor | `Store.History` takes no context. | Rejected: a bbolt read can't be cancelled. The checklist notes it. |
