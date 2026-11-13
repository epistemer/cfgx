# Status log

Newest first. Every working session adds an entry: what landed, the decisions
taken for you, what's next, and what's waiting on you. The terms are in
[ownership.md](ownership.md).

## 2026-11-13: labels as data (A14, ADR 0008); design only

- **Landed:** design records, no code:
  [ADR 0008](adr/0008-labels-as-data.md), amendment A14 in interface §12
  with notes in §3.7 and §5.5, roadmap milestone M11, and a note in
  implementation §2.5. They are in
  [PR #41](https://github.com/epistemer/cfgx/pull/41), which is open and not
  merged: an API change is yours to approve.
- **Asked by you:** a second, critical look at the generation pipeline you
  proposed for neozp (generators publish metadata, other packages ship
  declarative generators over it, a driver runs them), and then the records
  for it here and in i18n.
- **Decided by you:** the direction. It replaces A13's rule that every
  generator writes its own locales.
- **Decided for you, overturnable:**
  - cfgx writes the labels itself, as a `labels/v1` file. I had first
    suggested publishing cfgx's model for an adapter to convert; the second
    look reversed that, because `Model.L10n` already is the list, with its
    names claimed against the generated package.
  - `cfgx.lock.json` stays cfgx's own file. Other packages shouldn't read it.
  - The API is `App.Labels()`, still opt-in. The file is
    `zz_<package>.labels.json`, next to the generated code.
  - `App.I18n()` goes without a deprecation period, since no tag has it, and
    no tag is cut between M11.1 and M11.2.
- **Checked:** the 62 labels of `examples/zp`, copied by a script from
  today's `zz_settings_i18n.go` into a `labels/v1` file and rendered by a
  prototype of i18n's generator (one jq query, one Go template), give the
  same 117 lines of declarations as that file. Only comments differ.
- **Not reviewed yet:** this session ran in Claude's cloud workspace, which
  can't reach agy, so Gemini hasn't seen the ADR. The PR is the place for
  that review.
- **Next:** nothing in cfgx until the driver's spikes freeze `labels/v1`
  (driver M1.6); then M11.1.
- **Waiting on you:**
  - the PR;
  - the note in your global `CLAUDE.md` that every neozp generator emits its
    locales, which A14 makes out of date;
  - the earlier open items: the API proposals and v1.0.0.

## 2026-11-12: locales for the generated settings (A13)

- **Landed:** `App.I18n()`. With it, `cfgx gen` also writes
  `zz_<package>_i18n.go`: an epistemer/i18n key for the label of every
  group, key and enum value, named `settings.<path>`, with the description
  as its English text. Each group and key gets an `L10n<Name>` variable, and
  `L10n(path)` finds any label by the path `Store().Keys()` gives.
  `examples/zp` uses it, and a new test loads a real translator over the
  labels and renders an Arabic translation. Interface §3.7, §5.5, §9 and
  amendment A13; README; CHANGELOG under Unreleased.
  - The IR version is now 18, so every `cfgx.lock.json` changes one line.
  - Dropping `.I18n()` turns the file into a stub and prints a warning:
    deleting it would fail `go generate`, which had already listed it. Found
    by trying it on the zp example.
- **Asked by you:** generated locales, following zp's `gen` conventions with
  epistemer/i18n. A global note in `~/.claude/CLAUDE.md` now says every
  neozp code generator emits its locales.
- **Decided by you:** merge A13 as designed. I proposed it opt-in, because
  cfgx also serves apps that don't use i18n, and generating it always would
  make every one of them require i18n. The key names (`settings.` prefix,
  enum values under their key), the `L10n` names, and no labels for command
  Args stand as proposed.
- **Fixed after the PR opened:** the benchmark module, `examples/zp/bench`,
  imports the zp example's settings, so it needs i18n in its go.mod too. CI's
  benchmark step caught it.
- **Reviewed:** Gemini 3.8 Flash (High), about 80k tokens, plus about
  55k for a first run that headless mode denied its tools
  ([review 0039](reviews/0039-gemini-a13-review.md)). Argon was out of
  quota (it resets around 2026-11-15), and agy's MCP bridge was down, so it
  ran through the agy CLI. 4 findings, all rejected with reasons; the
  biggest, a path collision, is a schema cfgx already refuses.
- **Next:** nothing on the roadmap; Argon's milestone reviews wait for its
  quota.
- **Waiting on you:** the earlier open items: the API proposals and
  v1.0.0.

## 2026-11-11: v0.1.0 released; M9 complete

- **Landed:** [CHANGELOG.md](../CHANGELOG.md), then the `v0.1.0` tag.
  `go install github.com/epistemer/cfgx/cmd/cfgx@v0.1.0` works through the
  module proxy. Every roadmap item through M9 is done.
- **Decided by you:** tag v0.1.0 as is. The API proposals P1–P8 in
  [api-review.md](api-review.md) stay open for a later release,
  including the three I recommended (P2, P6, P7).
- **Next:** nothing on the roadmap. The ordered backlog after v0.1.0 is
  in the roadmap; Argon High's milestone reviews of M3, M4 and M7 can run
  once its quota is back, around 2026-11-14.
- **Waiting on you:** whether to take up any API proposals, and when to
  call it v1.0.0.

## 2026-11-11: M9.4 docs and README

- **Landed:** a new README. It says what cfgx does and has a quickstart
  that goes from `cfgx init`, through adding a key and `go generate`, to
  setting the key from a flag, an env var and the file. It also gives a
  tour of a larger schema, mutable settings, testing, the benchmark
  numbers, and links. "Pre-alpha" is gone; the status line says v0.1, and
  that the API may change before v1.0.0.
  - The quickstart was run step by step in an empty directory, and its
    output is pasted from that run.
  - Godoc examples: `Secret`, `Secret.Equal` and `Errors` in cfgx, which
    run and check their output; a whole schema, `RelevantIf` and `Enum`
    in dsl; an HTTP admin endpoint in store.
  - The zp example's README no longer promises store keys "in M7.9"; it
    points to the end-to-end script that has them.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 331k tokens
  ([review 0038](reviews/0038-gemini-m9.4-review.md)). 10 of 11 findings
  fixed. The worst: the quickstart's server blocked the terminal, and
  nothing said how to see its answer.
- **Next:** M9.5, the CHANGELOG and the v0.1.0 tag.
- **Waiting on you:** the API proposals in api-review.md, three of
  which I recommend before v0.1.0.

## 2026-11-11: M9.3 API review

- **Landed:** [api-review.md](api-review.md) checks every exported
  identifier in `cfgx`, `dsl` and `store`: whether it's needed, its name,
  its docs, whether misuse fails loudly, and whether it leaks internals.
  `engine`'s package doc now says it has no compatibility promise.
  - Doc fixes: `WithArgs` had a broken sentence, and the load options now
    say only LoadX uses them. `Uses` takes tags too. dsl explains the
    `K`, `T` and `G` that its method docs show.
  - Bug fixes: an `Enum` example outside the enum's values, or a
    `StringMap` example with a bad map key, used to pass silently. Both
    now fail at generate time, as a bad default does.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 281k tokens
  ([review 0037](reviews/0037-gemini-m9.3-review.md)). 7 of 10 findings
  fixed or turned into proposals.
- **Next:** M9.4 (docs and the README, which drops "pre-alpha"), then
  M9.5 (v0.1.0).
- **Waiting on you:** eight API proposals (P1–P8 in api-review.md).
  I recommend three of them before v0.1.0:
  - P2: options that do nothing where they're passed should fail.
  - P6: add `store.ErrUnknownKey`.
  - P7: `cfgx gen` should reject an app with no keys.

## 2026-11-10: M9.1 benchmarks against viper; M10 dropped

- **Landed:** `examples/zp/bench`, a module of its own so viper never
  becomes a dependency, compares cfgx with viper on the same inputs. The
  README has the numbers, measured on a 2015 laptop:
  - zp's `serve` (39 keys) loads in 0.31 ms with cfgx, 0.56 ms with viper.
  - 100 keys load in 0.46 ms with cfgx, 1.85 ms with viper.
  - `cfgx gen` on zp takes 1.4 s with a warm build cache and 20 s with a
    cold one, about what a cold `go build` of the app takes.
  - CI runs each benchmark once (the cold one excepted) and checks the
    generated code is current.
- **Found on the way:** a schema that builds its keys in `init` generates
  an app with no keys, since `init` runs after `App`'s initializer. The
  bench README says so; M9.4's docs will too.
- **Roadmap:** M10 (the zp port) is gone, at your request. The ownership
  terms now say v1.0.0 is your call.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 255k tokens
  ([review 0036](reviews/0036-gemini-m9.1-review.md)). 6 of 10 findings
  fixed, most of them making the viper side fairer.
- **Next:** M9.3 (API review), M9.4 (docs and the README, which drops
  "pre-alpha"), M9.5 (v0.1.0).
- **Waiting on you:** nothing.

## 2026-11-10: M9.2 fuzzing in CI

- **Landed:** CI fuzzes seven targets for 30s each: the parsers, the
  canonical text of lists and maps, and the config file parser on any
  input. The Secret leak suite is its own step, which fails if it finds
  no tests. Both run inside the required job.
- **Found by fuzzing:** an item holding `\r\n`, or a map key that is
  empty, holds `=` or has outer spaces, doesn't survive its text form. A
  write with such a value now fails with a clear error instead of storing
  something else.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 207k tokens
  ([review 0035](reviews/0035-gemini-m9.2-review.md)). 3 of 9 findings
  fixed.
- **Next:** M9.1 (benchmarks), M9.3 (API review), M9.4 (docs and the
  README, which drops "pre-alpha"), M9.5 (v0.1.0).
- **Waiting on you:** nothing.

## 2026-11-10: M8 `cfgx init`; M8 complete

- **Landed:** `cfgx init <module path>` creates a cobra application with
  a cfgx settings package (ADR 0007). It includes `serve`, `config` and
  `version` commands, a signal-aware context, logrus with an slog bridge
  that routes cfgx's warnings into it, `.gitignore` and a README. It then
  runs `go mod init`, adds cfgx as a tool, generates and builds.
  - It refuses to overwrite anything, and undoes its work if a step fails.
  - `init.txtar` runs it in an empty directory, builds, and runs
    `serve --help`, `config explain` and `version`.
  - The skeleton was also tried by hand: it serves a request and shuts down
    on SIGTERM.
- **Decided for you:**
  - The skeleton follows the roadmap's proposal, with logrus (ADR 0007).
    The ownership terms make the skeleton my call.
  - The config file is `<program>.yaml`.
  - `-cfgx dir` points the new module at a local cfgx checkout.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 215k tokens
  ([review 0034](reviews/0034-gemini-m8-review.md)). 10 of 12 findings
  fixed.
- **Next:** M9, hardening and v0.1.0: benchmarks, fuzzing in CI, the API
  review, docs, the release.
- **Waiting on you:** nothing.

## 2026-11-10: M7.9 end-to-end admin script; M7 complete

- **Landed:** `admin.txtar`, in which a server exposes a small HTTP admin
  endpoint over `Store()`. It shows that:
  - an update fires `OnChange`, is validated (a bad value is rejected) and
    survives a server restart
  - the app's own bucket lives in the same file across restarts
  - an invalid stored record falls back to the default
  - an epoch bump moves the value stored through the endpoint to history,
    once
  - a stored value that hides an env var warns at startup
- **Milestone:** M7, runtime-mutable settings with the built-in bbolt
  store, is complete.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 181k tokens
  ([review 0033](reviews/0033-gemini-m7.9-review.md)). 5 of 11 findings
  accepted.
- **Pending:** the M7 milestone review on Argon (High) once its quota
  resets (about 2026-11-14), with the earlier M3 and M4 ones.
- **Next:** M8, `cfgx init`.
- **Waiting on you:** nothing.

## 2026-11-10: M7.8 explain and read-only opens

- **Landed:** `config explain` shows a stored value as
  `store (2026-11-02, admin@zp: "PCI review"), hides env ZP_X`. For a
  stored value that was ignored, it shows the source that applied, then
  why. `print`, `explain` and `validate` open the store read-only:
  - a missing file is never created
  - a store another process holds is skipped with a warning
  - values from older epochs are ignored without being moved to history
  - `init`, `schema` and `env` never open the store
  The store e2e script covers the missing, locked and stored cases, and
  the startup warning about a hidden env var.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 349k tokens
  ([review 0032](reviews/0032-gemini-m7.8-review.md)). All 7 findings
  fixed.
- **Next:** M7.9, the end-to-end admin test, which closes M7.
- **Waiting on you:** nothing.

## 2026-11-10: M7.7 generated wiring

- **Landed:** commands that have the store get `Store()`, `Snapshot()`
  and `Close()`. Those that load a mutable key also get `Update()` and
  `OnChange()`. Getters read the store's live snapshot, so an update shows
  at once. Binders close the store after `run`. `LoadX` opens it, and the
  caller closes it.
  - The store e2e script writes through the generated `Update`, sees
    `OnChange`, reads the value back after a restart, lists settings
    through `Store().List()` and loads with `LoadServe`.
  - A command without the store has none of these methods; compile-failure
    tests check that.
- **Decided for you:**
  - `Snapshot()` returns the command type with pinned getters. It shares
    the store: `Store` and `Update` act on the live settings, and its
    `Close` does nothing.
  - A command that asks for the store (`WithStore`) but loads no mutable
    key has no `Update` or `OnChange`.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 351k tokens
  ([review 0031](reviews/0031-gemini-m7.7-review.md)). 3 of 5 findings
  fixed.
- **Next:** M7.8, `config explain` and the admin `List` with store details,
  and read-only opens for the `config` subcommands.
- **Waiting on you:** nothing.

## 2026-11-10: M7.6 OnChange

- **Landed:** the engine reports which settings a write changed. Callbacks
  run one at a time, in the order writes committed, after the new values
  are in place and outside the write lock, so a callback may write too.
  A write that changes no value (say, a reset to the value the env var
  gives) calls nothing. A callback that panics doesn't stop the others;
  the panic is raised again once they have run. The generator emits
  `Key`, a `KeyX` constant per mutable key, and `Change` with `Has` and
  `Keys`.
- **Decided for you:**
  - `KeyX` constants exist for mutable keys only, as `Key` strings holding
    the key's path.
  - `Change` also has `Keys()`.
  - A write can return before its callbacks run, when another writer is
    running callbacks at the time.
  - The command methods `OnChange()` and `Snapshot()` move to M7.7, with
    the rest of the wiring.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 112k tokens
  ([review 0030](reviews/0030-gemini-m7.6-review.md)). 3 of 10 findings
  fixed.
- **Next:** M7.7, the generated wiring on command types.
- **Waiting on you:** nothing.

## 2026-11-10: M7.5 admin API

- **Landed:** `store.Store`, the admin API for a settings dashboard: `Keys`
  (kind, tags, a JSON Schema per key), `Get`, `List` (value, source, the
  sources a stored value hides, the last change, why a stored value was
  ignored), `Update` and `Reset` with text values through M7.4's pipeline,
  `History`, `DB` and `Bucket`. Every kind's text round-trips: writing back
  what `Get` returns changes nothing.
- **Decided for you:**
  - `store.Record` has a `Reset` field, which interface §7.3 doesn't list.
    Without it, a dashboard can't tell a reset in the history from a value
    set to "".
  - `store.New` is exported for generated code (M7.7 calls it).
  - A static key's `Value` is what `config explain` shows (secrets masked,
    URL passwords redacted); a mutable key's is the canonical text `Update`
    takes back.
  - `Schema` describes the value as the config file takes it (a list is an
    array); `Update` takes the canonical text.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 101k tokens
  ([review 0029](reviews/0029-gemini-m7.5-review.md)). 2 of 8 findings
  accepted.
