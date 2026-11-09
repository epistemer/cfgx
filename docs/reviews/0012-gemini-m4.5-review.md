# Review 0012: Gemini review of M4.5 (Path, File, URL, Location)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M4.5, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `cede73e1-b292-416b-a0b5-5140a79f6efe`), about 384k tokens, run from a folder holding only the diff and short spec excerpts |
| Scope | M4.5 by Claude: the four kinds, defaults parsed when loading, `Key.Resolve`, the parsers, generator kinds, golden, check and e2e tests |
| Result | 2 findings, both about passwords in URL messages; both accepted and fixed. No findings on layering, defaults, path resolution or imports. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical (as filed) | A URL without a scheme is quoted through `Redacted()`, which hides nothing when there is no parsed user info. | Accepted in substance. Gemini's input (`admin_user:hunter2@db.internal:5432/app`) fails earlier in `url.Parse`, as checked by running Go, but `db/app?password=hunter2` leaks the same way. Messages no longer quote a URL at all. |
| F02 | Major | `url.Parse` errors quote parts of the URL: `https://user:hunter2 host/app` gives `invalid port ":hunter2 host" after host`. | Accepted: only the text before the first quote is kept (`invalid port`). `TestParsePathURLLocation` covers both findings and fails without the fix; the e2e script checks the message. |
