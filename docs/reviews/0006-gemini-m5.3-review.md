# Review 0006: Gemini review of M5.3 (rules)

| | |
|---|---|
| Date | 2026-11-08 |
| Roadmap item | M5.3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `b1d319ba-0cd7-41eb-bf07-9ec584bec171`), about 90k tokens. The input was the non-test diff, with the §3.5 rules in the prompt, capped at 6 findings. A first attempt overran the bridge's output limit. |
| Scope | M5.3 by Claude: rules in dsl, ir, gen and engine |
| Result | 6 findings. Claude checked each against the code: 3 accepted and fixed, 1 deferred to M4.1, 2 rejected as false positives. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A rule over a key set only by its default would index an empty origin list and panic. | Rejected: a key with a default always records the default as an origin, and a key with no origin is zero, so never "set". |
| F02 | Critical | `Export` resolves rules before walking the groups, so every rule is dropped. | Rejected: rules are resolved after the walk; `rules.txtar` checks every rule end to end. |
| F03 | Major | A command that loads only a `RelevantIf` key also loaded its condition as its own key, so a `Required` condition failed a command that never uses it. | Accepted. The engine, not the generator, now adds conditions: their flags are bound and their values parsed, but they aren't held to `Required` and take no part in rules. Tests: `TestConditionOnly`, `TestCommandBindsConditionFlags`. |
| F04 | Major | The zero-default test knows only string, int and bool, so M4's kinds (`0s`, `0.0`) would be rejected in exclusive rules. | Deferred: those kinds don't exist yet. M4.1's row now says each kind extends the test. |
| F05 | Major | A `RelevantIf` cycle was blamed on the first key that leads into it, not on a key in it. | Accepted. The error names a key on the cycle. Test: "RelevantIf cycle". |
| F06 | Minor | Setting an irrelevant key to an empty value or `false` warned, though under A4 that isn't "set". | Accepted. The value is parsed into a scratch copy, and only a value that counts as set warns. |