- **Next:** M7.6, `OnChange`, `Change.Has`, `KeyX` constants and
  `Snapshot()`.
- **Waiting on you:** nothing.

## 2026-11-10: M7.4 write pipeline

- **Landed:** writes to mutable keys, all or nothing.
  - The engine's `Writer` loads the command's keys again from the sources
    they loaded from at startup, with the stored records as they will be.
    So a write is validated exactly as startup is: checks, `Required` and
    rules, across static and mutable keys. A reset brings the flag, env,
    file or default value back.
  - One mutex covers building, validating, the bbolt transaction (history
    for every change, resets included, stamped with the key's epoch) and
    the snapshot swap. A test shows a missing lock loses updates.
  - The generator emits an `Update` type with `SetX`/`ResetX` per mutable
    key, and `cfgx.By`/`cfgx.Reason` go into history. Commands get their
    `Update` method in M7.7.
- **Decided for you:** a `File` key can't be `Mutable` (its value is a
  file's contents, which a write can't store), and a `Mutable` Text key's
  type needs `MarshalText`. Both are generate-time errors.
- **Reviewed:** Gemini 3.8 Flash (High), about 548k tokens
  ([review 0028](reviews/0028-gemini-m7.4-review.md)). 3 of 9 findings
  fixed.
- **Next:** M7.5, the admin API on `store.Store`.
- **Waiting on you:** nothing.

