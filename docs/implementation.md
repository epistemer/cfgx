# cfgx — Implementation brainstorm

> Status: **brainstorm.** This covers how to build what
> [interface.md](interface.md) describes. Decisions are marked **[decided]**,
> recommendations **[proposed]**, and unknowns that need a spike **[spike]**.
> The resulting plan is in [roadmap.md](roadmap.md).

---

## 1. Big picture

```
 schema/schema.go ──(1) load──▶ temp main ──(2) go run──▶ IR (JSON)
                                                          │
            settings/*.go (hand-written) ─(3) type check─▶│
                                                          ▼
                                                 (4) check & resolve
                                                          │
             ┌───────────────┬──────────────┬─────────────┼──────────────┐
             ▼               ▼              ▼             ▼              ▼
     zz_settings.go   config.example.yaml  .env.example  config.schema.json
     (tables + types)   docs/CONFIG.md     cfgx.lock.json
             │
             ▼   at runtime
       cfgx engine  ◀── flags (pflag) · env · file (node tree) · defaults · [store]
```

There are two halves, connected by one contract, the **IR**:

- **Generate time:** the `dsl` package records declarations, and `cmd/cfgx` turns
  them into code and artifacts.
- **Runtime:** the generated code consists of **tables** plus thin typed
  wrappers. One generic **engine** in package `cfgx` does all the work.

---

## 2. Generate time

### 2.1 Loading the schema **[proposed]**

1. `dsl.App`, `dsl.Command`, `dsl.Group` and `dsl.Tag` register themselves in
   package `dsl` when the schema package initializes. The generator doesn't need
   the schema's variable names, so it doesn't need `go/packages` to find them.
2. The generator builds a tiny `main` package that blank-imports the schema and
   prints the IR:

   ```go
   package main
   import ("os"; "github.com/epistemer/cfgx/dsl"; _ "zp/internal/settings/schema")
   func main() { dsl.Export(os.Stdout) }
   ```

3. It runs that program with `go run`, inheriting the user's `GOFLAGS`,
   `go.work` and vendor mode, and reads the IR from stdout.
4. If the schema panics or doesn't compile, the generator reports the error at
   the user's `schema.go` location.

**Placement [decided, ADR 0002, revised in M2.3]:** the temporary `main` sits
at a virtual path in the working directory (`zz_cfgx_gen_main/main.go`, which
must not exist on disk), inside the caller's own module. It's built with
`go build -overlay` and the binary is run directly. Nothing is written to the
working tree, `--check` included. A warm `cfgx gen` takes about 0.9 s on the
walking skeleton, and about 10 s on an empty cache. Module, workspace and
vendor modes and `go generate` all work.

The temporary `main` writes the IR to a file the generator names rather than to
stdout, so output from an `init` can't corrupt it. The generator resolves its
argument with `go list -e`, so a broken schema reaches the user as a compile
error from `go run`. It also merges any `-overlay` already in `GOFLAGS`,
canonicalizes paths with `filepath.EvalSymlinks`, and runs the program with a
timeout. On Unix the timeout dumps goroutine stacks (SIGQUIT), which show the
`init` that hung (ADR 0002).

### 2.2 What `dsl` records

Each constructor captures:
- `runtime.Caller` (file:line), so every generate-time error points at the
  user's schema line
- the typed value, as Go data

`dsl.Export` serializes everything to the IR:

| Concept | How it's captured |
|---------|-------------------|
| Key type | `reflect.Type` → import path + name (`models.ZpTaskExecutionMode`, `time.Duration`) |
| Default | literal (int, bool, float), canonical text (Duration, Enum, `Text` types via `MarshalText`), or a function reference |
| `Validate(f)`, `DefaultFunc(f)`, `Complete(f)` | `runtime.FuncForPC(...).Name()` → package path + function name, under the rules below |
| Enum values | canonical text of each value + Go type |
| Rules | key identities (pointer identity of the `*KeyDef` → stable path) |
| Tags, groups, commands | identities resolved to names |

**Function references [decided, ADR 0004].** A function is accepted only if:
- it's top-level and exported: after the package path, its symbol is a single
  identifier (`token.IsIdentifier` and `token.IsExported`), with no further dot. That rejects closures (`F.func1`), method values (`T.M-fm`) and
  method expressions (`T.M`, `(*T).M`)
- it isn't a generic instance (no `[` in the name)
- its package isn't `main` or the schema package, and doesn't import `cfgx/dsl`,
  because generated code must never import them. One batched `go list -deps`
  call checks every referenced package
- the generated package can import it under Go's `internal/` rule. A function in
  the generated package itself is referenced without an import

The package path ends at the first `.` after the last `/`. Go escapes dots in the last path element of symbol names as `%2e`
(`gopkg.in/yaml%2ev3.F`), which keeps that split unambiguous; the generator
unescapes them. Rejections point at the schema line.

Confirmed on Go 1.26.0 and 1.27.3 (ADR 0004). Closure names depend on inlining
and can even move into the caller's package, which is why the rule rejects any
further dot instead of matching closure patterns. Generated code imports every
referenced package under a generated alias, because a package's name can differ
from its last path element.

### 2.2a Key kinds **[decided, M4.1]**

- Every DSL key type embeds one generic base, `key[K, T]`, whose modifiers
  return the concrete type `K`, so `dsl.Duration(...).Default(...).Required()`
  chains and each key type can add only the modifiers that fit it. Schemas
  never name the generic type (interface §3.1). Adding a kind is a type, a
  constructor and a text encoder in `dsl`, a parser in `engine`, and a row in
  the generator's kinds table (`internal/gen/kinds.go`).
- The generator checks a default with the engine's own parser, so a default
  is accepted exactly when the same text in a file would be, and writes it
  canonically: `5m0s` in `--help` and `5 * time.Minute` in the code; sizes
  as `1MiB` in `--help`.
- `Duration` rejects a bare number other than 0 (`"30" has no unit`).
  Negative durations parse; constraints (M4.8) can forbid them.
- `ByteSize` takes bytes or a whole number with a unit, case-insensitive,
  optionally after a space: B, KB/MB/GB/TB/PB (1000s), KiB/MiB/GiB/TiB/PiB
  (1024s). Fractions such as `1.5GB` are rejected in v0.1.0.
- `Float` rejects NaN and infinities.

**Enum [decided, M4.2].**

- `dsl.Enum(name, desc, values ...T)` is generic over `T comparable`; Go
  infers `T` from the values, so `.Default` takes only a `T`, and the getter
  returns `T`. `T`'s underlying type must be a string, integer, float or
  bool type, so every value has a Go literal and the generated tables need
  no `UnmarshalText`.
- A value's text is, in order: `MarshalText` (value or pointer receiver);
  for a string type, the string itself; `String`; else the literal (`7`,
  `true`). A string type's own value beats its `String`, because types like
  sqlc's store the canonical text in the value.
