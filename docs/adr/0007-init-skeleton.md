# ADR 0007: What `cfgx init` creates

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-10 |
| Roadmap item | M8.1 |
| Deciders | Claude, under the ownership terms (the `cfgx init` skeleton is Claude's call); the list was proposed in the roadmap |

## Context

`cfgx init <module-path>` should leave a project that builds and runs with
no wiring by hand (problem 3). The roadmap proposed a list of standard
pieces; it had no answer. The skeleton is a starting point people edit, so
it should show each piece of the wiring once, in the simplest form that
works, and nothing an app has to delete first.

## Options

1. **Minimal:** a root command and a schema. Small, but every project then
   writes the same logger, signal and version wiring by hand, which is the
   problem `init` exists to solve.
2. **The roadmap's list:** a cobra root with an example `serve` command; a
   settings package and schema whose `Log` group configures logrus, with an
   slog bridge that routes cfgx's warnings into it; a signal-aware context;
   a `version` command from build info; cobra's completion; the go.mod
   `tool` directive; `.gitignore`; README.
3. **The list with log/slog instead of logrus.** One dependency fewer, and
   slog is the standard library; but the apps this tool is first for log
   with logrus, and cfgx's own warnings go through slog, so the bridge is
   exactly the wiring people get wrong.

## Decision

Option 2. The skeleton is:

- `main.go`: the cobra root, `settings.BindRoot` with `cfgx.WithLogger` set
  to the slog bridge, `serve`, `version` and `config` commands, cobra's
  default `completion` command, and `ExecuteContext` with a context from
  `signal.NotifyContext` (interrupt, and SIGTERM where it exists).
- `internal/settings/settings.go`: the `//go:generate go tool cfgx gen
  ./schema` line.
- `internal/settings/schema/schema.go`: a global `Log` group (level,
  format) and a `Server` group (addr, shutdown timeout); the app reads
  `<program>.yaml` from the working directory; `Serve` uses `Server`.
- `internal/logging/logging.go`: `New() *logrus.Logger`, the logger before
  the configuration loads (cfgx's own warnings need it during loading);
  `Configure(*logrus.Logger, settings.Log)`, which a command that loads the
  settings calls; and `Slog(*logrus.Logger) *slog.Logger`, the bridge.
  `config` and `version` load no log settings and log at the default level.
- A first interrupt or SIGTERM cancels the context; a second one stops the
  program at once.
- `cmd`-less layout: the module root is the program.
- `.gitignore` (the binary, `*.db`) and a README that says how to add a key.
- go.mod: `go mod init`, then `go get -tool github.com/epistemer/cfgx/cmd/cfgx`
  at the version of the running cfgx, then `go mod tidy`, `go generate`,
  `go mod tidy` and `go build .`, which leaves the program built.

`init` refuses to run if any file it or the go command would write exists,
and removes what it made if a step fails, so it can run again.

## Consequences

- Projects depend on logrus; one that wants slog only replaces two
  functions in `internal/logging`.
- M8.2 implements this with templates embedded in `cmd/cfgx`, golden-tested;
  M8.3 runs it end to end. `init` needs the network for `go get` and
  `go mod tidy`, like any new module.
- The README quickstart (M9.4) starts from this skeleton.