## 2026-11-10: M7.3 epochs

- **Landed:** raising a key's `.Epoch(n)` makes values stored under an
  older epoch move to history at the next start, with a log line. A value
  from a newer epoch, written by a newer release, is ignored but kept, so a
  rollback doesn't lose it.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 65k tokens
  ([review 0027](reviews/0027-gemini-m7.3-review.md)). Both findings
  rejected.
- **Note:** the end-to-end epoch test comes with M7.9, once M7.4 can write
  values.
- **Next:** M7.4, the write pipeline and the typed `Update`.
- **Waiting on you:** nothing.

## 2026-11-10: M7.2 store layer

- **Landed:** a mutable key's stored value now wins over flags, env, the
  file and the default.
  - A stored value that no longer passes the schema is ignored with a
    warning. That covers bad text, a failed check, and an empty value for a
    required key. The next layer applies, and the record stays in the
    store.
  - A stored value that hides a flag or env var says so at startup.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 83k tokens
  ([review 0026](reviews/0026-gemini-m7.2-review.md)). 3 of 6 findings
  fixed.
- **Next:** M7.3 (epochs), then M7.4, the write pipeline.
- **Waiting on you:** nothing.

## 2026-11-10: M7.1 store lifecycle

- **Landed:**
  - `cfgx/store`, the built-in bbolt store: mode 0600, a `cfgx` bucket
    with `settings`, per-key `history` capped at 1000 entries, and `meta`
    with a format version.
  - `App.Store(pathKey)`, `.Mutable()`, `.Epoch(n)` and
    `Command.WithStore()` in the DSL, with generate-time checks. Mutable
    secrets are rejected (A10).
  - Commands that have the store open it after loading, with a one-second
    wait for another process, and close it after `run`.
  - The engine reaches bbolt only through the opener the generated code
    passes. `examples/mini`, which has no store, gained no bbolt
    dependency.
- **Reviewed:** Gemini 3.8 Flash (High), about 162k tokens
  ([review 0025](reviews/0025-gemini-m7.1-review.md)). 4 of 8 findings
  fixed. The important one: trimming more than one history entry at once
  skipped every other entry, a bbolt cursor gotcha.
- **Also:** bbolt v1.5.0 raises pflag to v1.0.10, the latest v1.0.x, which
  ADR 0003 allows. Both examples are tidied for it.
- **Next:** M7.2, layering stored values over the other sources.
- **Waiting on you:** nothing.

## 2026-11-10: M6.7 checked in VS Code; M6 done

- **Checked by you:** VS Code's YAML extension (yaml-language-server) reads
  `examples/zp/config.schema.json` through a `$schema` modeline. It shows
  the schema's title and each key's description, and flags
  `database.port: 70000` with "Value is above the maximum of 65535"
  ([screenshot](images/m6.7-vscode-schema.png)).
