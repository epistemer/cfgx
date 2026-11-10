# Review 0031: Gemini review of M7.7 (generated wiring)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.7, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `51e2dd1a-48c4-445f-a75f-2a47d5a3e91c`), about 351k tokens, on the diff and a spec excerpt |
| Scope | M7.7 by Claude: `Store`, `Update`, `OnChange`, `Snapshot` and `Close` on command types; `engine.Build`; `LoadX` opening the store |
| Result | 5 findings: 3 accepted and fixed, 2 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A snapshot shares the store, so `defer snap.Close()` closes it for the running command; a snapshot's `Update` changes live values its getters don't show. | Accepted in part: a snapshot's `Close` does nothing. Its `Store` and `Update` work on the live settings, as its documentation now says; its getters stay pinned, which is what a snapshot is for. |
| F02 | Major | A command that asks for the store but loads no mutable key gets `Update` and `OnChange`, which can only fail. | Accepted: they are generated only on commands that load a mutable key. Compile-failure tests cover it. |
| F03 | Major | A nil writer would panic in `store.New`. | Accepted as hardening: loading a command with the store always gives a writer or an error, but the generated constructor now skips `store.New` for a nil writer. |
| F04 | Minor | `LoadX` leaks the store if building the command type panics. | Rejected: the constructor only assigns fields. |
| F05 | Minor | `LoadCommand` doesn't check `res` for nil. | Rejected: `Load` always returns a result. |
