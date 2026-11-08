# Review 0004: Gemini review of Claude's M3 code

| | |
|---|---|
| Date | 2026-11-08 |
| Roadmap item | M3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through agy's `adversarial_review` (session `be1b2bf2-275f-49f4-9557-b5e54c5faf67`). Gemini 4 Argon (High and Medium) was out of quota for about six days; Argon (High) gets a second pass when it is back (see status.md). |
| Scope | M3.2 and M3.4–M3.7 by Claude: engine (discovery, loading, provenance, binder), dsl, internal/ir, internal/gen, cfgx options. Claude reviewed Gemini's M3.1 (`internal/node`) and M3.3 suggestions (`internal/suggest`) and accepted both unchanged. |
| Result | 8 findings. Claude checked each against the code and interface: 4 accepted and fixed, 1 accepted in a different form, 3 declined. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | `--config ""` fell through to `ZP_CONFIG`, so a lower layer overrode a flag that was passed. | Accepted. A `--config` that was passed decides, even when empty; empty means "search as usual". Test: "an empty --config searches, ignoring the env var". |
| F02 | Critical | The Required message quoted the winning raw text, which for a future secret kind could be the secret (A10). | Accepted. The message never quotes the value: `is required, but is empty` or `is required, but is set to the zero value`. |
| F03 | Major | Extensions were compared case-sensitively, so `--config app.YAML` was "an unknown format" and `app.JSON` skipped the JSON surrogate rewrite. | Accepted. Extensions are compared ignoring case. Test: "extensions ignore case". |
| F04 | Major | Three M3 decisions were said to change the developer API without an amendment: `Global` groups must also be passed to `dsl.App`; `BindRoot` is required; `Required` with a default was a generate-time error. | Partly accepted. The `Required`-with-default check was wrong and is removed: under A4 an empty env var or flag clears a non-zero default, so `Required` still catches it (new test). The other two stand and aren't API changes: interface §3.1's example passes `Log` to both `dsl.App` and `Global`, and §6.1 always calls `BindRoot`, whose `// optional` comment is about the logger. A binder now fails clearly instead of silently dropping `--config` and the global flags. |
| F05 | Major | A command named `Config` would generate `ConfigCommand`, which M6.4's `config` subcommand also generates. | Accepted, more broadly: the generator claims `ConfigCommand`, `Test`, `All`, `Values` and `Load<Command>` now, so no schema that generates today breaks later. |
| F06 | Minor | The package variable `envCaseInsensitive` is changed by a test, which would race with parallel tests. | Declined. No engine test runs in parallel, and putting the switch in `Options` would make an OS fact configurable. |
| F07 | Minor | `engine.roots` holds every bound root forever. | Declined. An app binds one root for its lifetime; tests bind a handful. Cobra annotations can only hold strings. |
| F08 | Minor | A map under a leaf key (`smtp.host: {port: 25}`) is not reported when the running command doesn't load that key. | Declined. Commands validate only their own keys (implementation §3.3); a command that loads the key reports "expected a single value, found a map". |
