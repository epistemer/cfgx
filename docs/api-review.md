# API review for v0.1.0

Roadmap item M9.3: every exported identifier in `cfgx`, `dsl` and `store`,
checked before the first release. `engine` is outside the review: it is for
generated code and has no compatibility promise, as its package doc says.

v0.1.0 makes no compatibility promise either: Go modules treat every v0
release as unstable. This review is about getting the API right before
people start depending on it. Changes to it are amendments in
[interface.md §12](interface.md).

Each identifier was checked for:

1. **Needed.** Something an application, library or schema uses, or a
   generated-code entry point that must be exported, and says so.
2. **Named.** Reads well at the call site (`cfgx.By("admin")`,
   `dsl.Int("port", "Port").Min(1)`), consistent with its neighbours.
3. **Documented.** A doc comment that says what it does, its edge cases and
   its errors, with no internal jargon.
4. **Hard to misuse.** Misuse fails at compile time or at generate time, or
   gives a clear error, never a silent surprise.
5. **Hides internals.** No internal type a caller has to name.

✅ passes · 🔧 fixed in this review · 📝 proposal for the owner (below)

## Package cfgx

| Identifier | | Notes |
|---|---|---|
| `Secret`, `NewSecret` | ✅ | The cleartext is in a closure, so fmt, encoding, slog and reflection all see `******` (ADR 0006). An empty string is unset. A Secret holds a func, so `==` doesn't compile. |
| `Secret.Reveal`, `Bytes`, `IsSet` | ✅ | `Bytes` returns a copy. |
| `Secret.Equal` | ✅ 📝 | It takes a string, the candidate value, such as a token from a request header. It compares in constant time, and an unset Secret equals nothing, not even `""`. Go's convention is `Equal(T)`: see P8. |
| `Secret.String`, `GoString`, `Format`, `LogValue`, `MarshalJSON`, `MarshalText`, `MarshalYAML` | ✅ | Every one masks. The leak suite runs them all in CI. |
| `Option` | ✅ 📝 | A func type over an internal struct, so only cfgx can make options. godoc shows `func(*opts.Root)`: see P3. 🔧 The doc now says BindRoot uses only `WithLogger`. |
| `WithLogger` | ✅ | |
| `WithArgs`, `WithEnv`, `WithFile` | 🔧 📝 | The `WithArgs` doc had a broken sentence. All three now say only LoadX uses them. Passed to BindRoot they do nothing, and `WithFile` does nothing for an app with no config file: see P2. |
| `WriteOption`, `By`, `Reason` | ✅ | Same pattern as `Option`. |
| `Errors`, `Problem` | 🔧 | Loading returns every problem at once, sorted by key; `errors.As` finds `Errors`. 🔧 `Errors.Error` had no doc. |

## Package dsl

| Identifier | | Notes |
|---|---|---|
| Package doc | 🔧 | The shared modifiers come from an embedded generic type, so godoc writes them with `K`, `T` and `G`. The package doc now explains the three. See P5. |
| `App`, `AppDef` and its `ConfigFile`, `EnvPrefix`, `Formats`, `Global`, `Initialisms`, `Store` | ✅ | Misuse fails at generate time with the schema's file and line, e.g. a `Global` group missing from `App`, or a non-Path `Store` key. |
| `Group`, `GroupDef` and its `Computed`, `FlagPrefix`, `GoName`, `Rules`, `Tags` | ✅ | `Computed` names methods as strings. The generator type-checks the settings package and fails if one is missing, so a typo can't get through. |
| `Command`, `CommandDef` and its `Args`, `Uses`, `WithStore` | 🔧 | 🔧 The `Uses` doc said groups only; it takes tags too. |
| `Tag`, `TagDef` | ✅ | |
| `Member`, `KeyDef`, `Usable` | ✅ | Sealed interfaces with unexported methods: only dsl types satisfy them, so a misplaced value fails to compile. |
| `AtMostOne`, `ExactlyOne`, `AllOrNone`, `Requires`, `RelevantIf`, `RuleDef` | ✅ | The docs give the argument order (`Requires(a, b)`: b is required when a is set). Keys with non-zero defaults are rejected at generate time where a rule can't work with them. |
| Key constructors: `Bool`, `Int`, `Int64`, `Uint`, `Float`, `String`, `Duration`, `ByteSize`, `URL`, `Location`, `Path`, `File`, `Secret`, `SecretFile`, `Strings`, `Ints`, `Durations`, `StringMap`, `Enum`, `Text`, and their `…Key` types | ✅ | Each doc names the getter's type and the text it parses. `Enum` and `Text` are the only generic constructors, and Go infers their type argument at the call site. |
| Shared modifiers on all 19 key types: `Default`, `DefaultFunc`, `Example`, `Required`, `Validate`, `Alias`, `Deprecated`, `Env`, `Flag`, `Short`, `Key`, `GoName`, `Hidden`, `Sources`, `Tags`, `Complete`, `Mutable`, `Epoch` | ✅ 🔧 | Combinations that can't work fail at generate time with a reason, such as a Mutable secret, a Mutable `File` key, or `Default` together with `DefaultFunc`. 🔧 `Enum`'s `Example` took a value outside the enum without complaint, and `StringMap`'s took map keys that don't survive their text form; both now fail like `Default` does. |
| `Min`, `Max`, `Range` on `Int`, `Int64`, `Uint`, `Float`, `Duration`, `ByteSize`; `Len`, `Pattern` on `String` | ✅ | Each exists only where it makes sense: there's no `Min` on `StringKey`. `Pattern`'s regular expression is compiled at generate time, so a bad one fails there. |
| `Source`, `FromFlag`, `FromEnv`, `FromFile` | ✅ | The `From` prefix avoids a clash with the `File` constructor, as its doc says. |
| `Format`, `YAML`, `JSON` | ✅ | |
| `Completion`, `Files`, `Dirs`, `CompleteFunc` | ✅ | |
| `Export` | ✅ | For the generator's temporary main, which is built inside the user's module and so can't import cfgx's internal packages. The doc says schemas don't call it. |

