# ADR 0003: Flag binding on cobra and pflag

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-07 |
| Roadmap item | M1.2 (spike S2) |
| Deciders | Claude (owner) |

## Context

cfgx registers a flag for most keys, but a flag must count only when it was
actually passed. Otherwise its default would hide an env var or a file value,
which is viper's classic bug ([implementation.md §3.4](../implementation.md)).
The spike checked that this binding works with cobra's persistent flags, bool
and repeatable flags, `--help`, and shell completion, and that cfgx keeps
control of parsing and error messages.

The spike (`spikes/s2-flags`, code at commit `e1e3e77`) used cobra v1.10.2 and
pflag v1.1.0, with a `zp` root that has a persistent `--log-level` and a `serve`
command with `--port`/`-p`, `--color`, `--tag` and `--cert`.

## Options

There was one design to test, so this records what it does:

- Every key gets a custom `pflag.Value` that only stores the raw text. Its
  `Type()` gives the help label (`int`, `string`, `bool`, `duration`,
  `stringSlice`).
- `Flag.DefValue` holds the schema default for display only; pflag never
  applies it. (The decision below moves the displayed default into the usage
  text instead.)
- Bool flags set `NoOptDefVal = "true"`.
- Repeatable keys use a value that implements `pflag.SliceValue` and records
  every occurrence. (The spike split commas inside `Set`, which lets pflag
  reject malformed input; the decision below moves splitting into the engine.)
- The engine resolves precedence itself and reads a flag only if `Changed`.

All of these pass on Go 1.27.3 and 1.26.0:

| Behaviour | Result |
|-----------|--------|
| No flag, no env | the default, source `default` |
| Env set, flag not passed | the env value wins over the flag's displayed default |
| Flag and env both set | the flag wins |
| Short flag `-p 1234` | works |
| Env var present but empty | provides the empty value, as A4 requires |
| `--port abc` | reaches cfgx as raw text, so its error message names the source like every other source |
| Persistent flag before or after the subcommand | both visible through the child's `cmd.Flags()` in `RunE` |
| `--color`, `--color=false` | `true`, `false`; both count as passed |
| `--tag a --tag b,c` | `[a b c]` |
| `--help` | `-p, --port int … (default 7777)`, `(default "info")`, `--tag strings`; persistent flags under "Global Flags"; no `(default false)` |
| Enum completion | the values, with directive 4 (no file completion) |
| File completion (`MarkFlagFilename(…, "pem")`) | directive 8, filtered on `pem` |

One finding needs action: **pflag v1.1.0 changes how cobra's own help flag
renders**, as `-h, --help[=true|false]`. With pflag v1.0.10 it's `-h, --help`.
cfgx's own flags render correctly with either.

## Decision

Adopt the binding as tested. cfgx requires cobra v1.10.2 and doesn't raise
pflag beyond what cobra requires (v1.0.9; v1.0.10 is the latest v1.0.x), so it
never forces the v1.1.0 help rendering on applications.

## Consequences

- [implementation.md §3.4](../implementation.md) ("Flags.") records the
  decision; M2.4 implements it, and M4.6 implements the slice value.
- pflag never rejects a value, so every parse error comes from cfgx's own
  parsers with one message format and the flag named as its source.
- **Slices are split by the engine, not by pflag.** `Set` only appends the raw
  occurrence, so malformed input such as `--tag '"unclosed'` reaches cfgx's
  error report with the flag as its source. An explicitly empty occurrence
  (`--tag ""`) provides an empty list, as A4 requires (review 0002, F01).
- **cfgx writes `(default …)` into the usage text itself and leaves `DefValue`
  empty.** For value types it doesn't know, pflag decides whether a default is
  "zero" from its text. It would hide a string default of `0` or `false`, and
  show `(default 0s)` for an unset duration. Type labels are chosen to read well
  in help: `strings`, `ints`, `durations` (F09).
- If an application raises pflag to v1.1.0 for its own reasons, cobra's help
  flag renders oddly; cfgx can't prevent that, and it's cosmetic.
