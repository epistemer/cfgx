# Review 0032: Gemini review of M7.8 (explain and read-only opens)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.8, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `57422d6d-c134-4ef8-b816-e0d79a495d76`), about 349k tokens, on the diff and a spec excerpt |
| Scope | M7.8 by Claude: `StoreReadOnly`, the `config` subcommands' store handling, `explain`'s store source |
| Result | 7 findings, all accepted and fixed. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | Whether to load stored values was kept on the shared subcommand struct, so concurrent runs could race on it. | Fixed: it is passed to each subcommand. |
| F02 | Major | An empty store key warned on every `config` run, though a missing file is silent. | Fixed: in read-only mode an empty store key names no file and is skipped silently. Tested. |
| F03 | Major | `explain`'s source column wasn't escaped, so a reason or problem with a tab or newline could break the table. | Fixed: it goes through `cell`. |
| F04 | Minor | A reason without an author read as `2026-11-02: "why"`. | Fixed: date, then `who: "why"`, `who` or `"why"`. Tested. |
| F05 | Minor | A record without a time showed `0001-01-01`. | Fixed: the date is left out. Tested. |
| F06 | Minor | In read-only mode a failed open warned, but failing to read the records stopped the subcommand. | Fixed: both warn and carry on without stored values. |
| F07 | Minor | The "showing values without the store" warning had no slog attributes. | Fixed: `source=store`. |
