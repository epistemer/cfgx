# ADR 0006: Closure-backed `Secret` type and leak surface

| | |
|---|---|
| Status | Accepted |
| Date | 2026-11-07 |
| Roadmap item | M1.5 |
| Deciders | Claude (owner) decided; Gemini proposed |

## Context

`cfgx.Secret` (decision D8, [interface.md §7.1](../interface.md),
[implementation.md §3.6](../implementation.md), and amendment A10 from
[review 0001 F12](../reviews/0001-gemini-doc-review.md)) must keep sensitive
configuration values out of accidental printing, serialization, logging, error
formatting, and reflection-based struct dumping.

Spike S5 (`spikes/s5-secret/`, roadmap item M1.5, code at commit `5f95fcf`) prototypes `Secret` and runs a
leak test suite across standard library formatters, encoders, loggers,
templates, error wrappers, pure-Go reflection, and third-party reflection
dumpers (`github.com/davecgh/go-spew/spew` and `github.com/google/go-cmp/cmp`)
to answer two questions:

1. Can a closure-backed `Secret` keep its value out of every common printing,
   encoding, and logging path?
2. What can't it protect against?

The prototype in `spikes/s5-secret` has the following shape:

```go
type Secret struct {
	reveal func() string
}
```

- **Constructor and unset invariant:** `New(s string) Secret` returns `Secret{}`
  (`reveal == nil`) when `s == ""`, and wraps non-empty strings in a closure
  `func() string { return s }`. Treating `""` as unset matches cfgx's
  value-based "set" rule ([interface.md §3.5](../interface.md), A4), where an
  empty environment variable or flag clears a value and counts as unset.
- **Accessors and comparison:**
  - `Reveal() string` returns `""` when unset or calls `s.reveal()`.
  - `Bytes() []byte` returns `nil` when unset or a fresh defensive copy
    (`append([]byte(nil), v...)`) so mutating the slice cannot affect the
    `Secret` or subsequent calls.
  - `IsSet() bool` reports `s.reveal != nil && s.reveal() != ""`.
  - `Equal(v string) bool` returns `false` when unset (even if `v == ""`). The
    spike first compared raw bytes with `crypto/subtle.ConstantTimeCompare`;
    review changed that to SHA-256 digests compared without an early return
    (see Consequences).
  - Because `Secret` holds a `func() string` field, the type is incomparable
    (`==` and `map[Secret]T` are compile errors, and
    `reflect.TypeFor[Secret]().Comparable()` is `false`).
- **Masking methods:** `String() string`, `GoString() string`,
  `Format(fmt.State, rune)`, `MarshalJSON() ([]byte, error)`,
  `MarshalText() ([]byte, error)`, `MarshalYAML() (any, error)` (matching the
  standard YAML marshaler interface without importing a YAML library), and
  `LogValue() slog.Value` all emit the fixed mask `******` (`"******"` in JSON).

## Options

### 1. Storage representation

- **Option 1A: Closure field (`struct { reveal func() string }`) — chosen.**
  - *Benefits:* Neither pure-Go `reflect` nor `unsafe`-enabled struct dumpers
    (`go-spew` even with `DisableMethods: true`, `go-cmp` with
    `cmp.AllowUnexported`) can print the secret string, because walking the
    struct only reaches a `reflect.Func` pointer (`0x...`), not a `string`
    field. Calling an unexported `func` field via `reflect.Value.Call` panics.
    It also makes `Secret` non-comparable, forcing callers to use constant-time
    `Equal` instead of `==`.
  - *Costs:* One small closure allocation per set `Secret` at config load time
    (negligible: config loads once at startup). In an unexported field of a
    printed parent struct, `fmt` prints the function address (`{0x...}` or
    `{reveal:0x...}`) rather than `******`.
- **Option 1B: Unexported string field (`struct { value string }`) — rejected.**
  - *Costs:* Reflection dumpers that use `unsafe` (`spew` with methods disabled,
    `cmp.Diff` with `cmp.AllowUnexported`) and `fmt` on unexported parent fields
    (`%v`, `%+v`, `%#v`) print the raw `string` in cleartext. It also allows
    non-constant-time `==` comparisons and using `Secret` as a map key.

### 2. Formatting of an unset `Secret`

