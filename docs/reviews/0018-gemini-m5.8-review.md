# Review 0018: Gemini review of M5.8 (examples/zp)

| | |
|---|---|
| Date | 2026-11-09 |
| Roadmap item | M5.8, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `55354103-5440-4340-b534-35e3367afd68`), about 171k tokens, told to read only the diff. Medium, since the change is example code and a test harness |
| Scope | M5.8 by Claude: `examples/zp` (schema, stubs, `scope_test.go`) and `internal/e2e/examples_test.go`, which builds every example |
| Result | 6 findings: 5 accepted and fixed, 1 rejected. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major (as filed) | Without `TLS` in `Initialisms`, the getters are `SMTPTls()` and `ServerTlsEnabled()`, so the stubs don't compile. | Rejected: TLS is a built-in initialism (implementation.md §2.3a), and the module builds. |
| F02 | Major | If `go generate` fails, the examples test can leave a generated file changed. | Accepted: the committed files are restored in `t.Cleanup`, whatever happens. |
| F03 | Major | The scope test's temporary package sits in the module root, where `./...` matches it, for example after an aborted run. | Accepted: it's created as `_zzscope…`, which `./...` ignores. |
| F04 | Minor | A stale generated file is reported, then vet and test fail on it too, burying the message. | Accepted: the test stops after reporting stale files. |
| F05 | Minor | The scope test checks only `AuthJWTPrivate()`, not that `*TestEmail` lacks every other `Auth` getter. | Accepted: a case checks that `*settings.TestEmail` doesn't implement `settings.Auth`. |
| F06 | Minor | The examples test never runs `go build`, so the binary isn't linked. | Accepted: it runs `go build ./...` first. |
