# Review 0026: Gemini review of M7.2 (store layer)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.2, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `1db40951-538a-46ca-b817-a33ecafed0ce`), about 83k tokens, on the diff alone |
| Scope | M7.2 by Claude: the store layer in `engine.Load`, and opening the store while loading |
| Result | 6 findings: 3 accepted and fixed, 3 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A stored zero skips the checks, as zero means unset, so a required key then fails startup instead of falling back. | Accepted for required keys: an empty stored value for a `Required` key is invalid, ignored with a warning, and the next layer applies. For other keys, zero means unset wherever it comes from (A4). |
| F02 | Major | A stored value is checked against a scratch struct whose other fields are zero, which breaks checks across fields. | Rejected: a key's `Check` reads only its own field (constraints and `Validate(func(T) error)`); checks across keys are rules, which run on the full values. |
| F03 | Major | A store key that fails to load drops out of the pending keys, so loading succeeds without the store. | Rejected: its problem is in the loader's errors, so `Load` returns them and the command doesn't run. |
| F04 | Major | The store key loads before the config file is read, so a path in the file is ignored. | Rejected: the file is read before any key loads; `store.txtar` opens a store named in the config file. |
| F05 | Minor | After a record that fails to decode, the code goes on to the record map. | Accepted as defensive: a key is in one map or the other, but it now returns. |
| F06 | Minor | The warning says "using the default" when the key has none. | Accepted: "leaving it unset" then. |