- **Option 2A: Always mask as `******` (both set and unset) — chosen.**
  - *Benefits:* Matches [interface.md §7.1](../interface.md) and
    [implementation.md §3.6](../implementation.md) without an amendment. Never
    leaks even the 1-bit presence or absence of an optional secret (e.g.
    `auth.token` or `smtp.password`) into logs or debug payloads. Avoids
    serializing `"<unset>"` as a valid-looking string literal in `MarshalJSON`,
    `MarshalText`, and `MarshalYAML`. Callers and cfgx's own diagnostic commands
    (`config explain`, `Store.List()`) have `IsSet()` when they explicitly need
    to distinguish unset keys.
  - *Costs:* Printing a zero `Secret` in ad-hoc debugging shows `******` unless
    the caller checks `IsSet()`.
- **Option 2B: Print `<unset>` or `""` when unset — rejected.**
  - *Costs:* In `MarshalJSON`, `MarshalText`, and `MarshalYAML`, `"<unset>"` is
    indistinguishable from a secret whose cleartext is `"<unset>"`, while `""`
    can vanish silently in log lines. Having `String()` and `MarshalText()`
    disagree would make masking inconsistent across surfaces.

### 3. Whether `UnmarshalJSON` / `UnmarshalText` should exist

- **Option 3A: Omit `UnmarshalJSON` and `UnmarshalText` — chosen.**
  - *Benefits:*
    1. `cfgx` decodes secrets in its own engine via `cfgx.NewSecret(raw)` and
       never unmarshals into `cfgx.Secret` through `encoding/json` or
       `encoding.TextUnmarshaler`.
    2. Prevents the round-trip footgun where marshaling a struct to JSON/YAML
       (producing `"******"`) and unmarshaling it back silently overwrites the
       secret with the literal string `"******"`.
    3. Preserves the DSL security boundary ([interface.md §3.2](../interface.md)):
       `dsl.Text(name, desc, zero T)` accepts any `T` whose pointer implements
       `encoding.TextUnmarshaler`. If `*Secret` implemented `TextUnmarshaler`, a
       schema author could pass `cfgx.Secret{}` to `dsl.Text`, bypassing all
       `dsl.Secret` protections (no CLI flag by default, `_FILE` support,
       regular-file and 1 MiB limits, and rejection of `.Mutable()`).
- **Option 3B: Implement `UnmarshalJSON` and `UnmarshalText` — rejected.**
  - *Costs:* Unused by `cfgx`, enables silent `"******"` round-trip corruption,
    and weakens the compile-time boundary between `dsl.Secret` and `dsl.Text`.

### Leak suite results (`spikes/s5-secret/leak_test.go`)

Every test used canary value `"hunter2-LEAK-CANARY"` and verified that neither
the canary, any substring, nor its hex encoding appeared in the output:

