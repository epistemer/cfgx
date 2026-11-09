# Review 0017: Gemini review of M4.10 (completion)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.10, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `60024732-5da4-41dd-92d8-5e1d0291f4b8`), about 1.04M tokens. It was run from a folder holding only the diff, but it read files in the repository anyway, which is why it cost three to five times what earlier reviews did |
| Scope | M4.10 by Claude: `Complete`, `dsl.Files`, `dsl.Dirs`, `dsl.CompleteFunc`, enum and `--config` completion, the engine's `bindCompletions` |
| Result | 3 findings, all accepted and fixed. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | Alias flags get no completion, so they fall back to file names. | Accepted: aliases register the key's completion. `completion.txtar` completes `--log-verbosity`, and fails without the fix. |
| F02 | Minor | `Files("*..pem")`, a trailing dot or space, or braces pass the pattern check, and the shell then filters on a broken extension. | Accepted: an extension must be dot-separated words of letters, digits, `_`, `+` and `-` (`pem`, `tar.gz`). |
| F03 | Minor | `Complete` on an enum silently replaces its values. | Accepted: it's a problem; an enum completes its values. |
