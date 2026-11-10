# Review 0030: Gemini review of M7.6 (OnChange)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.6, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `4b249dc1-616b-4ec1-b559-878b4cc09919`), about 112k tokens, on the diff, the new tests and a spec excerpt |
| Scope | M7.6 by Claude: the OnChange queue in the engine's `Writer`, the generated `Key`, `KeyX` and `Change` |
| Result | 10 findings: 3 accepted and fixed, 7 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A callback that panics leaves later writes' changes queued with no one to run them. | Accepted: every callback runs under `recover`; the queue is drained, then the first panic is raised again. Tested. |
| F02 | Critical | A callback's panic surfaces on another writer's goroutine. | Rejected: callbacks run on whichever writer drains the queue, which is what keeps them one at a time without a goroutine to manage; documented on `OnChange`. |
| F03 | Critical (as filed) | Commands have no `OnChange` method yet, and nothing builds a `Change`. | Rejected for M7.6: the command methods (`OnChange`, `Snapshot`, `Store`, `Update`, `Close`) come with the wiring in M7.7. |
| F04 | Major | A panicking callback skips the remaining subscribers for that change. | Accepted, fixed with F01. Tested. |
| F05 | Major | A masked `Format` would hide a change. | Rejected: `Format` is the canonical text, never masked; secrets can't be `Mutable`. |
| F06 | Major | Pointers shown as addresses would report false changes. | Rejected: only mutable keys are compared, through `Format`; the `Show` fallback prints URLs and zones as text. |
| F07 | Major | A write can return before its callbacks run. | Rejected: by design, and documented; the spec only orders callbacks after the swap. |
| F08 | Minor | The queue keeps popped entries alive. | Accepted: the popped slot is cleared. |
| F09 | Minor | Every write compares every mutable key under the lock. | Rejected: tens of keys, and it catches value changes from resets too. |
| F10 | Minor | `OnChange` isn't claimed as a generated name. | Rejected: it is a method, already reserved on command types; a package-level name doesn't clash with it. |
