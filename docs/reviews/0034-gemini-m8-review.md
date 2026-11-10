# Review 0034: Gemini review of M8.2 and M8.3 (`cfgx init`)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap items | M8.2, M8.3, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `13238b6a-dfc8-47a4-b4be-880489df5bd3`), about 215k tokens, on the diff and ADR 0007's decision |
| Scope | `cmd/cfgx/init.go`, the skeleton templates, `init.txtar` |
| Result | 12 findings: 10 accepted and fixed, 2 rejected (the ADR now states both choices). |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Critical | A failed step leaves the files behind, and the retry refuses to overwrite them. | Fixed: init removes everything it made when a step fails. Tested. |
| F02 | Critical | The refusal check misses what `go mod` and `cfgx gen` write. | Fixed: `go.sum`, the generated file, the example config, schema, `.env.example`, `docs/CONFIG.md` and the lock file are checked too. |
| F03 | Critical | The slog bridge puts earlier attributes inside later groups, prints groups as slices, keeps empty attributes, and writes zero times. | Fixed: attributes are named when added, groups flatten to dotted keys, empty ones are dropped, a zero time is left to logrus. Checked against a built project. |
| F04 | Critical | Only `serve` applies the log settings. | Rejected: only commands that load the settings can apply them; `config` and `version` load none, and cfgx's warnings come at warning level, above the default. ADR 0007 says so. |
| F05 | Major | A second Ctrl+C is swallowed during shutdown. | Fixed: the first signal restores the default handling. |
| F06 | Major | "listening" is logged before the port is bound, and `:0` isn't resolved. | Fixed: it listens first and logs the bound address. |
| F07 | Major | Module paths with quotes, newlines or empty elements get through. | Fixed: `module.CheckImportPath`, and a path whose program name would be empty is rejected. Tested. |
| F08 | Major | The config file is named after the program, not the app. | Rejected: the program's name is what users type, so `hello.yaml` for `hello`; ADR 0007 now says `<program>.yaml`. |
| F09 | Minor | `Shutdown` never returns `ErrServerClosed`. | Fixed: the check is gone. |
| F10 | Minor | `go build ./...` leaves no binary. | Fixed: `go build .`; the message says to run it. |
| F11 | Minor | The replace path keeps Windows backslashes. | Fixed: `filepath.ToSlash`. |
| F12 | Minor | `Configure` never switches back to text. | Fixed. |