- **M6 is done.** `examples/zp` ships every generated file, and the
  examples test, run in CI, fails on any stale one: the milestone's exit
  criterion.
- **Next:** M7, settings that change at runtime, kept in the built-in
  bbolt store.
- **Waiting on you:** nothing.

## 2026-11-09: M6.4 config subcommand

- **Landed:** `settings.ConfigCommand()` adds `config init`, `print`,
  `explain`, `validate`, `schema` and `env`.
  - Each covers the whole app, or one command's keys by its command line,
    nested commands included: `zp config validate db migrate`.
  - Values come from defaults, the file and env only, never a command's
    flags.
  - Secrets are masked and URL passwords redacted everywhere; `env` never
    prints values.
  - `print` and `explain` work on an incomplete configuration. `validate`
    checks everything a command would.
  - `examples/zp` has it.
- **Reviewed:** Gemini 3.8 Flash (High), about 218k tokens
  ([review 0024](reviews/0024-gemini-m6.4-review.md)). 3 of 5 findings
  fixed, including `RelevantIf` conditions missing from one command's
  listing, and a DEFAULT column for `explain`.
- **Decided for you** (implementation.md §3.4):
  - `init` writes `config.yaml` in the working directory, or `-o path`,
    and won't overwrite without `--force`.
  - `print` is YAML by config file path.
- **Next:** M6 is done except **M6.7, which needs you** (see below). Then M7,
  the store.
- **Waiting on you:** M6.7, a manual check. Open `examples/zp/config.yaml`
  in VS Code with the YAML extension (yaml-language-server), add
  `# yaml-language-server: $schema=./config.schema.json` at the top, and
  check that keys autocomplete and that a wrong value, such as
  `database.port: 70000` or an unknown key, is flagged. A screenshot in the
  PR or a word here is enough.

## 2026-11-09: M6.3 cfgx diff

- **Landed:** `cfgx diff old.lock [new.lock]` lists what changed for
  operators, under Breaking, Behaviour changes and Safe.
  - Breaking: removed or retyped keys, removed enum values, newly required
    keys.
  - Behaviour changes: changed defaults.
  - Safe: added keys, new enum values, description edits.

  A rename kept working by an alias counts as a behaviour change.
  `-fail-breaking` exits 1, for CI.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 62k tokens
  ([review 0023](reviews/0023-gemini-m6.3-review.md)). 5 of 6 findings
  fixed, mostly around renames.
- **Next:** M6.4, the `config` subcommand, the last coding item in M6.
  M6.7 is the manual VS Code check, which is yours.
- **Waiting on you:** nothing yet.

## 2026-11-09: M6.2 gen -check

- **Landed:** `cfgx gen -check` writes nothing and fails, naming every
  stale or missing generated file. `cfgxtest.AssertGenerated(t)` reruns the
  package's own `go:generate` line with `-check`, so an app's `go test`
  catches stale code. `examples/zp` uses it.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 80k tokens
  ([review 0022](reviews/0022-gemini-m6.2-review.md)). 3 of 4 findings
  fixed, all in how `go:generate` lines are parsed.
- **Also:** M6.5's zp test assumed `/` in a cleaned path and failed on
  Windows CI. The test is fixed; cfgx's behaviour was right.
- **Next:** M6.3 (`cfgx diff`), M6.4 (`config` subcommand).
- **Waiting on you:** nothing.

## 2026-11-09: M6.1 generated files

- **Landed:** `cfgx gen` now also writes, under the module root (or
  `-artifacts dir`):
  - `config.example.yaml`: every key commented out at its default, with
    where else it can be set and which commands use it.
  - `config.schema.json` (draft 2020-12), so editors complete keys and flag
    typos.
  - `.env.example`, `docs/CONFIG.md` and `cfgx.lock.json`.

  The example config and the schema are embedded in the generated code for
  the `config` subcommand. Both examples ship the files, and the examples
  test fails if any generated file is stale.
- **Reviewed:** Gemini 3.8 Flash (High), about 97k tokens
  ([review 0021](reviews/0021-gemini-m6.1-review.md)). 7 of 10 findings
  fixed. The important one: a `Secret`'s default showed in the lock file.
- **Decided for you** (implementation.md §3.4):
  - The example config comments everything out, so `config init` pins no
    defaults.
  - "Used by" names the schema's commands (`Serve`), since cfgx doesn't know
    cobra's names.
  - The JSON Schema is stricter than cfgx about quoting numbers.
- **Next:** M6.2 (`gen --check`), M6.3 (`cfgx diff`), M6.4 (`config`
  subcommand).
- **Waiting on you:** nothing.

## 2026-11-09: M6.6 LoadX without cobra

- **Landed:** `settings.LoadMigrate(ctx, cfgx.WithEnv(env),
  cfgx.WithArgs("--steps", "3", "extra"))` loads a command's configuration
  without cobra. `WithArgs` is parsed like the command's own flags, `Args()`
  returns `["extra"]`, and `WithFile` acts as `--config`. `examples/zp` tests
  it.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 79k tokens
  ([review 0020](reviews/0020-gemini-m6.6-review.md)). No findings. A
  short brief kept this the cheapest review so far.
- **Decided for you** (implementation.md §3.4): `WithEnv` replaces the whole
  environment, `$VAR`s in search paths included. Warnings go to stderr unless
  `WithLogger` is given, as for commands.
- **Next:** M6.1 (generated files), M6.2, M6.3, M6.4.
- **Waiting on you:** nothing.

## 2026-11-09: M6.5 test helpers

- **Landed:**
  - `settings.Test(func(v *settings.Values) {...})` returns an `*All`,
    which implements every group and tag interface.
  - It holds every default, changed by plain fields such as
    `v.SMTP.Host = "localhost"`.
  - It reads no config file, env var, flag or key file.
  - `examples/zp` has the library test from interface §5.4.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 207k tokens
  ([review 0019](reviews/0019-gemini-m6.5-review.md)). No findings. I added
  a check for a case I found myself.
- **Decided by you:** keep `dsl.FromFlag`, `dsl.FromEnv` and `dsl.FromFile`,
  and `dsl.CompleteFunc`.
- **Decided for you** (implementation.md §3.4): a `Location` default loads
  its zone in `Test()` too, as `time.LoadLocation` does. Otherwise
  `t.In(loc)` would panic in tests. It's the one read `Test()` makes.
- **Next:** M6.6 (`LoadX` without cobra), then M6.1–M6.4.
- **Waiting on you:** nothing.

## 2026-11-09: M5.8 examples/zp; M5 done

