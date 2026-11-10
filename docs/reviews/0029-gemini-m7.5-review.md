# Review 0029: Gemini review of M7.5 (the admin API)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.5, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `c911dc02-c101-49c1-8a20-908e050c37fd`), about 101k tokens, on the diff, the new tests and a spec excerpt |
| Scope | M7.5 by Claude: `store.Store`, `storage.Admin`, the engine's admin methods, per-key schemas in the generator |
| Result | 8 findings: 2 accepted, 6 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical (as filed) | `KeyInfo.Default` would show a secret's default. | Accepted as hardening: generated tables already mask it (`******`), and `adminKey` now masks it too, for hand-built tables. |
| F02 | Critical (as filed) | A secret's default would be in its schema. | Rejected: the schema's `default` comes from `value`, which returns nothing for secrets. |
| F03 | Critical (as filed) | A mutable secret would be stored and shown in clear text. | Rejected: the generator rejects a `Mutable` secret (A10), so no table has one. |
| F04 | Major | Per-key schemas are set in `Artifacts`, not `Build`. | Rejected: `Artifacts` already sets the embedded example config and file schema; `cfgx gen` always calls it before `Render`. |
| F05 | Major | `AdminSettings` reads values after releasing the lock. | Rejected: a write builds new values and swaps the pointer; published values are never changed, so reading them unlocked is safe (the engine tests run with `-race`). |
| F06 | Major | A key reset twice writes two history entries. | Rejected: `Edits` keeps one change per key, so it is reset once. |
| F07 | Minor | A `Format` error leaves `Value` empty. | Accepted: `Value` falls back to the shown value. |
| F08 | Minor | `Get` builds the whole list. | Rejected: a command has tens of keys; one lookup path keeps `Get` and `List` consistent. |
