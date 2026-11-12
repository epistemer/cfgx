# Review 0039: Gemini review of A13 (`App.I18n`, generated labels)

| | |
|---|---|
| Date | 2026-11-12 |
| Roadmap item | A13, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (High), through the `agy` CLI with the brief and diff inline (agy's MCP bridge was down, and Argon was out of quota), about 80k tokens. A first run that read the diff with tools was denied them in headless mode and returned nothing (about 55k tokens). |
| Scope | The diff (DSL, IR, model, render, `cfgx gen`, tests, the zp example, docs) and excerpts of the A13 spec and i18n's rules |
| Result | 4 findings, all rejected with reasons. A comment in `Build` now says why label paths can't collide. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | High | An enum value's path (`auth.otp` + `ttl`) can equal another key's path (`auth.otp.ttl`), giving a duplicate map key and a key registered twice. | Rejected: cfgx already fails such a schema ("file key auth.otp.ttl needs auth.otp to be a map, but auth.otp is a key"), checked by building one. A key is a leaf, so nothing else lives under its path. `Build` now has a comment saying so. |
| F02 | Medium | The README's `settings.L10nRateLimit` should be `L10nServerRateLimit`. | Rejected: the README's schema has a `rate` group with a `limit` key, whose getter is `RateLimit`. The reviewer read the zp example's `server.rate.limit`. |
| F03 | Medium | Claim the identifier `i18n`, which the labels file imports. | Rejected: no generated identifier can be `i18n` (exported names, and unexported ones end in `Group` or start with `new`), so the claim would catch nothing. A hand-written `i18n` in the settings package would clash, as one named `cfgx` or `cobra` already does with zz_settings.go. |
| F04 | Low | `cfgx gen -check` prints the "delete the stub" warning. | Rejected: `-check` prints schema warnings too, and this one is something CI should surface. |