- The schema run records problems only `reflect` can see (no values, a value
  listed twice, two values with one text, an empty text, text with outer
  spaces, a `MarshalText` error, NaN, an unexported or generic type, a
  default that isn't a value) in `ir.Key.Problems`; the generator reports
  them at the key's line with every other problem. The generator adds: the
  type's package is the schema package; `Required` on an enum whose values
  include the type's zero value, which counts as not set (A4).
- The generated file has two tables per enum key, the texts and the values
  (`[]T{...}` of bare literals), which the key's `Enum` field and
  `engine.ParseEnum` share. Matching is exact after trimming spaces; a near
  miss, including a case difference, gets "did you mean". `--help` appends
  "; one of a, b, c" and shows the default unquoted.
- A type's package is imported under its own name unless the generated file
  already uses that name (`keys`, `app`, `v`, `cobra`, ...), then with a
  number added (`keys2`). Standard-library imports go in the first group.
- Not checked yet: an enum type in a package that imports `cfgx/dsl`, or in
  the generated package itself; M4.8's `go list -deps` check covers the
  first, and the second fails to compile with an import cycle.

### 2.3 Computed methods: a type-checking pass **[decided, M5.6]**

`Group.Computed("DSN")` declares that a hand-written method belongs to the
group's interface. Its signature has to be exact, so the generator type-checks
rather than copying source text (A8):
1. Build `zz_settings.go` with the computed methods left out of the interfaces,
   in memory.
2. Type-check the package with `go/packages` (`NeedTypes`), passing that stub
   through `packages.Config.Overlay`, so nothing is written yet (review 0002,
   F06).
3. Find each declared method on the group's receiver type (naming rule in
   interface §5.3) and render its signature with `types.TypeString` and an
   import-tracking qualifier, so package names, `/v2` paths and aliases come
   out right.
4. Emit the final file.

Any method without parameters is allowed, e.g. `DSN() string` or
`RetryPolicy() (types.RetryPolicy, error)`. A missing method, a wrong receiver
or parameters are generate-time errors.

Settled while building it:
- The pass runs only when a group declares computed methods, through
  `golang.org/x/tools/go/packages` (v0.52.0), which `cmd/cfgx` needs but the
  runtime packages don't import.
- A method must be declared on the group's own type. One promoted from a
  nested group is rejected: list it on the nested group instead.
- Packages named in signatures are imported under their package name, or
  with a number added (`rand2`) when that name is taken in the settings
  package or by the file's other imports. Named results keep their names.
- Type errors elsewhere in the settings package don't stop generation (the
  stub may not satisfy code that uses computed methods yet); they are shown
  only when a computed method can't be resolved, as a likely cause.
- A computed method can't share a name with another method in its group's
  interface, nested groups included, or with a reserved method. Two groups
  that a command embeds can't both have it, because Go wouldn't promote it.

### 2.3a Structure **[decided, M5.1–M5.2]**

- A group has one place in the tree: nesting the same `*GroupDef` twice, or
  nesting a group that is also passed to `dsl.App`, is an error.
- A nested group's names extend its parent's: interface `SchedulerRetry`,
  type `schedulerRetryGroup`, getters `SchedulerRetryMax()`. Its flag prefix
  starts from the parent's, so under `FlagPrefix("")` a nested `retry` gives
  `--retry-max`. Env vars and file keys always use the full path.
- A group's interface lists its own getters, then embeds its children's
  interfaces; the group type embeds the children's types, so their getters
  and computed methods are promoted.
- A command that uses a group and one of its descendants embeds the group
  once; the descendant comes through it.
- A key can't sit where the file needs a map: `tls` and `tls.enabled` can't
  both be keys, and a dotted key can't share its path with a nested group's
  key. Both are generate-time errors.
- A tag's interface lists its keys' getters in key-table order. A tag that is
  on no key is an error, since its empty interface would accept anything.
- A command that uses a tag gets forwarding getters for the tagged keys its
  embedded groups don't already provide (A5), so it can embed `smtp` whole and
  forward only `AuthEmailfrom()`.

### 2.3b Rules **[decided, M5.3]**

- A command checks a rule only if it loads every key in it. A rule over a key
  that failed to load is skipped, because that key is already a problem.
- A command that loads a `RelevantIf` key also loads its conditions, with
  their flags but no getters, so it can tell whether the key matters. Those
  condition-only keys aren't held to `Required` and take no part in rules:
  they aren't the command's own keys.
- A key whose condition isn't set keeps its default and isn't parsed or
  checked (`Required` included). It counts as unset in other rules. If a
  layer set it to a value that counts as set (A4: not empty, not false),
  startup warns: `server.tls.cert is set (flag
  --tls-cert) but ignored: it only matters when server.tls.enabled is set`. No
  warning is printed when the condition itself failed to load.
- Rule problems are reported under the declaring group's path, naming keys
  relative to it and listing the source of each key that is set:
  `auth: token and keys are mutually exclusive (token: env ZP_AUTH_TOKEN,
  keys: config.yaml:22:9)`. The other messages are `at most one of a, b and c
  may be set`, `one of a and b must be set`, `only one of a and b may be set`,
  `a, b and c must be set together or not at all; a and c aren't set`, and
  `a requires b`.
- Generate-time checks: exclusive rules need two or more distinct keys and no
  non-zero defaults (A4); `Requires` and `RelevantIf` take exactly two; a
  `RelevantIf` cycle is an error.

### 2.3c Arguments, aliases, deprecation **[decided, M5.4–M5.5]**

- Command `Args` are flag-only: never read from env or the config file, not
  file keys, so the unknown-key and unknown-env scans ignore them. The getter
  is the argument's name alone (`Steps()`); its values field is prefixed with
  the command (`MigrateSteps`), so two commands can share an argument name.
  An argument's flag can't clash with `--help`, `--config` or any flag the
  command already has (globals and `RelevantIf` conditions included), and an
  argument can't have tags or aliases.
- `.Alias("database.ssl_mode")` derives the old env var and flag from the old
  path, as for any key. The old flag is hidden from `--help`. In each layer
  the new name wins over an alias. Using an alias warns (`database.ssl_mode
  is deprecated, use database.sslmode (config.yaml:9:3)`, `ZP_DATABASE_SSL_MODE
  is deprecated, use ZP_DATABASE_SSLMODE`, `--database-ssl-mode is deprecated,
  use --database-sslmode`), even when a higher layer overrides it; an alias
  set next to the new name warns that it is ignored.
- `.Deprecated(msg)` warns `database.legacy is deprecated: <msg> (<source>)`
  when the key is set to a value that counts as set (A4), and not when the key
  is irrelevant.

### 2.4 Checks **[decided by interface; M5.7 status]**

Every check the generator makes today has a bad schema in
`cmd/cfgx/testdata/checks/<name>/schema.go`, run through the real DSL and
loader; a `// want "..."` comment marks the line each problem must point at
(`TestSchemaChecks`). Checks the DSL can't trigger (an IR naming a group or
tag that doesn't exist, an unknown rule kind) are covered by
`internal/gen`'s `TestChecks` instead. Checks on things later milestones add
(defaults against constraints, function references, mutable secrets, the
store key, short flags) get their bad schemas with those features.