- **Landed:**
  - `examples/zp`: the schema from interface §3.1, without the store and
    mutable keys (M7.9). Stub `mailer`, `server` and `db` packages take
    the generated interfaces.
  - Compile-failure tests show that `*settings.Migrate` can't be passed to
    `mailer.New`, and that `*settings.TestEmail` has `AuthEmailFrom()` but
    no other `Auth` getter.
  - The main module's tests now build, vet and test every example, and fail
    if an example's generated file is stale. `examples/mini` wasn't checked
    by CI before.
- **Reviewed:** Gemini 3.8 Flash (Medium), about 171k tokens
  ([review 0018](reviews/0018-gemini-m5.8-review.md)). 5 of 6 findings
  fixed, all in the test harness.
- **Also:** M4.10's PR failed CI on `gofmt`, an alignment change in a
  struct I added to. Fixed. I now run `gofmt -l` right before every commit.
- **Decided for you:** the example keeps the file key `emailfrom` and uses
  `.GoName("EmailFrom")`, so the getter is `AuthEmailFrom()`, as §5.1 has it.
- **Next:** M6, the developer surface (`Test()`, `LoadX`, docs, example
  files and JSON Schema).
- **Waiting on you:** the naming veto from M4.9 and M4.10 (`dsl.FromFlag`
  and the other sources, and `dsl.CompleteFunc`).

## 2026-11-09: M4.10 completion; M4 done

- **Landed:** `Complete(dsl.Files("*.pem"))`, `dsl.Dirs()` and
  `dsl.CompleteFunc(f)`. Enums complete their values automatically, and
  `--config` completes the app's file formats. A testscript runs cobra's
  `__complete` and checks the candidates.
- **M4 is done.** Every key kind and modifier in interface §3.2–3.3 that M4
  covers has golden and e2e tests. Only the store and reload modifiers
  (`Mutable`, `Epoch`, `Reloadable`) are left, for M7.
- **Reviewed:** Gemini 3.8 Flash (High)
  ([review 0017](reviews/0017-gemini-m4.10-review.md)). All 3 findings
  fixed: completion on alias flags, `Files` patterns, and `Complete` on an
  enum. This review cost about 1.04M tokens because Gemini read the repo
  despite the diff-only folder. I'll watch for that.
- **Decided for you** (implementation.md §3.4): a custom completion is
  `dsl.CompleteFunc(f)`, with `f` a top-level `func(toComplete string)
  []string`. Go can't pass a bare function where `Files` and `Dirs` values
  go.
- **Pending:** M4's milestone review on Argon High, together with the
  second review of M3, once Argon's quota resets (about 2026-11-14).
- **Next:** M5.8 (`examples/zp`, which needed M4), then M6, the developer
  surface.
- **Waiting on you:** the same naming veto as M4.9 (`dsl.FromFlag`/
  `FromEnv`/`FromFile`), plus `dsl.CompleteFunc`.

## 2026-11-09: M4.9 naming and source modifiers

- **Landed:** `Short`, `Flag`, `Env`, `Key`, `GoName`, `Sources`, `Hidden`
  and `Example`. Overrides go through the same collision checks as derived
  names, short flags included. The generator also checks the names cfgx
  reserves: `--help`, `-h`, and, when the app has a config file, `--config`
  and `<PREFIX>_CONFIG`.
- **Reviewed:** Gemini 3.8 Flash (High), about 203k tokens
  ([review 0016](reviews/0016-gemini-m4.9-review.md)). 2 of 5 findings
  fixed; 3 were already handled by code outside the diff.
- **Decided for you** (implementation.md §3.4):
  - `Flag` and `Env` replace the whole name, with no prefix added.
  - `Key` renames the key only in the config file.
  - `GoName` replaces the key's segment of the getter name, as
    `Group.GoName` does.
  - A key that `Sources` keeps out of the config file is ignored there, with
    a warning.
- **Waiting on you:** one naming call you may want to veto. The interface
  wrote `.Sources(dsl.Flag, dsl.Env, dsl.File)`, but `dsl.File` is already
  the File key constructor, so the sources are `dsl.FromFlag`,
  `dsl.FromEnv` and `dsl.FromFile`. Renaming them is a one-line change if
  you'd rather have something else.
- **Next:** M4.10 (completion), which ends M4.

## 2026-11-09: M4.8 constraints and function references

- **Landed:**
  - `Range`, `Min` and `Max` on numbers, durations and sizes; `Len` and
    `Pattern` on strings.
  - `Validate(f)` and `DefaultFunc(f)`, typed by what the getter returns, so
    a `Secret` validator takes a `cfgx.Secret`.
  - The schema rejects defaults that break the key's own constraints, and
    closures, methods, unexported or generic functions, and functions from
    the schema package or `main`.
  - One `go list` call checks every package the generated code would import
    for a key, enum and `Text` types included: none may import `cfgx/dsl`,
    and `internal/` packages must be reachable. A function in the generated
    package itself is called without an import.
  - Failures name the key and the value's source, never the value.
- **Reviewed:** Gemini 3.8 Flash (High), about 316k tokens
  ([review 0015](reviews/0015-gemini-m4.8-review.md)). 5 of 8 findings
  fixed, including directories behind symlinks (macOS `/tmp`) and enum types
  declared in the generated package. Fixing one exposed a parsing bug that
  left some packages unchecked; also fixed.
- **Decided for you** (implementation.md §3.4):
  - Constraints and `Validate` run only on values that are set. An optional
    port with `Range(1, 65535)` can stay 0; `Required` is for presence.
  - `Pattern` matches the whole value.
  - Constraint messages don't quote the value. A `Validate` function's own
    message is passed through as written.
- **Next:** M4.9 (naming and source modifiers), then M4.10 (completion).
- **Waiting on you:** nothing.

## 2026-11-09: M4.7 Text keys

- **Landed:** `dsl.Text("allow", "...", netip.Prefix{})` takes any named
  type whose pointer implements `encoding.TextUnmarshaler`, and the getter
  returns that type. The schema checks the type, and a default must write
  text that parses back to the same value.
- **Reviewed:** Gemini 3.8 Flash (High), about 299k tokens
  ([review 0014](reviews/0014-gemini-m4.7-review.md)). No findings. I found
  and fixed one edge case myself: a default that writes empty text.
- **Decided for you** (implementation.md §3.4): the third argument only
  names the type and must be its zero value; `.Default` needs
  `MarshalText`. The text reaches `UnmarshalText` exactly, untrimmed.
- **Next:** M4.8 (constraints and function references), M4.9, M4.10.
- **Waiting on you:** nothing.

## 2026-11-09: M4.6 list and map keys

