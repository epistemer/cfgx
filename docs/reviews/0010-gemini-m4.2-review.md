# Review 0010: Gemini review of M4.2 (Enum)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.2, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `d9279ec0-325a-4710-964a-099cbf95761f`), about 360k tokens, run from a folder holding only the diff and implementation.md's Enum section |
| Scope | M4.2 by Claude: `dsl.Enum`, `ir.Key.Problems`, the generator's enum tables and import naming, `engine.ParseEnum` |
| Result | 3 findings: 2 accepted and fixed, 1 rejected. Gemini also checked, and found correct, the literal forms for float32, uint64, negative and bool types, the text order, and the zero-value handling for `Required` and rules. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | An enum type from a package named like the generated package (`settings`) shadows it and fails to compile. | Rejected: a package clause declares no name in any scope, so importing another `settings` into `package settings` compiles. |
| F02 | Major | A value listed three or more times is reported once per extra copy, and the checks harness fails on repeated problems. | Accepted: each value is reported once, as "listed more than once"; `enum_twice` lists a value three times. |
| F03 | Minor | A second package named `models` at a path ending in `models2` gets the name `models2`, which matches the path's last element, so the import was written without a name and Go used `models`. | Accepted: `ImportModel` records the package's own name, and the name is left out only when it is both the package's name and the path's last element. The enum e2e script covers it, and fails with the old rule. |
