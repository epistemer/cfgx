# Review 0028: Gemini review of M7.4 (the write pipeline)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.4, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `c19ff180-e1b3-4189-bd42-389ba4e57ef0`), about 548k tokens, on the diff, the new tests and a spec excerpt |
| Scope | M7.4 by Claude: `engine.Writer`, `Edits`, the `Load` refactor, the Format helpers, the generated `Update` |
| Result | 9 findings: 3 accepted and fixed, 6 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `FormatStringMap` writes a map key holding `=` as text that parses back to a different key. | Accepted: such a key has no text form (the parser splits at the first `=`), so `FormatStringMap` now returns an error and the write fails. Tested. |
| F02 | Major | `FormatEnum` writes an enum's zero value as `""` even when it isn't one of the values, so the stored value overrides the default with "unset". | Rejected: empty text means unset in every source (A4); `APP_MODE=` does the same. `Required` still rejects it. |
| F03 | Major | A key index out of range panics in `Edits`. | Accepted: `Set`, `SetText` and `Reset` now fail the write with "no key N". Generated code passes constant indexes, but M7.5 maps paths to indexes. Tested. |
| F04 | Major | A file read for the first time during a write that then fails stays in the read cache, so a later write sees stale contents. | Accepted: a write replays with a copy of the cache and keeps it only if it commits. |
| F05 | Major | `FormatLocation` writes a `time.FixedZone` as a name `ParseLocation` can't load. | Rejected: no source can produce a fixed zone, and the write fails validation with a clear problem, changing nothing. |
| F06 | Minor | The config file's env var isn't recorded for replay. | Rejected: a write reuses the parsed config file; discovery isn't replayed. |
| F07 | Minor | Every write parses all of the command's static keys again. | Rejected: writes are rare, and replaying the whole load is what keeps their validation identical to startup's. |
| F08 | Minor | Setting a key to its stored value records no history, dropping `By` and `Reason`. | Rejected: history records changes; a write that changes nothing writes nothing. |
| F09 | Minor | The generated `Update` can't be reached yet. | Rejected for M7.4: the commands' `Update` method is M7.7. The golden is compiled, and the engine tests cover the pipeline. |
