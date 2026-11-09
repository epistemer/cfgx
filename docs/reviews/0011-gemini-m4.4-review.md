# Review 0011: Gemini review of M4.4 (Secret and SecretFile)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.4, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `9a71e278-c3d9-4dd3-974b-adbe8c0ab827`), about 395k tokens, run from a folder holding only the diff and short spec excerpts |
| Scope | M4.4 by Claude: the Secret and SecretFile kinds, `_FILE` env vars, file reads in the engine, generator checks, the e2e harness's `writebytes` and `mkfifo` |
| Result | 2 findings, both accepted and fixed. Gemini also checked, and found correct: no secret in any message or help text, no read for keys that aren't relevant or for losing layers, precedence between `X`, `X_FILE`, aliases and the file, FIFO and size-limit handling, and the `_FILE` collision checks. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Minor | With `X` and `X_FILE` both set on a key that isn't relevant, the "set but ignored" warning is dropped, since the conflicting candidate has no raw value. | Accepted: a candidate with a problem counts as set. `TestBothSetOnIrrelevantKeyWarns` fails without the fix. |
| F02 | Minor | On Windows, a config-file path rooted without a drive (`\secrets\key.pem`) or on a drive without a root (`D:key.pem`) is joined onto the config file's directory. | Accepted: only paths that are relative in every sense are joined; `TestRelative` covers both forms on Windows. |