## Package store

| Identifier | | Notes |
|---|---|---|
| `Store` | ✅ | The doc says writes are all or nothing and validate the whole configuration. A command gets it from `Store()`; nothing else creates one. |
| `Store.Keys`, `List`, `Get` | ✅ 📝 | Static keys are included, with secrets masked. `Setting.Source` is display text: see P1. An unknown key's error can't be matched: see P6. `Get` builds the whole list to find one key; with tens or hundreds of keys that costs microseconds, so it stays. |
| `Store.Update`, `Reset` | ✅ | They take canonical text, which `Get` returns, so writing back what you read changes nothing; tests cover it for the formatters of every kind and for the store. Empty input does nothing. |
| `Store.History` | ✅ | Newest first, at most 1000 entries kept. `Record.Reset` tells a reset apart from an empty value. It takes no context: a bbolt read can't be cancelled, so one would promise nothing. |
| `Store.DB`, `Bucket` | ✅ | For the app's own buckets in the same file. The doc says not to close the database or write inside `Bucket()`. |
| `KeyInfo`, `Setting`, `Record` | ✅ | Plain structs with documented fields. Secrets can't be Mutable (A10), so history never holds cleartext. |
| `Open`, `New` | ✅ 📝 | Entry points for generated code, documented as such. Their parameter and result types are in `internal/storage`, so an application can't call them usefully: see P4. |

## Proposals for the owner

Each would change what an application writes or sees, so each is your
decision (ownership.md). I recommend P2, P6 and P7 before v0.1.0, since
each is small and the release is when people start to depend on the API.
The rest can wait or stay as they are.

| ID | Proposal | Kind | Recommendation |
|----|----------|------|----------------|
| P1 | `store.Setting.Source` is display text ("flag --port", "env ZP_X", "file config.yaml:7:9"). A dashboard that wants to group or colour by source has to parse it. Add a `Kind` field (store, flag, env, file, default, unset) next to it. | Additive | Do it after v0.1.0, when a dashboard asks for it. |
| P2 | Options that do nothing where they're passed. `WithArgs`, `WithEnv` and `WithFile` passed to BindRoot are ignored, and `WithFile` is ignored by LoadX for an app without `App.ConfigFile`. Either way the program runs without the input the developer meant to give it. Make BindRoot panic at startup with a message naming the option, and make LoadX return an error. (A separate `LoadOption` type would catch the first at compile time, but splits one option list into two.) | Behaviour change | Do it before v0.1.0. It's small, and silent misuse is what this API avoids everywhere else. |
| P3 | `cfgx.Option` and `WriteOption` are func types over internal structs, so godoc shows `func(*opts.Root)`. An opaque struct would hide that, with an internal bridge so the engine can apply it. Correct code keeps compiling, except code that compares an option with `nil`. | Type change | Decide before v0.1.0 if you'd rather never change it after; otherwise before v1.0.0. It changes nothing at call sites, so I lean to leaving it. |
| P4 | `store.Open` and `store.New` are generated-code entry points in a public package, with internal types in their signatures. They're needed because the generated `Store()` returns `*store.Store`, which only package store can build. Generated code lives in the app's module, so it can't reach an internal package; the only other way is a bridge through `engine`. | Removal | Leave them. They're documented as generated-code hooks, and an app can't call them usefully. |
| P5 | The shared modifiers show as `Default(v T) K` in godoc. Generating a copy of every modifier on each key type would show `Default(v int) *IntKey` instead. Call sites stay the same, but dsl gains about 400 lines of generated code. | Docs only | Leave it. The package doc now explains `K`, `T` and `G`. |
| P6 | An unknown key gives a plain error that can't be matched, and `Get`'s differs from the one `Update`, `Reset` and `History` give. An HTTP dashboard that serves `GET /settings/{key}` can't tell a 404 from a 500 without matching the text. Add `store.ErrUnknownKey`, wrapped by all four. | Additive | Do it before v0.1.0. It's small, and a dashboard is the store API's main user. |
| P7 | A schema that builds its groups in `init` produces an app with no keys and no error, because `init` runs after `App`'s initializer. `cfgx gen` could reject an app with no keys. | Generate-time check | Do it before v0.1.0: it turns a confusing empty result into an error that names the cause. |
| P8 | `Secret.Equal(v string)` breaks Go's convention that `Equal` compares two values of the same type, as in `time.Time.Equal`. Comparing two Secrets means calling `a.Equal(b.Reveal())`. The intended use is checking a candidate, such as a request's token, against the configured secret. | Rename | Keep it: it's the method's whole purpose, and `Equal(string)` reads well at that call site. A `Secret`-to-`Secret` comparison can be added later if anyone needs one. |