The generator also has warnings, printed as `cfgx: warning: <pos>: <msg>`
without failing: today, only a command that lists a `Global` group in `Uses`.
A problem isn't reported twice: an invalid tag name doesn't also make its
keys' tag "undeclared", an app without a name doesn't also report its derived
env prefix, and an argument whose getter clashes isn't reported again by the
computed-method check.


These all run on the IR before anything is written:
- name collisions on every surface: file key, env, flag, short flag, getter,
  interface name, tag vs group name
- an empty description
- a default outside its constraints (range, enum, pattern)
- a `Global` group that is also listed explicitly in a command's `Uses`
  (warning)
- `Args` colliding with group flags
- keys in `AtMostOne`, `ExactlyOne` or `AllOrNone` with a non-zero default
  (rules are value-based, A4)
- a `Validate`, `DefaultFunc` or `Complete` function that isn't an accepted
  reference (§2.2)
- `.Mutable()` or `.Epoch()` on a `Secret` or `SecretFile` key (A10)
- the `App.Store` key must be a static `Path` key, and no mutable key may
  configure the store

### 2.5 Emitting

- **[proposed]** Use `text/template`, `go/format` and
  `golang.org/x/tools/imports`. A code-builder library (jennifer) isn't worth the
  dependency for this amount of code.
- Output must be deterministic: sorted, stable names, no timestamps. Both
  `--check` and `cfgx.lock.json` diffs depend on it.
- `cfgx.lock.json` is the IR minus source positions. `cfgx diff` compares two
  lock files.
- The JSON Schema targets draft 2020-12, for yaml-language-server and VS Code.
- `zz_settings.go` embeds what the binary needs at run time: the example config
  and the JSON Schema as string constants (for `config init` and
  `config schema`), and per-key metadata (description, type, default text, enum
  values, tags, JSON Schema fragment, mutable, epoch) for `explain` and
  `Store.Keys()`.

---

## 3. Runtime

### 3.1 Generated code is table-driven **[proposed]**

The alternative, fully unrolled code per key, produces enormous files and
duplicates logic that can only be tested through generated output. Instead, the
generator emits **data and tiny closures**, and the engine is tested once:

```go
// zz_settings.go (sketch; generics here are internal, never user-facing)
type values struct {
	DBHost string
	DBPort int
	DBPassword cfgx.Secret
	SchedulerExecMode models.ZpTaskExecutionMode
	// ...
}

var keys = [...]engine.Key[values]{
	{
		Path: "database.port", Env: "ZP_DATABASE_PORT", Flag: "database-port",
		Type: "int", Desc: "Database server port", DefaultText: "5432",
		Schema:  `{"type":"integer","minimum":1,"maximum":65535}`,
		Set:     func(v *values, r engine.Raw) error { return engine.Int(r, &v.DBPort) },
		Format:  func(v *values) string { return strconv.Itoa(v.DBPort) },
		Default: func(v *values) { v.DBPort = 5432 },
		Check:   func(v *values) error { return engine.Range(v.DBPort, 1, 65535) },
		IsSet:   func(v *values) bool { return v.DBPort != 0 },
		// also: Enum, Tags, Mutable, Epoch, RelevantIf condition
	},
	// ...
}

var serveKeys = []int{0, 1, 2, 5, 9 /* ... */} // which keys the Serve command loads
```

Getters are the only per-key code in the generated output:

```go
func (g *dbGroup) DBPort() int { return g.r.cur.Load().DBPort }
```

A command type embeds the types of the groups it uses whole, so their getters
and computed methods are promoted. For a group it reaches only through a tag,
the generator writes forwarding getters for just the tagged keys on the command
type, and no computed methods. Either way, a command type has getters only for
keys it loads (A5).

### 3.2 Values and snapshots **[proposed]**

- One `values` struct per app. **Every getter reads through an
  `atomic.Pointer[values]`**, whether the key is static, reloadable or mutable.
  - Cost: one atomic load per getter call, which is negligible.
  - Gain: one code path. A reload or store update builds a new `values`,
    validates it as a whole, and swaps the pointer. That gives the
    all-or-nothing rule in interface §7.2 for free.
- Command types embed the types of the groups they use whole (§3.1); all of
  them share the same root pointer. `Snapshot()` gives a consistent read across
  several keys.
- The engine also keeps the **bootstrap snapshot**: values and provenance from
  default, file, env and flag, before the store. Store writes compute the new
  snapshot as bootstrap plus stored records, which is how `ResetX` brings the
  bootstrap value back.

### 3.3 Loading pipeline (per command)

```
keys := the command's keys + Global keys + the store key (if it has the store)
for k in keys:
    apply default
    for layer in [file, env, flag]:          # lowest to highest
        if layer has k: Set(v, raw); append provenance(k, layer, where)
bootstrap := v                                # kept for Reset and explain
if the command has the store: open it; layer stored records over mutable keys
skip keys whose RelevantIf condition isn't set (no file reads, no checks)
read SecretFile / File contents for the remaining keys
collect parse errors
per-key Check()  →  Validate funcs  →  Required  →  Rules
if errors: return engine.Errors (all problems, sorted by key)
publish snapshot
```

**Rules are value-based [decided, A4].** A key is *set* when its effective
value is non-zero; a Bool is set when it's true. Each key's `IsSet` closure
answers that from the merged value, so provenance plays no part. An env var
that exists but is empty provides the empty value, which is how an operator
clears a file value. `Required` uses the same test.

Loading per command fixes a real bug in the old `zp` design: `migrate` must not
fail because the JWT key files are missing. Required checks, `SecretFile` reads
and validation only run for the command's own keys. Unknown-key detection still
uses the whole app's key set, so another command's keys are never reported as
unknown.

### 3.4 Sources

**Flags.**
- The engine registers a custom `pflag.Value` per key that stores raw text. Its
  `Type()` gives the help label (`int`, `duration`, `strings`, …). cfgx writes
  `(default …)` into the usage text itself and leaves `DefValue` empty, because
  pflag decides whether an unknown type's default is "zero" from its text and
  would hide a string default of `0` or `false` (ADR 0003).
- A flag counts **only if `Changed`**. This avoids viper's classic bug, where a
  flag's default overrides the env var and the file.
- Bool flags set `NoOptDefVal = "true"`.
- Slice and map keys record each occurrence's raw text, because pflag calls
  `Set` once per `--tag a --tag b`. The engine splits and validates them, so
  pflag never rejects a value (ADR 0003). An explicitly empty occurrence provides
  an empty list.
- `Global` keys are registered as persistent flags on the root by `BindRoot`.
- **[decided, ADR 0003]** Confirmed on cobra v1.10.2: persistent flags are
  visible through the child's `Flags()` in `RunE`, completion works with the
  custom values, and `--help` shows defaults without applying them. cfgx doesn't
  raise pflag beyond what cobra requires, because pflag v1.1.0 renders cobra's
  own help flag as `--help[=true|false]`.