- **Landed:**
  - `dsl.Strings`, `dsl.Ints`, `dsl.Durations` and `dsl.StringMap`.
  - A flag can repeat or take comma-separated items: `--tag a --tag b,c`.
    Env vars are comma-separated, and the config file takes a real list or
    map.
  - Items are parsed as CSV, so `"a,b"` keeps its comma. Malformed input is
    reported by cfgx with its source, never by pflag.
  - The layer that sets a key replaces the whole list; layers never merge.
  - Fuzzers cover the CSV splitter and the map parser.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff, about 295k tokens
  ([review 0013](reviews/0013-gemini-m4.6-review.md)). No findings.
- **Decided for you** (implementation.md §3.4):
  - An empty item is allowed in `Strings` but rejected in `Ints` and
    `Durations`, where it would quietly become 0.
  - In a `StringMap`, a later item with the same key wins, so a script can
    override `--label a=1` with a later `--label a=2`.
  - An empty list or map is nil.
- **Next:** M4.7 (`Text`), M4.8 (constraints and validators), M4.9 (naming
  modifiers), M4.10 (completion).
- **Waiting on you:** nothing.

## 2026-11-09: M4.5 path, file, URL and zone keys

- **Landed:**
  - `dsl.Path`: cleaned; a relative path in the config file is relative to
    the config file's directory, and elsewhere to the working directory.
  - `dsl.File`: like `SecretFile`, but returns `[]byte`, up to 16 MiB.
  - `dsl.URL` returns a `*url.URL`, and `dsl.Location` a `*time.Location`.
  - Defaults of these kinds are text the app parses when it loads the key,
    and only if nothing else sets it. A path default is cleaned for the OS the
    app runs on, and a zone missing on that machine is a normal config
    problem rather than a panic.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff, about 384k tokens
  ([review 0012](reviews/0012-gemini-m4.5-review.md)). Two findings, both
  password leaks in URL error messages; both fixed with tests. URL messages
  now never quote the URL.
- **Decided for you** (implementation.md §3.4):
  - URLs must be absolute and not opaque. `localhost:5432` would otherwise
    parse with `localhost` as the scheme. This also rules out `mailto:` and
    `urn:`; use a `String` key for those.
  - An unset `URL` or `Location` is nil.
  - Zones come from the system. An image built `FROM scratch` needs
    `import _ "time/tzdata"`, as the DSL docs say.
  - Getters return shared values (`[]byte`, `*url.URL`); callers mustn't
    modify them.
- **Open:** `Test()` does no I/O, but a `Location` default needs
  `LoadLocation`. To decide when `Test()` lands.
- **Next:** M4.6 (lists and maps), then M4.7–M4.10.
- **Waiting on you:** nothing.

## 2026-11-09: M4.4 secret keys

- **Landed:**
  - `dsl.Secret`: no flag; `<ENV>_FILE` may name a file instead, which loses
    one trailing newline. Setting both is an error.
  - `dsl.SecretFile`: the setting is a path, and the getter returns the
    file's exact contents. A relative path in the config file is relative to
    the config file.
  - Files are read only for keys the command loads and that are relevant, so
    TLS off with no key file on disk works. Only regular files of at most
    1 MiB are read, so a FIFO can't hang startup.
  - Messages name keys, paths and sources, never a secret. The e2e script
    checks that.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff, about 395k tokens
  ([review 0011](reviews/0011-gemini-m4.4-review.md)). Two minor findings,
  both fixed: a missing warning when `X` and `X_FILE` are both set on a key
  that doesn't matter, and Windows paths such as `\secrets\key.pem` in the
  config file.
- **Decided for you** (implementation.md §3.4):
  - An empty `X_FILE` counts as not set, so a deployment's `X_FILE` can be
    overridden locally with `X_FILE=` and `X=...`.
  - A `Secret` may have a default, and can't be a command argument. A
    `SecretFile` has a flag, since its value is a path.
- **Next:** M4.5 (`Path`, `File`, `URL`, `Location`), which reuses this
  file reading; then M4.6–M4.10.
- **Waiting on you:** nothing.

## 2026-11-09: M4.2 enum keys

- **Landed:**
  - `dsl.Enum(name, desc, values...)`: the getter returns the values' own
    type, such as zp's `models.ZpTaskExecutionMode`, so `.Default` takes only
    that type. The e2e script uses a type shaped exactly like zp's.
  - Files, env vars and flags name values by their text. A wrong value is a
    problem listing the values, with "did you mean" for a near miss such as
    `Parallel`. `--help` lists the values.
  - 14 new bad-schema cases (`enum_*`), from duplicate values to `Required`
    on an enum whose values include the zero value.
  - Generated imports: standard-library packages now go in the first group,
    and a type's package is renamed when its name is taken (`keys2`).
- **Reviewed:** Gemini 3.8 Flash (High) on the diff, about 360k tokens
  ([review 0010](reviews/0010-gemini-m4.2-review.md)). Two findings fixed
  (a repeated problem, a missing import name); one false positive.
- **Decided for you:** the Enum details in implementation.md §2.2a. Two
  change interface.md §3.2's row, which I updated:
  - Wider: an `int` type with only `String` (the usual `stringer` enum)
    works, as do plain numbers and bools, whose text is the literal.
  - Narrower: the underlying type must be a string, number or bool. A struct
    type with `MarshalText`, such as `netip.Addr`, is rejected, because the
    generated tables write each value as a Go literal. `Text` (M4.7) covers
    such types; supporting them in `Enum` too would need `UnmarshalText`.
- **Next:** M4.4 and M4.5 (secrets and file-backed kinds), M4.6 (lists and
  maps), M4.7, M4.8, M4.9 and M4.10.
- **Waiting on you:** nothing. Say if you want struct enums despite the
  above.

## 2026-11-09: M4.1 numeric and duration keys

- **Landed:**
  - New key types: `Int64`, `Uint`, `Float`, `Duration` (a unit is required)
    and `ByteSize` (`512KB`, `1MiB`).
  - Defaults are checked with the same parser as file values, and written
    canonically: `5m0s` in `--help`, `5 * time.Minute` in the code.
  - The DSL's key types now share one generic base, so each new kind takes a
    few lines, and each key type still offers only its own modifiers.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff, about 190k tokens
  ([review 0009](reviews/0009-gemini-m4.1-review.md)). One finding was fixed
  (`+1` for a `Uint`); four were false positives, two of them disproved by
  running Go.
- **Decided for you:** the kind details in implementation.md §2.2a. Among
  them: fractional sizes such as `1.5GB` are rejected in v0.1.0, and NaN and
  infinities are rejected.
- **Next:** M4.2 (`Enum`), then M4.4 and M4.5 (secrets and file-backed kinds),
  M4.6 (lists and maps), M4.7, M4.8 (constraints and validators), M4.9 and
  M4.10. With the kinds table in place, M4.5–M4.7 suit Gemini on a cheap
  model.
