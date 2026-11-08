# Review 0008: Gemini review of M5.7 (generate-time checks)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M5.7, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `fbb1e38b-5525-466b-8c48-ea2111b7ae7a`), about 50k tokens, run from a folder holding only the diff |
| Scope | M5.7 by Claude: generator warnings, three fixes against doubled reports, the bad-schema harness |
| Result | 4 findings: 2 accepted and fixed, 2 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | A group getter that clashes with the command's `Args()` or a forwarded getter is no longer reported, since the command check now looks only at computed methods. | Rejected: a getter named `Args` is already rejected as reserved, and getters are unique across the app, so only a computed method can clash there. |
| F02 | Major | The harness lets a duplicated report pass, since each copy matches the same want. | Accepted: a problem reported twice fails the test. |
| F03 | Major | Warnings are dropped when a schema has problems, and printed even when a later step fails. | Rejected: problems come first; a warning shown before a later failure is still accurate and useful. |
| F04 | Minor | Only the first want comment on a line is read. | Accepted: a line may carry several. |