**Env.**
- `os.LookupEnv` is called only for known names, plus the `_FILE` variants for
  secrets.
- At the end, the engine scans `os.Environ()` for unknown `ZP_*` names and warns
  about likely typos.
- A variable that exists but is empty provides the empty value.
- Secrets read from `_FILE` lose one trailing `\n` or `\r\n`. `SecretFile`
  keeps the bytes exactly. Setting both `ZP_X` and `ZP_X_FILE` is an error.
- On Windows, env names are case-insensitive, so the unknown-name scan compares
  upper-cased names.

**File.**
- The file is parsed into an internal node tree `{Kind, Scalar, Map, List, Pos}`,
  and keys are matched case-sensitively.
- Unknown keys get Levenshtein "did you mean" suggestions.
- Relative paths in `Path`, `SecretFile` and `File` values from a config file
  are resolved against that file's directory (A11).
- File-backed values (`SecretFile`, `File`, `_FILE`) must be regular files, so a
  FIFO or device can't hang startup, and are capped at 1 MiB for secrets and
  16 MiB for `File`.
- **[decided, M4.4]** Secrets and file-backed values:
  - The engine reads files only after a key's winning layer is known and the
    key is relevant, so a key a command doesn't load, or one whose
    `RelevantIf` condition is off, is never read (TLS off with no key file on
    disk works). Deciding whether a path "sets" a key, for warnings, doesn't
    read it either: a non-empty path counts as set.
  - A `SecretFile` default is a path, read like any other; a missing default
    file is a problem only for commands that load the key while it is
    relevant.
  - Files are checked with `os.Stat` before opening (opening a FIFO blocks),
    then read through a limit of max+1 bytes, since a file can grow after
    `Stat`. Messages name the path and the source, never the contents.
  - An empty `<ENV>_FILE` names no file, so it counts as not set: a
    deployment's `_FILE` can be overridden with `ZP_X_FILE=` and `ZP_X=...`.
    Aliases get `_FILE` variants too, and every `_FILE` name is a known env
    var for the unknown-name scan. The generator checks `_FILE` names for
    collisions with other keys' env vars.
  - A `Secret` may have a default (it goes into the generated code); a
    `Secret` can't be a command argument, which is flag-only. A `SecretFile`
    has a flag, since its value is a path, and can be an argument.
- **[decided, M4.5]** Paths, files, URLs and zones:
  - `Path`, `File`, `URL` and `Location` defaults aren't Go literals in the
    generated code. The generator checks the text, and the engine parses it
    when loading, only if no other layer sets the key (`Key.Default` is nil
    and `DefaultText` holds the text). So a path is cleaned for the OS the app
    runs on, not the one that generated the file, and a zone missing on the
    machine that runs the app is a problem with source `default`, not a
    panic. A key that isn't relevant still gets such a default if it parses;
    a `File` key's default path isn't read.
  - `Path` is `filepath.Clean`ed; empty stays empty. A relative path from the
    config file (or an alias's file key) is joined to the file's directory;
    from an env var, a flag or the default it stays relative to the working
    directory. `--help` shows a default as written.
  - `File` is `SecretFile` with a `[]byte` value and a 16 MiB cap. An empty
    file counts as not set. Getters return the shared slice: callers must
    not modify it.
  - `URL` must be absolute and not opaque: `localhost:5432` and
    `user:pw@host` parse with the first word as the scheme, so they're
    rejected, as are `mailto:` and `urn:` URLs (use a `String` key for
    those). Messages never quote a URL, nor the quoted parts of `url.Parse`'s
    errors (`invalid port ":pw host"` becomes `invalid port`): a password
    can sit in the user info, the query, or anywhere in a malformed URL, and
    `Redacted()` only hides a parsed user info's.
  - `Location` takes `time.LoadLocation` names: IANA names, `UTC`, `Local`.
    An empty value is nil, so a `Location` without a default or `Required`
    may be nil. Zones come from the system's zoneinfo; an image without it
    needs `import _ "time/tzdata"` in main, which the DSL doc says.
  - Flag type labels: `path`, `file`, `url`, `location`. Path, file and URL
    defaults are quoted in `--help`; zone names aren't.
  - Open for `Test()` (§5.4 of the interface): it does no I/O, but a
    `Location` default needs `LoadLocation`. Decide when `Test()` lands.
- **[decided, M4.6]** Lists and maps:
  - Flags record every occurrence (`rawValue` keeps them all), and the engine
    splits each occurrence, an env var, or a single value in the config file
    as one CSV record (`encoding/csv`, spaces after a comma dropped): `--tag a
    --tag b,c` and `ZP_TAG='a, "b,c"'` both work. A list in the config file
    gives its items as they are, commas included; a map gives `key=value`
    items for `StringMap`. Items must be single values: a null, list or map
    item is a problem at its own position.
  - Only the winning layer counts, and it replaces the whole list or map:
    layers never merge. An empty flag or env var gives an empty list, which
    counts as not set (A4); the getter returns nil for an empty list or map.
  - Empty items are kept in `Strings` (`a,,b`) but rejected in `Ints` and
    `Durations`, where they would quietly be 0. Item problems name the item:
    `item 2: "x" is not an integer`.
  - `StringMap` items split at the first `=`; spaces around a key are
    dropped, the value is kept, and a later item with the same key wins, so a
    script can override `--label a=1` with a later `--label a=2`. An empty
    key is a problem. In the config file a map key can't contain `=`.
  - Nothing under a `StringMap` key in the config file is an unknown key.
  - Defaults are recorded as one CSV record, checked with the engine's
    parsers at generate time, and written as literals (`[]int{80, 443}`), so
    each load gets a fresh slice. `--help` shows `[a,"b,c"]`, quoting items
    that hold a comma, a quote or outer spaces, and `[k=v]` for maps.
  - Help labels: `strings`, `ints`, `durations`, `key=value`.
  - The engine's `Key` gets `SetItems` in place of `Set` for these kinds, and
    `Map` for `StringMap`.
- **[decided, M4.7]** `Text` keys:
  - The schema checks T with reflect: a named, exported, non-generic type
    from a package, whose pointer implements `encoding.TextUnmarshaler`. The
    generator rejects a type declared in the schema package, as for enums.
    The third argument only names T and must be its zero value; a default
    goes through `Default`.
  - `Default` needs T or `*T` to implement `encoding.TextMarshaler`. The
    schema marshals the value, parses the text back with `UnmarshalText`,
    and requires the same value (`reflect.DeepEqual`), so the default the
    app loads is the one the schema wrote. Like `URL` and `Location`
    defaults, the text is parsed when loading (M4.5).
  - The text reaches `UnmarshalText` exactly, without trimming; empty text
    is T's zero value, which counts as not set (A4), checked with
    `reflect.Value.IsZero`, so T needn't be comparable.
  - The help label is the type's name in lower case (`prefix`), and a
    default is quoted.
- **[decided, M4.8]** Constraints, `Validate` and `DefaultFunc`:
  - `Range`, `Min` and `Max` exist on number, duration and size keys;
    `Len` and `Pattern` on `String` keys. Other keys use `Validate`. A
    `Pattern` matches the whole value, as `^(?:re)$`; it is compiled by the
    schema, and the generated code compiles it once into a package-level
    variable.
  - The schema checks a default against the constraints, whatever order
    the modifiers come in, and rejects `Range(max, min)`, a bad `Len` or a
    pattern that doesn't compile.
  - Checks run in the engine after parsing, on keys that are loaded,
    relevant and **set** (A4): the key's constraints in declaration order,
    then `Validate`. The first failure is the key's problem, with the
    winning source, or `default`. An unset value isn't checked, so an
    optional `Range(1, 65535)` port can be left at 0; `Required` is for
    presence. Messages don't quote the value (`must be at most 65535`), so
    they are safe for secrets; a `Validate` function's own message is
    passed through, and must not quote a secret.
  - `Validate(f)` takes `func(G) error` and `DefaultFunc(f)` takes `func()
    G`, where G is the getter's type (`cfgx.Secret`, `*url.URL`, `[]byte`),
    so the DSL's key type carries G. Method expressions can't type-check
    against those signatures, so only method values, closures, unexported
    and generic functions need the name rules. `DefaultFunc` is rejected on
    `File` and `SecretFile` keys, and with `Default`. `--help` shows
    `(default from runtime.NumCPU)`. In exclusive rules a `DefaultFunc`
    counts as a non-zero default.
  - `ResolveRefs` runs after `Build`: one `go list -e` call in the generated
    package's directory lists every package imported for a key (functions,
    enum and `Text` types) with its name and dependencies. It rejects
    `main`, packages that import `cfgx/dsl`, and `internal/` packages the
    generated package can't import; it calls functions in the generated
    package itself without an import. Function packages are imported under
    their real name, or an alias where it differs from the path.
