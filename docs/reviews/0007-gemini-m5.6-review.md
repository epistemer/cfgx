# Review 0007: Gemini review of M5.6 (computed methods)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M5.6, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `e5c81ed4-5192-45fd-a7bb-fb304fcab379`), about 210k tokens. The input was the non-test diff, capped at 6 findings. Though the run was "read-only", it left a compiled binary (`testfunc`) in the repository root, which Claude deleted; reviews now run from a scratch folder that holds only the diff. |
| Scope | M5.6 by Claude: `Group.Computed`, the type-checking pass, imports for signatures, name checks |
| Result | 6 findings. Claude checked each: 2 accepted and fixed, 4 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `context` and `slices` are treated as already imported when there are commands, but the template never imports them. | Rejected: the template imports both whenever there are commands, which is when the qualifier treats them as already imported. |
| F02 | Critical | A relative output path would be ignored as an overlay key, so the stale file on disk (or none) would be type-checked. | Accepted defensively (`cmd/cfgx` already passes an absolute path): the path is made absolute. Test: `TestComputedRelativeOutFile`. |
| F03 | Major | A nested group's computed method that clashes with a getter of its parent wasn't caught, since each group only checked its own computed methods. | Accepted. Every group's interface, embedded ones included, is checked for repeated names, each reported once. Test: "nested computed same as a parent getter". |
| F04 | Major | A value-receiver method is accepted, though the spec names `*dbGroup`. | Rejected: a value-receiver method is in `*dbGroup`'s method set and is promoted the same way; nothing breaks. |
| F05 | Major | Type errors elsewhere in the settings package are ignored when every computed method resolves. | Rejected, as decided in implementation §2.3: the stub may not satisfy code that already uses computed methods, and `go build` reports real errors. They are shown when a method can't be resolved. |
| F06 | Minor | Types from vendored standard-library packages would get a `vendor/` import path. | Rejected: user code can't name such a type in a method signature, and module vendoring doesn't prefix import paths. |
