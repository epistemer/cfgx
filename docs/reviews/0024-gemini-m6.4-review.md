# Review 0024: Gemini review of M6.4 (config subcommand)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M6.4, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `129c51b9-10b4-43c9-ba4f-d6b83eb30c86`), about 218k tokens, on the engine and generator diff |
| Scope | M6.4 by Claude: `engine.ConfigCommand`, `Options.Partial`, the `Show` hooks, profile annotations, generated `ConfigCommand()` |
| Result | 5 findings: 3 accepted and fixed, 2 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | For one command, `print`, `explain` and `env` leave out the `RelevantIf` conditions it reads from other groups. | Accepted: they list the conditions after the command's keys; `config.txtar` checks `config explain web`. |
| F02 | Major | `explain` has no default, though §8 says "value, source, default". | Accepted: a DEFAULT column, with `runtime.NumCPU()` for a `DefaultFunc`. |
| F03 | Major | `config init`'s flags keep their values if the same command runs twice in one process. | Rejected: that is how every cobra flag behaves; `ConfigCommand()` returns a fresh command each call. |
| F04 | Minor | A value with a tab or line break breaks the `explain` table. | Accepted: such cells are quoted. |
| F05 | Minor | `init --force` on a symlink overwrites its target. | Rejected: `--force` is an explicit overwrite, and following the link is what shell redirection and `cp` do. |
