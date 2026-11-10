# Review 0038: Gemini review of M9.4 (docs and README quickstart)

| | |
|---|---|
| Date | 2026-11-11 |
| Roadmap item | M9.4, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `6faa6a31-2fee-48b1-92ef-a5a2f419cc66`), about 331k tokens, on the diff, the new README and the two skeleton files the quickstart edits |
| Scope | README, godoc examples for cfgx, dsl and store, the zp example's README |
| Result | 11 findings: 10 accepted and fixed, 1 rejected. The quickstart was rerun step by step after the fixes. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `./hello serve` runs in the foreground, so a newcomer following the steps is stuck, and the next `serve` collides on the port. | Accepted: the README says how to stop it, and the steps that show sources use `config explain`, which doesn't block. |
| F02 | Critical | Nothing tells the newcomer how to see the greeting. | Accepted: each server step says what `curl localhost:8080` answers. |
| F03 | Major | The precedence claim was never shown: each source was set alone. | Accepted: the file and an env var are set together, and the env var wins; a flag then wins over both. |
| F04 | Major | Where the getter goes in `serve.go` wasn't said. | Accepted: the snippet shows the whole handler and the line it replaces. |
| F05 | Major | The mutable snippet used `RateLimit()` without showing the `rate` group. | Accepted: the snippet declares the group, the store key and the App. |
| F06 | Major | `LoadServe` lacked its `settings.` qualifier. | Accepted. |
| F07 | Major | The dsl and store examples have no `// Output:`, so they never run. | Rejected: dsl keeps a global registry that the tests in the same binary share, and the store example needs a running command. Compiling them is what keeps them correct; the cfgx examples, which can run, have output. |
| F08 | Major | The dsl example said "package-level variables, as here" but declares them in a function. | Accepted: it says why the example uses a function. |
| F09 | Minor | The store example's handler read an unbounded body and fell through after an error. | Accepted: it limits the body, returns after errors, and answers 204 on success. |
| F10 | Minor | `ExampleEnum` claimed named types but showed only strings. | Accepted: `ExampleEnum_namedType` shows typed constants. |
| F11 | Minor | The list of rewritten files missed `.env.example`. | Accepted. |
