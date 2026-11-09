# Review 0023: Gemini review of M6.3 (cfgx diff)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M6.3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `195c864a-70b3-4466-b314-a1f7f073a700`), about 62k tokens, on the diff alone |
| Scope | M6.3 by Claude: `internal/lockdiff` and `cfgx diff` |
| Result | 6 findings: 5 accepted and fixed, 1 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A key renamed with an alias for its old path is also reported as added (breaking if required), and its other changes aren't compared. | Accepted: the old key is compared with the one it became, and the new path isn't reported as added. |
| F02 | Major | A default of the text `none` can't be told from no default. | Accepted: defaults are compared as values and shown quoted, or as "no default". |
| F03 | Major | Dropping an alias isn't reported, though the old name stops working. | Accepted: breaking. |
| F04 | Major | Arguments are matched by their flag, so a `Flag` override change shows as one removed and one added. | Accepted: they're matched by declared name. |
| F05 | Minor | Adding a short flag is called breaking. | Accepted: adding one is safe; changing or removing one is breaking. |
| F06 | Minor | A command without arguments being added or removed isn't reported. | Rejected: command profiles are code, not configuration; a removed one fails to compile where it's used. |
