# Status log

Newest first. Every working session adds an entry: what landed, the decisions
taken for you, what's next, and what's waiting on you. The terms are in
[ownership.md](ownership.md).

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