- **Waiting on you:** nothing.

## 2026-11-09: M5.7 generate-time checks

- **Landed:**
  - Every check the generator makes has a bad schema: 63 of them, run
    through the real DSL, loader and generator. Each one asserts the message
    and the schema line it points at.
  - The generator can now warn without failing. The first warning: a
    command that lists a `Global` group in `Uses`.
  - Writing the cases found three problems that were reported twice. All
    three are fixed.
- **Reviewed:** Gemini 3.8 Flash (Medium) on the diff, about 50k tokens
  ([review 0008](reviews/0008-gemini-m5.7-review.md)). Two harness
  improvements came out of it; two findings were rejected.
- **Cost:** this was the cheapest review so far, from a Medium tier and a
  small diff. Writing the 63 cases myself with a script was cheaper than
  briefing Gemini.
- **Next:** M5 is done except M5.8, which needs M4. M4 is the key vocabulary:
  durations, enums, file kinds (secret files, paths, URLs), lists, maps,
  constraints, and naming modifiers.
- **Waiting on you:** nothing.

## 2026-11-09: M5.6 computed methods

- **Landed:** `Group.Computed("DSN")` adds a hand-written method to the
  group's interface (A8):
  - `cfgx gen` type-checks the settings package with an in-memory stub and
    renders each method's exact signature, such as `RetryPolicy()
    (time.Duration, error)`, with the imports it needs (`/v2` paths and
    renamed imports included).
  - A command that reaches the group only through a tag doesn't get the
    method. An end-to-end test checks this with a compile failure.
  - `examples/mini` now declares cfgx as a `tool`, which the roadmap always
    said it did. Without it, `go mod tidy` drops the generator's new
    dependency, `golang.org/x/tools`.
- **Reviewed:** Gemini 3.8 Flash (High) on the diff (about 210k tokens,
  [review 0007](reviews/0007-gemini-m5.6-review.md)). Two fixes came out of
  it: a nested group's computed method could clash with a parent's getter, and
  the overlay path is now always absolute. The other 4 findings were rejected.
  The "read-only" review left a compiled binary in the repo; Claude deleted it,
  and reviews now run from a folder holding only the diff.
- **Decided for you:** the details in implementation.md §2.3:
  - Promoted methods are rejected; declare the method on the group itself.
  - Imports get a number added when their name is taken.
  - Unrelated type errors don't block generation.
- **Next:** M5.7 (the full list of generate-time checks); then M4, the key
  vocabulary. M5.8 needs M4.
- **Waiting on you:** nothing.

## 2026-11-09: M5.4 and M5.5, by Gemini

- **Landed:** through a PR:
  - **M5.5, command `Args`:** flag-only arguments with getters on the command
    type, such as `m.Down()` and `m.Steps()`.
  - **M5.4, `Alias` and `Deprecated`:** an old file key, env var or flag keeps
    working and warns; a deprecated key warns when it is set.
- **Reviewed:** Gemini 3.8 Flash (High) wrote both items, and Claude reviewed
  them. Claude added two checks to M5.5: an argument's flag can't clash with a
  `RelevantIf` condition's flag, and arguments can't have tags. Since Claude
  reviewed Gemini's work, no second-model review was needed.
- **Cost:** about 570k tokens for M5.5 and 510k for M5.4, mostly Gemini
  reading the code and re-running tests. That's high for small items, even on
  Flash. Next time, for items this size, a brief that hands over the exact
  code skeleton and needs less exploration should cost much less. Doing them
  myself is an option too.
- **Decided for you:** the details in implementation.md §2.3c. Among them:
  - Within a layer, the new name wins over an alias.
  - An alias that is used always warns.
  - Argument getters carry no group prefix.
- **Next:** M5.6 (computed methods, a type-checking pass) and M5.7 (the full
  check list), both Claude's; then M4.
- **Waiting on you:** nothing.

## 2026-11-08: M5.3 rules

- **Landed:**
  - `AtMostOne`, `ExactlyOne`, `AllOrNone` and `Requires` are checked after
    merging. They are value-based (A4): a key counts as set when it's non-zero,
    and a Bool only when it's true.
  - `RelevantIf` decides what loads. A key whose condition isn't set keeps its
    default, isn't parsed or checked, and warns if it was set to a value.
  - Rule problems follow interface §8:
    `auth: token and keys are mutually exclusive (token: env ZP_AUTH_TOKEN,
    keys: app.yaml:2:9)`.
  - The PR merges itself once CI passes.
- **Reviewed:** Gemini 3.8 Flash (High) on the non-test diff (about 90k tokens;
  [review 0006](reviews/0006-gemini-m5.3-review.md)). Three findings fixed: a
  command was held to `Required` on a condition it doesn't own; a `RelevantIf`
  cycle was blamed on the wrong key; and an empty value on an irrelevant key
  warned. One finding is deferred to M4.1, and two were false positives.
- **Decided for you:** the rule details in implementation.md §2.3b:
  - A command checks a rule only when it loads every key in it.
  - The conditions of a command's `RelevantIf` keys are loaded for that
    purpose only.
  - Under A4, `Required` together with a non-zero default is useful, so it
    stays allowed.
  - The "no key file on disk" check moves to M4.4, which adds file kinds.
- **Next:** M5.4 (`Alias`, `Deprecated`) and M5.5 (command `Args`), both
  delegated to Gemini, then M5.6 and M5.7.
- **Waiting on you:** nothing.

## 2026-11-08: M4.3, M5.1 and M5.2

- **Landed:** through a PR:
  - `cfgx.Secret` is in package `cfgx` (ADR 0006), and its leak suite now runs
    in CI.
  - Nested groups produce embedded interfaces (`Scheduler` embeds
    `SchedulerRetry`).
  - Dotted key names nest the file key without adding an interface.
  - `Group.GoName`, `Group.FlagPrefix` and `App.Initialisms` work.
  - Tags work: `dsl.Tag`, `.Tags` on keys and groups, and `Uses(tag)`. A
    command that uses only `Mailer` gets exactly the tagged getters (A5).
- **Reviewed:** Gemini 3.8 Flash (High) reviewed the diff for about 150k
  tokens, a third of M3's review. 3 of its 5 findings were real and are fixed
  ([review 0005](reviews/0005-gemini-m4.3-m5.2-review.md)): a nil key
  panicked instead of being reported, `GoName("DB1")` gave `dB1Group`, and an
  undeclared tag was ignored. The other 2 were false positives, already
  covered by tests.
- **Decided for you:** the structure rules in implementation.md §2.3a. Among
  them: a group has one place in the tree; nested groups inherit their
  parent's flag prefix; a tag on no key is an error. Gemini did no
  implementation this round, because M4.3 was a move and M5.1 and M5.2 touch
  the same files; briefing it would have cost more than doing the work.
- **Noticed:** interface §3.1's key `emailfrom` derives `AuthEmailfrom()`,
  while §5.1 shows `AuthEmailFrom()`. M5.8's example needs `email_from` or a
  key `GoName` (M4.9); noted in the roadmap.
- **Changed with your approval:** the repo now allows auto-merge and deletes
  merged branches, and `main` requires the six CI jobs (admins exempt, for
  docs-only commits). From now on each PR gets auto-merge, with merge commits,
  once the cross-model review is done, so a green PR lands without waiting
  for Claude to be prompted.
- **Next:** M5.3 (rules), M5.4 and M5.5 (to Gemini, Flash or Argon Medium),
  M5.6 (computed methods) and M5.7 (checks); the Argon High second review of
  M3 once its quota resets.
- **Waiting on you:** nothing. You deleted the old merged branches; GitHub
  deletes branches after each merge from now on.

## 2026-11-08: M3 config files, errors and provenance

- **Landed:** M3 through a PR. Config files are the lowest explicit layer
  (default < file < env < flag), found as A11 describes, in YAML or JSON
  through one parser. Every problem is reported at once with where it came
  from, in interface §8's format. Unknown keys in the file and unknown `ZP_*`
  env vars are warnings with "did you mean". Warnings go through `slog`, and
  `cfgx.WithLogger` takes them. `Required` is checked only for the running
  command's keys, and `Global` groups become persistent flags.
  `examples/mini` now reads `mini.yaml`.
- **Reviewed:** Claude reviewed Gemini's parser (M3.1) and suggestion package
  (M3.3) and took both as written. Gemini reviewed Claude's code and found 8
  problems; 5 led to fixes ([review 0004](reviews/0004-gemini-m3-review.md)).
  The biggest: an empty `--config` let `ZP_CONFIG` win over a flag, and the
  "required" message could have quoted a secret.
- **Decided for you:** the details in implementation.md §3.4, "[decided, M3]".
  Among them: an app without `ConfigFile` has no `--config` flag; a null value
  in the file counts as absent; `BindRoot` must be called when the app has a
  config file or global groups. Models: Argon High built the parser and Flash
  High the suggestions. Argon (High and Medium) then ran out of quota for about
  six days, so Flash High ran this review instead.
- **Next:** an Argon High second review of M3 once its quota resets; meanwhile
  M4.3 (the `Secret` type) and M5 in lane B, M4.4–M4.6 in lane A.
- **Waiting on you:** nothing.

## 2026-11-08: amendment A12, positional arguments

- **Landed:** you chose the `Args()` method. Every generated command type now
  has `Args() []string`, returning a copy of the arguments cobra parsed; rules
  such as `cobra.ExactArgs(1)` work as before. Recorded as A12 in interface §12,
  with an end-to-end script.
- **Reviewed:** Gemini found five problems in the change, all fixed. The one
  that mattered: a method on a command type would silently shadow a group getter
  with the same name, so `Args` and the method names M7 will add are now
  reserved, checked at generate time.
- **Next:** M3, config files, errors and provenance.
- **Waiting on you:** nothing.

## 2026-11-08: M2 walking skeleton done

- **Landed:** M2 through a PR. The first real code: IR, DSL, engine, generator,
  loader and `cfgx gen`, plus `examples/mini` and end-to-end scripts. A schema
  becomes generated code and a running binary, with default < env < flag.
- **Reviewed:** Claude reviewed Gemini's M2.6 work and fixed the harness's
  hard-coded dependency versions. Gemini found 13 problems in Claude's code, all
  fixed ([review 0003](reviews/0003-gemini-m2-review.md)). The biggest: a bad
  env var failed startup even when a flag overrode it, and a schema without
  commands generated code that didn't compile.
- **Decided for you:** the temporary `main` now lives in the working directory
  and is built, then run (ADR 0002, revision). A warm `cfgx gen` takes about
  0.9 s. Gemini models: Argon High for this milestone's delegation and reviews.
- **Next:** M3, config files, errors and provenance; in parallel, M4.3 (the
  `Secret` type) and M5.
- **Waiting on you:** one API decision: how a command's run function gets its
  positional arguments. A recommendation is in the conversation.

## 2026-11-08: full ownership

- **Changed:** you gave Claude full ownership. Only API design decisions and
  things Claude lacks permission for come back to you; releases, the `cfgx init`
  skeleton and how Gemini is used are now Claude's calls. ownership.md and the
  roadmap say so.

## 2026-11-08: M1 spikes done

- **Landed:** ADRs 0002–0006 on branch `m1-spikes`, merged through a PR. Gemini
  ran S4 and S5; Claude ran S1–S3, reviewed Gemini's work, and re-checked the
  claims the decisions rest on.
- **Reviewed:** Gemini reviewed Claude's ADRs and found 12 problems, all fixed
  before merge ([review 0002](reviews/0002-gemini-m1-review.md)). The biggest:
  pflag could reject a slice flag before cfgx saw it, and the planned JSON
  surrogate rewrite would have corrupted escaped backslashes.
- **Decided for you:**
  - The generator pipeline stays. Its temporary `main` runs through
    `go run -overlay`, so `cfgx gen` never writes to your tree (ADR 0002).
  - Flags use raw custom values, and cfgx won't raise pflag to v1.1.0, which
    garbles cobra's help flag (ADR 0003).
  - Validators are referenced through `FuncForPC`, with strict rules (ADR 0004).
  - YAML and JSON both go through `go.yaml.in/yaml/v4`. Anchors, aliases and
    merge keys are rejected in v0.1.0 (now a backlog item), and JSON surrogate
    pairs are rewritten so valid JSON never fails (ADR 0005).
  - `Secret` uses a closure: one mask for set and unset, no unmarshalers, and
    `Equal` on SHA-256 digests (ADR 0006).
- **Next:** M2, the walking skeleton.
- **Waiting on you:** how often work should happen (unchanged); optionally,
  deleting merged branches.

## 2026-11-07: ownership starts, M0 merged

- **Landed:** M0 is merged into `main` (fast-forward to `237b78c`). CI passed on
  all six jobs, and the e2e script ran on every OS.
- **Decided for you:** the terms in ownership.md. From M1 on, code lands
  through PRs that Claude merges once CI is green and the cross-model review is
  handled.
- **Next:** M1, the five spikes. agy takes S4 (YAML library) and S5 (secret
  leaks); Claude takes S1 (generator pipeline), S2 (flags) and S3 (function
  references).
- **Waiting on you:**
  - How often work should happen: when you ask, or on a schedule.
  - Optional: delete the merged `m0-foundation` branch. The safety classifier
    blocked Claude from deleting it.