| Surface / Path | Target | Result | Exact output / behaviour |
|----------------|--------|--------|--------------------------|
| `fmt` verbs `%v`, `%+v`, `%#v`, `%s`, `%q`, `%x`, `%X`, `%d` (plus width/precision `%10s`, `%.3s`, `%-10v`) | `Secret` and `*Secret` (set and unset) | **Masked** | `******` |
| `fmt` verb `%T` | `Secret` and `*Secret` | **Type only** | `secret.Secret` / `*secret.Secret` |
| `fmt` verb `%p` | `*Secret` and `Secret` | **Address only** | `0x...` on `*Secret`; `%!p(secret.Secret={0x...})` (or `{<nil>}` when unset) on `Secret` value (handled by `fmt` before `fmt.Formatter`) |
| `fmt.Print`, `Println`, `Sprint`, `Sprintln`, `Sprintf`, `Fprint`, `Fprintln`, `Fprintf`, `Errorf` | `Secret` and `*Secret` | **Masked** | `******` |
| Parent struct with **exported** `Secret` field (by value and pointer) | `%v`, `%+v`, `%#v` | **Masked** | `{****** db}`, `{Secret:****** Name:db}`, `secret.exportedHolder{Secret:******, Name:"db"}` |
| Parent struct with **unexported** `Secret` field (by value and pointer) | `%v` | **Address only** | `{{0x...}}` when set, `{{<nil>}}` when unset (`&{{0x...}}` for pointer; field names omitted by `%v`) |
| Parent struct with **unexported** `Secret` field (by value and pointer) | `%+v` | **Address only** | `{secret:{reveal:0x...}}` when set, `{secret:{reveal:<nil>}}` when unset (`&{secret:{reveal:0x...}}` for pointer) |
| Parent struct with **unexported** `Secret` field (by value and pointer) | `%#v` | **Address only** | `secret.unexportedHolder{secret:secret.Secret{reveal:(func() string)(0x...)}}` |
| Slices (`[]Secret`), arrays (`[N]Secret`), maps (`map[string]Secret`), nested structs, interfaces (`any`, `fmt.Stringer`) | `%v`, `%+v`, `%#v` | **Masked** | `******` (or address only for nested unexported struct fields) |
| Map key (`map[Secret]V` / `reflect.MapOf`) | Type system / `reflect` | **Error / Panic** | Compile error (`invalid map key type Secret`); `reflect.TypeFor[Secret]().Comparable() == false`; `reflect.MapOf` panics |
| `encoding/json` (`json.Marshal`) | `Secret`, `*Secret`, exported struct field, slice, map | **Masked** | `"******"` |
| `encoding/json` (`json.Marshal`) | Unexported struct field | **Omitted** | `{}` |
| `MarshalText()` and `MarshalYAML()` | `Secret` | **Masked** | `******` |
| `encoding/xml` (`xml.Marshal`) | `Secret`, `*Secret`, exported struct field | **Masked** | `<Secret>******</Secret>` |
| `encoding/xml` (`xml.Marshal`) | Unexported struct field | **Omitted** | `<unexportedHolder></unexportedHolder>` |
| `text/template` and `html/template` (`{{.}}`, `{{printf "%v" .}}`, `%+v`, `%#v`) | `Secret`, `*Secret`, exported and unexported parent structs | **Masked / Address only** | `******` for value, pointer, and exported field; `{0x...}` / `{reveal:0x...}` for unexported field |
| `log.Print`, `log.Println`, `log.Printf` | `Secret`, `*Secret`, exported and unexported parent structs | **Masked / Address only** | `******` (or function address for unexported field) |
| `log/slog` `TextHandler` and `JSONHandler` | Top-level attr, `slog.Any` on `Secret`/`*Secret`, inside `slog.Group` | **Masked** | `******` (via `slog.LogValuer`) |
| `log/slog` `TextHandler` and `JSONHandler` | `slog.Any` on parent struct with **exported** `Secret` field | **Masked** | `{Secret:****** Name:db}` in text; `{"Secret":"******","Name":"db"}` in JSON |
| `log/slog` `TextHandler` and `JSONHandler` | `slog.Any` on parent struct with **unexported** `Secret` field | **Address only / Omitted** | `{secret:{reveal:0x...}}` in `TextHandler`; `{}` in `JSONHandler` |
| `errors.New(fmt.Sprint(...))` and `fmt.Errorf("...: %w", ...)` | `Secret`, `*Secret`, exported and unexported parent structs | **Masked / Address only** | `******` (or function address for unexported field) |
| `github.com/davecgh/go-spew/spew` (`spew.Sdump` default config) | `Secret`, `*Secret`, exported and unexported parent structs | **Masked** | `(secret.Secret) ******` (even for unexported fields: `spew` uses `unsafe` to invoke `String()`) |
| `github.com/davecgh/go-spew/spew` (`DisableMethods: true`) | `Secret`, `*Secret`, exported and unexported parent structs | **Address only** | `reveal: (func() string) 0x...` |
| `github.com/google/go-cmp/cmp` (`cmp.Diff` default) | Structs holding two different `Secret` values | **Panic (no leak)** | Panics on unexported `Secret.reveal` field |
| `github.com/google/go-cmp/cmp` (`cmp.Diff` with `cmp.AllowUnexported`) | Structs holding two different `Secret` values | **Address only** | `Secret{reveal: ⟪0x...⟫}` |
| `encoding/gob` (`gob.Encode`) | `Secret`, `*Secret`, exported struct field, or unexported-only struct | **Error (no leak)** | `gob: type secret.Secret has no exported fields` (`gob` does not use `TextMarshaler`) |
| `encoding/gob` (`gob.Encode`) | Struct with exported non-secret field and unexported `Secret` field | **Omitted** | Encodes only the exported field; ignores the unexported `Secret` |
| Pure-Go `reflect` field inspection (no `unsafe`) | `reflect.ValueOf(s).Field(0)` | **Blocked / Panic** | `Kind() == reflect.Func`; `CanInterface() == false`; `Field(0).Call(nil)` panics; closure captures are invisible to `reflect` |