- **[decided, M4.9]** Naming and source modifiers:
  - `Flag(name)` and `Env(name)` replace the derived name whole: the flag
    prefix and the env prefix aren't added. `Key(name)` replaces the key's
    name in the config file (its path within the group); the env var,
    flag, getter and rules keep using the declared name. `GoName(name)`
    replaces the key's segment of Go names, as `Group.GoName` does.
    `Short(r)` takes one ASCII letter or digit.
  - Every override goes through the same collision checks as derived names,
    app-wide, short flags included. Reserved: `--help` and `-h` always;
    `--config` and `<PREFIX>_CONFIG` when the app has a config file.
  - `Sources(dsl.FromFlag, dsl.FromEnv, dsl.FromFile)` drops the flag or
    env var a key would otherwise have. The interface wrote the sources as
    `dsl.Flag`, `dsl.Env`, `dsl.File`, but `dsl.File` is the File key
    constructor (M4.5), so they're `From…`. A key that can't come from the
    config file and is found there is ignored with a warning (not reported
    as unknown), and its required message doesn't offer the file. A
    `Secret` can't list `FromFlag`; a key naming a flag or env var that
    `Sources` drops, a file-only key in an app without a config file, and
    an empty `Sources()` are problems.
  - Arguments take `Short`, `Flag`, `GoName` and `Hidden`; `Env`, `Key` and
    `Sources` don't apply to them.
  - `Hidden` sets pflag's `Hidden`. `Example` is checked like a default and
    recorded for M6's example files.
- **[decided, M4.10]** Completion:
  - `Complete(dsl.Files("*.pem", "*.crt"))` filters file names by
    extension (patterns must be `*.ext`); `dsl.Dirs()` completes
    directories; `dsl.CompleteFunc(f)` calls a top-level `func(toComplete
    string) []string` under `Validate`'s rules, and turns file names off.
    The interface said `.Complete(c)` takes "a top-level func"; Go can't
    pass a function where `Files` and `Dirs` values go, so it's wrapped in
    `CompleteFunc`.
  - Enums complete their values, filtered by what's typed, with no file
    names. `--config` completes the app's formats (`yaml`, `yml`, `json`).
  - The engine registers completions with `RegisterFlagCompletionFunc` on
    the command whose flags it binds, and on the root for global keys. A
    key without a flag can't have a completion.
- **[decided, M6.5]** Test helpers:
  - `Values` nests one struct per group (`DBValues`, `SchedulerRetryValues`),
    named after the group's interface; a key's field is its own segment of
    the getter (`v.Auth.EmailFrom`, `v.Scheduler.Retry.Max`). Command
    arguments aren't in it. `All` embeds every top-level group type, so it
    implements every group and tag interface, computed methods included.
  - `Test(edit)` applies each key's default, then edit, through the same
    `values` struct the loader fills, so getters and computed methods behave
    as they do at runtime. It reads no config file, env var, flag, or
    `SecretFile`/`File` default path (A6). Defaults parsed from text are
    parsed (paths, URLs, `Text` types), and a `Location` default loads its
    zone as `time.LoadLocation` does: the zone database is the one read it
    makes, so `t.In(loc)` works in tests. A default that fails to parse
    panics. `DefaultFunc`s run.
- **[decided, M6.6]** Loading without cobra: the generated `LoadX(ctx,
  opts...)` calls `engine.LoadCommand`, which builds a private flag set
  with the command's flags (its keys, the global keys and their
  `RelevantIf` conditions, plus `--config`), so `cfgx.WithArgs` is parsed
  exactly as the command's flags would be, and the arguments that aren't
  flags become `Args()` (A12). `WithEnv(map)` replaces the process
  environment for env vars, the config file variable and `$VAR`s in search
  paths. `WithFile(path)` is `--config`; a `--config` in `WithArgs` wins.
  Warnings go to `WithLogger`'s logger, or to stderr as a command's do;
  problems come back as the error. Bad flags are an error, not a panic or
  exit.
- **[decided, M6.1]** Generated files: `cfgx gen` writes them under the
  module root (from `go env GOMOD`), or under `-artifacts dir` (`-` for
  none), only when their contents change:
  - `<config>.example.yaml` and `<config>.schema.json`, named after
    `App.ConfigFile`, only when the app has a config file.
  - `.env.example`, `docs/CONFIG.md` and `cfgx.lock.json`.

  The details:
  - The example config comments every key out at its example or default
    (`# port: 7777`), under its group's description. Each key gets its
    description, `Required.`, where else it can be set, and the commands
    that use it ("Used by Serve, Migrate", or "every command" for global
    keys), by schema command name, since cfgx doesn't know cobra's. A copy
    of it changes nothing until a line is uncommented, so `config init`
    pins no defaults. Values are JSON, which is YAML; secrets and
    `DefaultFunc` defaults are never written. Hidden keys and keys
    `Sources` keeps out of the file are left out.
  - The JSON Schema (draft 2020-12) has `additionalProperties: false`, so
    an editor flags a typo as cfgx warns about it. Kinds map to types,
    `anyOf` where the file also takes a CSV string or a duration's `0`;
    enums, `minimum`/`maximum`, `minLength`/`maxLength` and `pattern` come
    from the key; defaults and examples are typed; aliases are deprecated
    properties. Every key allows null, which cfgx reads as absent. The
    schema is stricter than cfgx about quoting: cfgx reads `port: "7777"`,
    but the schema wants `port: 7777`. A `Secret`'s default shows as
    `******` in the lock file and nowhere else.
  - `docs/CONFIG.md` has a table per group (hidden keys included), the
    rules, and each command's arguments.
  - `cfgx.lock.json` is the IR as JSON, without positions, keys sorted.
  - The example config and the schema are also embedded in the generated
    file, as `engine.App.ExampleConfig` and `JSONSchema` (raw string
    literals where possible), for `config init` and `config schema`
    (M6.4). Per-key metadata for `explain` comes with M6.4, which uses it.
