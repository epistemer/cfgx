# Review 0033: Gemini review of M7.9 (end-to-end admin script)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M7.9, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `131eae2d-d18c-47df-b4aa-65c4aa3c1801`), about 181k tokens, on the script and the roadmap row |
| Scope | M7.9 by Claude: `internal/e2e/testdata/script/admin.txtar` |
| Result | 11 findings: 5 accepted, 6 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | The epoch bump discarded an injected record, not a value stored through the endpoint. | Accepted: the value the epoch bump discards is now written through the endpoint. |
| F02 | Critical | The env var set for the shadow check stayed set, so fallbacks went to it and never to the default. | Accepted: the shadow check moved to the end; the invalid-record and epoch steps fall back to the default. |
| F03 | Critical | The client could dial before the server accepts connections. | Rejected: the listener is bound before the address is published, so a connection waits in its backlog. |
| F04 | Major | The server is never restarted, only the CLI. | Accepted: the server restarts and serves the stored value. |
| F05 | Major | The server waits only for `/quit`, so a failed step would hang. | Rejected: testscript kills background commands when a script fails. |
| F06 | Major | No `go.mod` in the archive. | Rejected: the harness writes `go.mod` and `go.sum` for every script. |
| F07 | Major | The move to history is checked only through the log line. | Accepted: `show --history` prints the reset entry by cfgx and the old value. |
| F08 | Minor | `inject` doesn't check for missing buckets. | Rejected: the store exists by then; a panic would fail the step clearly. |
| F09 | Minor | `OnChange` output is checked only after the server stops. | Rejected: background output is only available after `wait`; ordering is covered by the engine tests. |
| F10 | Minor | The app's bucket isn't checked across restarts. | Accepted: checked after the restart. |
| F11 | Minor | The script generates and builds twice. | Rejected: the epoch bump needs a new build. |
