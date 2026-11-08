# Review 0005: Gemini review of M4.3, M5.1 and M5.2

| | |
|---|---|
| Date | 2026-11-08 |
| Roadmap item | M4.3, M5.1, M5.2, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `3cfb1215-f4ee-4216-9460-fe768245b147`), about 150k tokens. Argon was still out of quota. The input was the diff only, with spec excerpts in the prompt (the new token budget in roadmap.md). |
| Scope | All by Claude: `cfgx.Secret` moved out of the spike; nested groups, `GoName`, `FlagPrefix`, dotted keys, `App.Initialisms` and tags in dsl, ir and gen |
| Result | 5 findings. Claude checked each against the code: 3 accepted and fixed, 2 rejected as false positives. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A command with no groups or tags would generate a constructor without a return. | Rejected: the template's `else` branch returns `&Version{args: ...}`. `TestGeneratedCodeCompiles` vets that case ("no groups"). |
| F02 | Major | A command that also lists a `Global` group in `Uses` would be rejected as using it twice. | Rejected: the global check runs before the duplicate check. The sample schema's `Serve` does exactly this, and the golden test passes. |
| F03 | Major | A nil `*StringKey` passed to `dsl.Group` panics in `Export` instead of being reported, because the interface value isn't nil. | Accepted. `Group` turns a typed nil key into nil, which `Export` reports with the group's position. Test: `TestNilKeyIsAnError`. |
| F04 | Minor | `lowerGo` mangled `GoName`s with digits: `DB1` became `dB1Group`, `HTTP2` became `httP2Group`. | Accepted. Digits after the first capital extend the run, and the last capital is kept only before a lower-case letter. Tests: `DB1`, `HTTP2`, `HTTP2Server`, `S3Bucket`. |
| F05 | Minor | An undeclared tag on a key or group in the IR was ignored silently. | Accepted, though the DSL can't produce one (tags register when created): the generator reports it. |