- **[decided, M6.2]** `cfgx gen -check` renders the code and the artifacts
  in memory, writes nothing, and fails naming every file that is stale or
  missing; with `-artifacts -` it checks only the code.
  `cfgxtest.AssertGenerated(t)` finds the package's `//go:generate` line
  that runs `cfgx gen`, and runs it with `-check` added after `gen`, so it
  checks exactly what `go generate` would write. `examples/zp` calls it, so
  the examples test, which runs the example's tests, fails on a stale file
  twice over.
- **[decided, M6.3]** `cfgx diff old.lock [new.lock]` (new defaults to the
  module's `cfgx.lock.json`) compares keys by declared path, and command
  arguments as `Command --flag`. Breaking: a key removed (unless an alias
  in the new lock keeps its old path, which is a behaviour change), a type
  or enum type changed, an enum value removed, a key newly required or
  added as required, an `Env`/`Flag`/`Key`/`Short` override changed, a
  source dropped, the env prefix or config file name changed. Behaviour:
  a default, `DefaultFunc`, constraint or `Validate` changed, a key newly
  deprecated, the search paths changed. Safe: optional keys and enum values
  added, `Required` dropped, descriptions changed. The report groups them
  under those headings; `-fail-breaking` exits 1 on a breaking change, for
  CI. A lock file of any IR version parses; unknown fields are ignored.
- **[decided, M6.4]** The `config` subcommand (`engine.ConfigCommand`, generated
  as `ConfigCommand()`):
  - Scope: every key but command arguments, or the keys of the command
    named by the remaining arguments, resolved with `cmd.Root().Find`, so a
    nested command is `config validate db migrate`. Binders tag their
    command with `Annotations["cfgx.profile"]`, and `App.Profiles` maps
    profiles to key lists. Arguments are never in scope (A9).
  - Sources: defaults, the config file and env. The subcommands load
    through a flag set holding only `--config`, so a command's flags (and
    global flags) never count.
  - `print` and `explain` load with `Options.Partial`, skipping `Required`
    and rules, so they work on an incomplete configuration; values that
    don't parse are still problems. `validate` loads as a command would.
    Problems print as at startup and exit non-zero.
  - Values come from a generated `Show` per key: a `Secret` is `******`, a
    URL is `Redacted()`, a `File` is `(N bytes)`, an enum or `Text` value is
    its text, durations and sizes are canonical. `print` writes YAML by
    config file path, with JSON values. `explain` shows the winning
    source, `default runtime.NumCPU()` for a `DefaultFunc`, `unset`, and
    `(not relevant)` for a key a `RelevantIf` condition leaves out.
  - `env` lists the variables, their keys, and whether each is set now,
    never values. `init` writes the example config to `<config>.yaml` (or
    `-o`), refusing to overwrite without `--force`. `init` and `schema`
    exist only when the app has a config file.
- **[decided, ADR 0005]** One parser for both formats: `go.yaml.in/yaml/v4`,
  loading into `yaml.Node`. It rejects multi-document files and reports line and
  column in errors (`*yaml.LoadError`). The walker that builds cfgx's node tree
  also:
  - rejects duplicate keys itself, naming both positions, since the library
    doesn't when loading into a `Node`
  - rejects anchors, aliases and merge keys in v0.1.0 (a backlog item)
  - treats an empty or comment-only file (the loader's `io.EOF`) and a null root
    as an empty map
  - for `.json` files, first rewrites surrogate escapes with a string-aware
    scanner, because YAML rejects them but JSON tools such as Python's
    `json.dumps` write them. A pair becomes one `\U` escape, padded with spaces
    after the closing quote so columns stay exact; a lone surrogate becomes
    `\uFFFD` (ADR 0005)
- `.json` files accept YAML's superset syntax (comments, trailing commas). The
  leniency is documented rather than enforced against.

**Config file discovery.**
1. `--config` (persistent flag, added by `BindRoot`)
2. `ZP_CONFIG`
3. the search paths from `App.ConfigFile`, in order. `$VAR`s are expanded, and a
   path whose variable is unset or empty is skipped (`$HOME` comes from
   `os.UserHomeDir`). In each directory, `<basename>.yaml`, `.yml` and `.json`
   are tried; more than one match is an error.

An explicit path (1 or 2) that doesn't exist is an error. If the search paths
find nothing, that's fine. A file that exists but is invalid is always an
error.

**[decided, M3]** Details settled while building it:
- An app without `ConfigFile` reads no file: it has no `--config` flag, and
  `ZP_CONFIG` is just an unknown env var.
- A `--config` that was passed decides, even when empty, so `ZP_CONFIG` isn't
  consulted; an empty `--config` or `ZP_CONFIG` means "search as usual".
- The explicit path's extension must name an accepted format (compared
  ignoring case), and it must be a regular file. In a search path, a directory that happens to be named
  `config.yaml` is skipped.
- A null value in the file (`port: ~`, `port:`) is absent, not empty: only an
  env var or flag that is present but empty clears a value (A4).