### What `Secret` cannot protect against

1. **`Reveal()` or `Bytes()` passed to a logger, formatter, or error:** Once the
   caller extracts the cleartext `string` or `[]byte`, `Secret` is no longer in
   the path. (Mitigation in the post-v0.1.0 backlog: a `go vet` analyzer that
   flags `Secret.Reveal()` and `Secret.Bytes()` values flowing into logging or
   formatting calls.)
2. **`unsafe`:** Code importing `unsafe` can cast `*Secret` to `*func() string`
   (or use `reflect.NewAt` with `UnsafeAddr()`) and call the closure, or
   dereference the runtime `funcval` struct to read the captured `string` header
   directly from memory.
3. **Explicit method invocation via `reflect` or templates:** While field-walking
   reflection cannot read or call the unexported `reveal` closure, `Reveal()`
   and `Bytes()` are exported methods. Calling
   `reflect.ValueOf(s).MethodByName("Reveal").Call(nil)` or evaluating
   `{{.Reveal}}` / `{{printf "%s" .Bytes}}` in `text/template` or
   `html/template` invokes the exported accessor and returns the cleartext.
4. **In-memory lifetime (Go strings cannot be zeroed):** Go `string` values are
   immutable. The cleartext string captured by the closure (and any buffer used
   while parsing env vars or files) remains in heap memory until garbage
   collected and overwritten, and the garbage collector may copy it. Zeroing the
   slice returned by `Bytes()` clears only that copy.
5. **Core dumps, heap dumps (`runtime/debug.WriteHeapDump`), `/proc/<pid>/mem`,
   debuggers, and swap:** Any OS-level memory inspection can read the cleartext
   string from process memory.
6. **Process environment (`/proc/<pid>/environ`):** Secrets passed via
   environment variables remain readable in the process environment block; using
   `<ENV>_FILE` or `dsl.SecretFile` avoids placing secrets in `environ`.

## Decision

Adopt the closure-backed `Secret` design (`type Secret struct { reveal func() string }`)
as prototyped in `spikes/s5-secret`. Every tested printing, encoding, logging,
template, and reflection-dumper path either masks the value as `******`, prints
only a function pointer address (`0x...`), omits the field, or returns an error
without leaking the secret. Mask both set and unset `Secret` values uniformly as
`******`, treat `NewSecret("")` as unset (`Secret{}`), and omit `UnmarshalJSON`
and `UnmarshalText`.

## Consequences

- **`docs/interface.md` §7.1:** No interface amendment is needed. Section 7.1's
  statement that an unexported `Secret` field prints `{reveal:0x4f2a80}` is
  exact for `%+v`; if §7.1 is edited for clarity later, it can note that `%v`
  omits the field name (`{0x4f2a80}`) while `%+v` prints `{reveal:0x4f2a80}` and
  `%#v` prints `cfgx.Secret{reveal:(func() string)(0x4f2a80)}`, and can add
  unzeroable Go strings in memory alongside `Reveal()`, `unsafe`, and core
  dumps.
- **Roadmap item M4.3 (`cfgx.Secret`):** Implements `Secret` and `NewSecret` in
  package `cfgx` with the methods from `spikes/s5-secret/secret.go`
  (`NewSecret`, `Reveal`, `Bytes`, `IsSet`, `Equal`, `String`, `GoString`,
  `Format`, `MarshalJSON`, `MarshalText`, `MarshalYAML`, `LogValue`) and no
  unmarshalers.
- **`Equal` without timing leaks (added in review):** `subtle.ConstantTimeCompare`
  returns early when the lengths differ, so timing reveals whether a guess has the
  secret's length, and an early `false` for an unset secret reveals whether it's
  set. `Equal` therefore hashes both values with SHA-256 (an unset secret hashes
  the empty string), compares the 32-byte digests in constant time, and combines
  that with set-ness without branching. `spikes/s5-secret/secret.go` does this
  (review 0002, F10).
- **Leak test suite migration:** In M4.3, `spikes/s5-secret/leak_test.go` moves
  into the `cfgx` package to guard `cfgx.Secret` permanently in CI (and is
  required by M9.2), and `spikes/s5-secret/` is removed once M4.3 lands.
