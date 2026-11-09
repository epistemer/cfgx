# Review 0016: Gemini review of M4.9 (naming and source modifiers)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.9, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `fe6042c2-be55-476e-bc74-6ec20595c55f`), about 203k tokens, run from a folder holding only the diff and short spec excerpts |
| Scope | M4.9 by Claude: `Short`, `Flag`, `Env`, `Key`, `GoName`, `Sources`, `Hidden`, `Example`; collision and reserved-name checks; the engine's `Hidden` and `NoFile` |
| Result | 5 findings: 2 accepted and fixed, 3 rejected because the code outside the diff already handles them. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical (as filed) | A `Secret` can be a command argument, so it gets a flag. | Rejected: the argument code rejects a `Secret` after the lines in the diff (check case `secret_arg`). |
| F02 | Critical (as filed) | Arguments skip the `--help` and `--config` checks. | Rejected: the argument code checks both, after the overrides apply (cases `arg_help`, `arg_config`). |
| F03 | Major | Alias flags of a `Hidden` key show in `--help`. | Rejected: alias flags are always hidden (`MarkHidden`), since they're deprecated names. |
| F04 | Major | `Text(...).Example(v)` gives an empty example when T has no `MarshalText`. | Accepted: `TextKey.Example` reports it, as `Default` does (case `naming_sources`). |
| F05 | Minor | `Sources(FromFile, FromFile)` in an app without a config file slips past the file-only check, which counted sources. | Accepted: sources are deduplicated, and the check asks whether any source is left (case `naming_sources`). |