- A file value of the wrong shape (a map or list where a value belongs, a value
  where a group's map belongs) is a problem only if that layer wins, like any
  other bad value. A shape problem above several keys is reported once.
- Not finding or not parsing the file is a plain error, printed as
  `zp: the config file is invalid: config.yaml:3:3: …`, because nothing else
  can be checked without it. Every other problem is collected into
  `cfgx.Errors`.
- On a configuration error the binder prints `<root name>: <problems>`
  itself and sets `SilenceErrors` and `SilenceUsage`, so cobra adds neither
  `Error:` nor the usage text; warnings follow the problems, as in interface
  §8. `Execute` still returns the error.
- A required key that isn't set says how to set it:
  `database.name: is required; set --database-name, ZP_DATABASE_NAME or
  database.name in the config file`. If a layer set it to the zero value, the
  problem names that layer, never the value: `is required, but is empty (env
  ZP_DATABASE_NAME)`, or `is required, but is set to the zero value` for text
  such as `0`. `Required` with a non-zero default is allowed and useful: an
  empty env var or flag clears the default (A4).
- The unknown-env warning reads `unknown environment variable ZP_DATABSE_HOST;
  did you mean ZP_DATABASE_HOST?`. Warnings carry slog attributes (`key` and
  `source`, or `env`) for custom loggers; the default handler prints only the
  message.
- Suggestions use optimal string alignment distance, with a limit that grows
  with the name's length (1 for up to 5 runes, at most 4), and a case-only
  difference is always suggested (`internal/suggest`).
- `BindRoot` is required when the app has a config file or `Global` groups,
  since they add flags to the root. A binder whose root wasn't bound fails
  with an error saying so, instead of silently ignoring `--config`.
- `Global` groups must also be passed to `dsl.App`, as in interface §3.1's
  example, so `dsl.App` stays the one list of groups. A command that also
  lists a global group in `Uses` is fine.
- The generator claims the identifiers later milestones generate
  (`ConfigCommand`, `Test`, `All`, `Values`, `Load<Command>`), so a schema
  that generates today keeps generating when they arrive.

### 3.5 Errors and provenance

- `engine.Errors` holds a slice of `Problem{Key, Message, Source}`, where
  `Source` is a flag name, an env var, `file:line:col`, `default` or `store`.
- Provenance is kept per key in the snapshot, as the list of every layer that
  provided a value, lowest to highest. `config explain`, shadow warnings and
  `Store.List()` use it. Rules don't: they're value-based (§3.3).
- **Warnings [decided, A2]** go to a `*slog.Logger`. The default is a small cfgx
  handler that prints `warning: <message>` lines, without timestamps, on the
  command's stderr (`cmd.ErrOrStderr()`, so tests can capture them).
  `cfgx.WithLogger` replaces it. cfgx has no logrus dependency; apps bridge it.
- Errors and warnings never include a secret's value: a problem on a `Secret` or
  `SecretFile` key names the key and the rule (A10).

### 3.6 `cfgx.Secret`

```go
type Secret struct{ reveal func() string } // a closure: reflection can't read it without unsafe
```

- `String`, `GoString`, `Format` (all verbs), `MarshalJSON`, `MarshalText`,
  `MarshalYAML` and `LogValue` all return the mask.
- `Equal` compares SHA-256 digests with `crypto/subtle`, without an early
  return (ADR 0006), so timing reveals neither the secret's length nor whether
  it's set. A `Secret` isn't comparable with `==`,
  since it holds a func.
- In an unexported field of a printed struct, `fmt` can't call these methods and
  prints the field as an address (`{reveal:0x4f2a80}`). The secret isn't shown,
  and reflection-based dumpers can't reach it without `unsafe`. Interface §7.1
  says so (A10).
- **[decided, ADR 0006]** The closure design passed every leak path tested:
  fmt, json, xml, templates, log, slog, errors, gob, reflect, go-spew and go-cmp.
  An unset Secret masks the same as a set one. There are no unmarshalers: they
  would let a masked `"******"` round-trip into a real value, and would let
  `dsl.Text` accept a Secret. The leak suite moves into package `cfgx` in M4.3.

### 3.7 Reloadable keys (after v0.1.0)

- `fsnotify` watches the config file's **directory**, not the file. Editors save
  by atomic rename, and Kubernetes ConfigMaps swap a `..data` symlink. Watch
  events are debounced.
- On a change, the engine re-runs the pipeline with flag, env and store layers
  unchanged. Only `Reloadable` keys take the new values. A changed non-reloadable
  key gets the warning "requires restart".
- A `SecretFile`/`File` key marked `Reloadable` also watches its referenced file
  (certificate rotation).
- `OnChange` subscribers receive the set of keys whose values changed. Callbacks
  run serially, after the snapshot swap.

### 3.8 Mutable keys and the store (v1, bbolt built in)

**Decided:**
- bbolt is a built-in dependency, and the app reaches the store through an
  injected `Store()` (amendment A1, interface §7.3).
- The store wins over flag, env, file and default (interface §4).
- An invalid stored value is ignored with a warning, and its record is kept.
- An env-var reset escape hatch (`ZP_SETTINGS_RESET=...`) was rejected on
  2026-11-07. Recovery from a bad stored value goes through the application
  (`ResetX()` or `Store.Reset`), or through a release that bumps the key's epoch.

**Packages.**
- `cfgx/store` holds the bbolt-backed `Store` and its admin API. Generated code
  imports it only when the schema declares `App.Store`, so programs without
  mutable settings don't link bbolt.
- `cfgx/internal/storage` is a small internal seam: load records, apply a batch,
  read history. `cfgx/store` implements it with bbolt. It isn't public; it only
  keeps a later shared backend possible.

**Lifecycle, per command that has the store:**
1. Load the static layers (default, file, env, flag), including the `App.Store`
   key, which every command with the store loads (A7).
2. Open the file named by the `App.Store` key with mode `0600` and
   `bbolt.Options{Timeout: ...}` (default 1 s).
3. Read the records for the command's mutable keys, apply epochs, decode and
   validate them, and layer them on top.
4. Call `run`. When it returns, the binder closes the store.

**On-disk layout:** one top-level bucket, `cfgx` (what `Bucket()` returns), with
nested buckets:
- `settings`: key → current record
- `history`: one nested bucket per key, with entries keyed by an 8-byte
  big-endian `NextSequence()`, so there are no timestamp collisions or prefix
  clashes between keys like `auth.otp` and `auth.otp.length`. The last 1000
  entries per key are kept.
- `meta`: format version, schema hash

Records are JSON (raw text, epoch, `updated_at`, `updated_by`, reason), so the
file stays inspectable with ordinary bbolt tools.

**Write pipeline**, shared by typed writes (`SetX`/`ResetX`) and dynamic ones
(`Store.Update`/`Store.Reset` with text values). One mutex per process is held
from step 1 to step 4, so concurrent writers can't lose each other's changes:
1. Build the candidate: the bootstrap snapshot (§3.2) plus the stored records as
   they will be after this change, parsed by the generated per-key parsers. A
   reset removes the key's record, so its bootstrap value comes back.
2. Validate the whole candidate: per-key checks, `Validate` functions, and rules
   across static and mutable keys.
3. In one bbolt transaction, append a history entry for every changed key,
   resets included, with who, when and why; then write or delete the records.
4. Swap the snapshot pointer.

After the mutex is released, `OnChange` callbacks run one at a time, in commit
order. A failure before step 3 commits leaves both the store and the snapshot
unchanged. Stored values are limited to 64 KiB each.

**Shadow warnings.** At startup, every stored value that hides a flag or env
value produces a log line:
`auth.otp.length: stored value 8 (admin@zp, 2026-11-02) overrides env ZP_AUTH_OTP_LENGTH=6`.
`config explain` and `Store.List()` show both values.

**Sharing the file.** The app uses `Store().DB()` for its own buckets; zp keeps
its existing data in the same file. The app must not write inside the `cfgx`
bucket. Nothing can enforce that, but a record that fails to decode is ignored
with a warning.

**Other processes.** bbolt locks the file per process. `config init`, `schema`
and `env` never open the store. `config print`, `explain` and `validate` skip it
if the file doesn't exist (they never create it), and otherwise open it
read-only. A CLI command run while the server holds the file waits for the
timeout, then:
- `config print`, `explain` and `validate` carry on without stored values and
  say so
- anything that needs the store fails with "store is held by a running process;
  change settings through the application"

**Limits.**
- Each replica has its own file, so runtime settings are per instance. A shared
  store (e.g. Postgres with `LISTEN/NOTIFY`) is post-v1 work behind
  `internal/storage`.
- No secrets in the store in v1, since bbolt doesn't encrypt.

### 3.9 Cobra integration

- `ServeCommand(cmd, run)` registers the command's flags and completions. It
  sets `RunE` to: load the config from `cmd.Flags()`, then
  `run(cmd.Context(), cfg)`. It also sets `SilenceUsage` for config errors, so a
  validation failure doesn't print the usage text.
- Completion: enums use `RegisterFlagCompletionFunc`; `dsl.Files` uses
  `MarkFlagFilename`; `dsl.Dirs` uses `MarkFlagDirname`.
- `BindRoot` records its options, such as the logger, in an internal registry
  keyed by the root command. Binders and the `config` subcommand look them up at
  run time through `cmd.Root()`, so there is no user-visible global and several
  roots can coexist in tests.
- Each binder tags its command with `Annotations["cfgx.profile"]`. The `config`
  subcommand walks `cmd.Root()` to map command names, as typed, to profiles.
- The `config` subcommand is generic engine code driven by the tables. It sees
  default, file, env and store values, never other commands' flags, and never
  checks command-only `Args` (A9).

---

## 4. The cfgx repository

```
cfgx/                  runtime: Secret, Option, Errors, public helpers
cfgx/engine/           generic engine used by generated code (exported, not for humans)
cfgx/dsl/              schema vocabulary + Export
cfgx/store/            built-in bbolt store + admin API (linked only when a schema declares a store)
cfgx/internal/storage/ backend seam for a future shared store (not public)
cfgx/cfgxtest/         test helpers: AssertGenerated, leak checks
cfgx/cmd/cfgx/         generator CLI: gen, gen --check, diff, init
cfgx/internal/ir/      IR types (shared by dsl and the generator)
cfgx/internal/gen/     templates + emitters
cfgx/internal/node/    format-agnostic node tree + YAML/JSON builders
cfgx/examples/zp/      the zp schema from interface.md, an end-to-end acceptance test
```

**[proposed]** Use a single Go module for now. Programs link only the packages
they import, so an app never links the generator's dependencies (`x/tools`).
Those dependencies do appear in the app's `go.mod` and `go.sum`, but apps add
`cmd/cfgx` as a `tool` anyway, so a separate module would save little. (An
earlier version of this note credited module graph pruning; that was wrong.)

Runtime dependencies:
- `spf13/cobra`
- `spf13/pflag`
- a YAML library
- `go.etcd.io/bbolt`, linked only into programs whose schema declares a store
  (it lives in `cfgx/store`)
- later: `fsnotify` (reload)

---

## 5. Testing strategy

| Layer | How |
|-------|-----|
| DSL → IR | unit tests on `dsl.Export` output |
| IR checks | table tests: one bad schema per error, asserting message and schema line |
| Generator | golden files under `testdata/` (`-update` flag) |
| Generated code | the e2e module in `examples/zp` runs `go generate`, `go vet` and `go test`, and `cfgx gen --check` must be clean |
| Engine | unit tests with hand-written tables; precedence matrix (every pair of layers); value-based rule tests; fuzzing for Duration, ByteSize, lists, maps and `k=v` parsing |
| Secret | leak test suite (S5) |
| Store | temp-file bbolt tests: locked file, read-only opens, missing file never created, per-key history sequences, concurrent writers under the race detector |
| CLI behaviour | `testscript` (`rogpeppe/go-internal`) scripts: run the binary with env, flags and files, and assert stdout/stderr, covering `config explain` and error output |
| Performance | benchmark the example's startup time against a viper equivalent, to check the startup-speed claim honestly |

---

## 6. Spikes (do these first, in a day or two each)

| ID | Question | Kill criterion |
|----|----------|----------------|
| S1 | Does the temp-main + `go run` pipeline work with `internal/`, `go.work` and vendor, how fast is it, and where can the temporary `main` live without touching the working tree (`-overlay`, `_` or `.` directories)? | warm run > 5 s, or vendor/work modes impossible → fall back to static AST evaluation of a restricted DSL |
| S2 | Do pflag custom values, `Changed` semantics, cobra persistent flags and completion compose? | precedence can't be made correct → rethink flag binding |
| S3 | Are `FuncForPC` names reliably convertible to importable references, including `%2e`-escaped paths, method expressions and ABI wrappers? | no → validators must be referenced by `dsl.Func("pkg.Name")` |
| S4 | Which YAML library: node positions, duplicate keys, JSON parsing? | none acceptable → separate JSON path with `encoding/json/jsontext` offsets |
| S5 | Can Secret masking hold under all `fmt`, json, slog and reflection-dumper paths? | a leak path we can't close → document it and add the vet analyzer earlier |

**Outcome (2026-11-07):** all five spikes passed and none of the kill criteria
fired. The decisions are ADRs [0002](adr/0002-generator-pipeline.md) to
[0006](adr/0006-secret-type.md).

---

## 7. Open questions resolved in this brainstorm

| # | Question (interface §11) | Proposal |
|---|--------------------------|----------|
| 1 | Mutable precedence | **[decided]** The store wins over everything. Mitigated by shadow warnings and `explain`; recovery through `ResetX()` or an epoch bump (§3.8) |
| 2 | Invalid stored value at startup | **[decided]** Fall back, warn, and keep the record |
| 3 | Epochs vs migrations | **Epochs only** at first. Numbered migrations only if a real type or unit change requires one |
| 4 | Lists of objects | Generated read-only item types with getters (`[]settings.Upstream`), file-only. Deferred past v1 |
| 5 | File layering | **First found wins** in v1. Repeated `--config` overlays later |
| 6 | TOML in v1 | **No.** YAML and JSON only |
| 7 | Module path and name | **[decided]** `github.com/epistemer/cfgx` |
| 8 | Test helper scope | One `Test(...) *All` |

## 8. v1 scope [decided]

The first usable release has:
- **The static core:**
  - the DSL (all key kinds except lists of objects), groups, tags, rules and
    commands
  - flags, env, and YAML/JSON files
  - generated getters, interfaces, command binders and test helpers
  - errors with provenance
  - example files, JSON Schema, `docs/CONFIG.md` and the lock file
  - the `config` subcommand
  - `gen --check` and `diff`
- **`cfgx init` scaffolding.**
- **Mutable keys** with epochs, audit history, the generated `Update` API, the
  built-in **bbolt** store, and the dynamic admin API for dashboards (A1).

After v1:
- Reloadable keys
- a shared store for multiple replicas (e.g. Postgres)
- lists of objects
- TOML
- config overlays
- grouped `--help` output
- the vet analyzer for `Reveal()` leaks
