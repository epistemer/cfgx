# Changelog

All notable changes to cfgx are listed here. Versions follow
[semantic versioning](https://semver.org). Until v1.0.0, a minor release
may change the API; the changes will be listed here.

## v0.1.0 (2026-11-11)

The first release. The design is in [docs/interface.md](docs/interface.md),
and the [README](README.md) has a quickstart.

### Schema (`dsl`)

- Declare an app, groups of keys, commands and tags in Go. `cfgx gen`
  runs the schema and generates the settings package, so the schema is
  never linked into the app.
- Key types: `Bool`, `Int`, `Int64`, `Uint`, `Float`, `String`,
  `Duration`, `ByteSize`, `URL`, `Location`, `Path`, `File`, `Secret`,
  `SecretFile`, `Strings`, `Ints`, `Durations`, `StringMap`, `Enum` (its
  Go type inferred from the values) and `Text` (any
  `encoding.TextUnmarshaler`).
- Modifiers: defaults (also computed, with `DefaultFunc`), `Required`,
  `Min`, `Max`, `Range`, `Len`, `Pattern`, custom `Validate` functions,
  `Alias` for renamed keys, `Deprecated`, `Hidden`, `Sources`, short
  flags, and names overridden with `Env`, `Flag`, `Key` and `GoName`.
- Rules across keys: `AtMostOne`, `ExactlyOne`, `AllOrNone`, `Requires`,
  `RelevantIf`.
- Computed values: hand-written group methods, such as `DSN()`, that
  become part of the group's interface.
- Mistakes in a schema fail `go generate` with the file and line.

### Generated code

- A typed getter for every key. Every group and tag becomes an interface,
  and every command a type with exactly the getters it uses, so the
  compiler enforces each command's scope.
- Cobra binders: `XCommand` binds a command's flags and loads its
  configuration. `BindRoot` adds `--config` and the global flags, and
  `ConfigCommand` adds `config init`, `print`, `explain`, `validate`,
  `schema` and `env`.
- `LoadX` loads a command's configuration without cobra, and `Test`
  gives defaults that a test can change.
- `go generate` also writes an example config file, a JSON Schema,
  `.env.example`, `docs/CONFIG.md` and a lock file. `cfgx gen -check` fails
  in CI when any of them is stale.

### Loading

- Sources, highest first: flags, env vars, a YAML or JSON config file,
  defaults. A Mutable key's stored value wins over all of them.
- Every problem is reported at once, with where the value came from.
- Unknown keys in the file and unknown env vars get warnings with "did you
  mean" suggestions. Warnings go through `log/slog`, and `WithLogger`
  routes them into the app's own logger.
- `config explain` shows each value, where it came from, and what it hides.

### Secrets (`cfgx.Secret`)

- A Secret prints as `******` through fmt, JSON, YAML, text marshaling,
  slog and reflection, and a test suite in CI checks every one.
- `Reveal` gives the value to the code that needs it, and `Equal` compares
  in constant time.
- A secret can't be a flag. Its env var also has a `_FILE` form.

### Runtime settings (`store`)

- `Mutable` keys are kept in a built-in bbolt store, in a file that
  `App.Store` names. The app may keep its own buckets in the same file.
- A generated `Update` writes several keys in one transaction, after
  validating the whole configuration, with who made the change and why.
- `OnChange` callbacks run in commit order, and a callback may write
  without deadlocking.
- Each key's history keeps up to 1000 changes. Raising a key's `Epoch`
  moves values stored under an older epoch to history.
- `store.Store` is an admin API over every setting of a command, for a
  settings page or an HTTP endpoint.

### Tooling

- `cfgx init <module>` creates a cobra app with a settings package,
  logrus and an slog bridge, `serve`, `config` and `version` commands, and
  graceful shutdown. It builds and runs with no edits.
- Benchmarks against viper are in [examples/zp/bench](examples/zp/bench).

### Known limits

- Secrets can't be Mutable, because the store isn't encrypted.
- There's no live reload of the config file; mutable keys are the way to
  change settings while running.
- The open API proposals for a later release are in
  [docs/api-review.md](docs/api-review.md).
