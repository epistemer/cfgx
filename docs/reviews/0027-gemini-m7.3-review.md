# Review 0027: Gemini review of M7.3 (epochs)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `52576fa6-a956-4615-a3af-f8e3c7193f17`), about 65k tokens, on the diff alone |
| Scope | M7.3 by Claude: epochs in the store layer |
| Result | 2 findings, both rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical (as filed) | A record from a newer epoch marks the key invalid, which fails loading. | Rejected: `Result.Invalid` only records why a stored value was ignored, for `explain`; it isn't a problem. `TestEpochs` loads the default with no error. |
| F02 | Minor | Without a writable store, retiring older records is skipped silently after saying they move. | Rejected for M7.3: records are read only when the store is open read-write. The read-only opens of the `config` subcommands come in M7.8, which must not retire records; noted there. |

The end-to-end test M7.3 asks for needs stored values written, which comes
with M7.4; M7.9's end-to-end script covers an epoch bump.
