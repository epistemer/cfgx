# Review 0021: Gemini review of M6.1 (generated files)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M6.1, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `7fb95517-92fd-4fdc-a18b-c2b50bde5bba`), about 97k tokens, on the diff of the generator files plus one sample output |
| Scope | M6.1 by Claude: `internal/gen/artifacts.go` (example config, JSON Schema, `.env.example`, `docs/CONFIG.md`, lock file), `cfgx gen -artifacts`, the embedding in `engine.App`, and the examples test |
| Result | 10 findings: 7 accepted and fixed, 3 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `cfgx.lock.json` shows a `Secret`'s default in plain text. | Accepted: the lock file says `******` instead. `TestArtifactsEdgeCases` checks no file shows it. |
| F02 | Major | The schema rejects `port: "7777"` and `"true"`, which cfgx accepts. | Rejected: the schema is deliberately stricter about quoting, steering editors to the canonical form; documented in implementation.md. |
| F03 | Major | The schema rejects null, so uncommenting `# host:` from the example gives an editor error, though cfgx treats null as absent. | Accepted: every key allows null. |
| F04 | Major | A multi-line description breaks the example config and `.env.example`. | Accepted: comments are written line by line; the edge-case test parses the result. |
| F05 | Major | `0` in a `Durations` list is rejected by the schema. | Accepted: items take `0` too. |
| F06 | Major | `Len` on a list becomes `minLength`/`maxLength` on an array. | Rejected: `Len` exists only on `String` keys. |
| F07 | Minor | The size pattern rejects `512kb`. | Accepted: it now matches the engine's units (`b`, `kb`–`pb`, `kib`–`pib`, any case), and no longer accepts a bare `K`, which the engine rejects. |
| F08 | Minor | A default with a line break isn't quoted in `.env.example`. | Accepted: it's quoted, `"a\nb"`. |
| F09 | Minor | A default of `""` shows as no value. | Rejected: an empty value means not set (A4), the same as none. |
| F10 | Minor | A description ending in a period gets two. | Accepted. |
