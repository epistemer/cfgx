# Review 0025: Gemini review of M7.1 (store lifecycle)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.1, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `da43b86b-4e9c-4bdc-9b80-89175bbb0e47`), about 162k tokens, on the diff alone |
| Scope | M7.1 by Claude: `internal/storage`, `cfgx/store`'s bbolt backend, the DSL's `App.Store`, `Mutable`, `Epoch` and `WithStore`, the generator's store checks, and opening the store in `engine.Command` |
| Result | 8 findings: 4 accepted and fixed, 4 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `trim` deletes with a cursor and then calls `Next`, which skips an entry after a delete: trimming more than one entry removes every other one, newest included. | Accepted: `trim` deletes from `First` up to the oldest sequence to keep, `last - max + 1`. `TestTrimSeveral` trims seven at once. |
| F02 | Critical | The command holds the store during `run` but doesn't hand it over, so a `WithStore` command can't use it. | Rejected for M7.1: `Store()` on command types is M7.7. |
| F03 | Major | A read-only open accepts a bbolt file without the `cfgx` bucket. | Rejected: that is a store nothing has written yet, such as zp's existing file, which the app shares with cfgx (§7.3). |
| F04 | Major | The 64 KiB limit doesn't cover history entries. | Accepted. |
| F05 | Minor | An invalid `App.Store` key keeps its index, so commands add it. | Rejected: the problem stops generation, so nothing is generated. |
| F06 | Minor | Counting the history on every write is O(n). | Accepted with F01: trimming seeks by sequence and counts nothing. |
| F07 | Minor | `History(key, 0)` returns everything. | Rejected: 0 or less means all; now documented on `Backend.History`. |
| F08 | Minor | A format of `0` or junk is accepted, or called "newer". | Accepted: either is "isn't one cfgx wrote". |
