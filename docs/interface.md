# cfgx — Developer Interface (v1, locked)

> Status: **locked on 2026-11-07.** Any change needs an explicit amendment,
> recorded in §12. This document describes only what a developer writes and what
> they get back. How it's built is in [implementation.md](implementation.md).

## 0. Decisions so far

| # | Decision | Choice |
|---|----------|--------|
| D1 | Source of truth | **Go DSL**, executed by the generator at `go generate` time |
| D2 | Struct tags | **None.** Structs, getters and interfaces are generated *output* |
| D3 | How libraries consume config | **Interfaces of getters** only. No validation, sources or commands |
| D4 | Getter naming | `SMTPHost()` — path-derived, no `Get` prefix, Go initialisms |
| D5 | Viper | **Replaced.** cfgx owns flag/env/file merging; cobra + pflag stay |
| D6 | Command-local args | Declared in the schema via `dsl.Command(...).Args(...)` |
| D7 | Where tag interfaces live | **Central generated package only.** Libraries that would hit an import cycle declare their own interface |
| D8 | `Secret` | Non-generic, string-backed, with a `Bytes()` view |
| D9 | Runtime store | bbolt is built in, exposed through an injected `Store()` (§7.3). No global variables |
| D10 | Logging | `log/slog` inside cfgx, replaceable at startup. Apps that use logrus bridge it |
| D11 | License | MIT |

Decisions made after the lock are amendments, listed in §12.

---

## 1. Principles

1. **Config-first.** One schema declares every setting the program has. Commands
   *select* settings; settings never name commands.
2. **One declaration, every surface.** A key declared once yields its flag, env
   var, file key, getter, interface method, validation, docs, example files,
   JSON Schema and shell completion.
3. **Convention over configuration.** Names are derived from the key path; you
   override only when you must.
4. **Fail at generate time, then at startup — never later.** Name collisions,
   bad defaults and unknown references fail `go generate`. Invalid values fail
   startup with every problem listed at once. No silent fallbacks.
5. **Consumers can't tell static, reloadable and mutable settings apart.** They
   call a getter.
6. **No reflection at runtime.** Reflection is used only inside the generator.
7. **Strings are for humans.** References between declarations (rules, tags,
   commands) are Go values, so a typo is a compile error.

---

## 2. Project layout

```
cmd/zp/main.go                    # hand-written: cobra tree
internal/settings/
  schema/schema.go                # hand-written: THE schema (DSL)
  settings.go                     # hand-written: //go:generate line + computed methods
  zz_settings.go                  # generated
config.example.yaml               # generated
.env.example                      # generated
config.schema.json                # generated (editor autocomplete for config.yaml)
docs/CONFIG.md                    # generated reference
cfgx.lock.json                    # generated snapshot of the schema (diffable in PRs)
```

```go
// internal/settings/settings.go
package settings

//go:generate go tool cfgx gen ./schema
```

The generated package name is whatever package the `go:generate` line lives in.

Three Go packages ship with cfgx:

| Package | Imported by | Purpose |
|---------|-------------|---------|
| `cfgx/dsl` | your `schema` package only (never linked into the binary) | declaration vocabulary |
| `cfgx` | generated code, and libraries that touch `cfgx.Secret` | small runtime: `Secret`, load and write options, errors, explain |
| `cfgx/store` | generated code, only when the schema declares a store; your admin code | the built-in bbolt store and its admin API (§7.3) |

`cfgx` itself is dependency-light: its only third-party runtime deps are cobra,
pflag and the file-format parsers. bbolt comes in through `cfgx/store`, so only
programs whose schema declares a store link it.

---

## 3. The schema DSL

### 3.1 A complete example

```go
package schema

import (
	"runtime"
	"time"

	"github.com/epistemer/cfgx/dsl"

	"zp/internal/validate"
	"zp/models"
)

// Tags: cross-cutting sets of keys that become interfaces.
var (
	Mailer = dsl.Tag("Mailer", "Everything the mailer needs")
	Tokens = dsl.Tag("Tokens", "Signing keys and lifetimes")
)

var Log = dsl.Group("log", "Logging",
	dsl.Enum("level", "Minimum log level", "debug", "info", "warn", "error").Default("info"),
	dsl.Enum("format", "Output format", "text", "json").Default("text"),
	dsl.Bool("color", "Force colored output even without a TTY"),
)

var DB = dsl.Group("database", "Database connection",
	dsl.String("host", "Database server host").Default("localhost"),
	dsl.Int("port", "Database server port").Default(5432).Range(1, 65535),
	dsl.String("name", "Database name").Required(),
	dsl.String("user", "Database user").Default("postgres"),
	dsl.Secret("password", "Database password").Required(),
	dsl.Enum("sslmode", "SSL mode", "disable", "require", "verify-full").
		Default("disable").Alias("database.ssl_mode"),
).GoName("DB").Computed("DSN")

var SMTP = dsl.Group("smtp", "Outgoing mail",
	dsl.String("host", "SMTP server host").Required(),
	dsl.Int("port", "SMTP server port").Default(587),
	dsl.String("user", "SMTP user"),
	dsl.Secret("password", "SMTP password"),
	dsl.Enum("auth", "SMTP auth mechanism", "PLAIN", "LOGIN", "CRAM-MD5").Default("PLAIN"),
	dsl.Bool("tls", "Use implicit TLS"),
).Tags(Mailer)

var (
	apiToken = dsl.Secret("token", "Static API token")
	apiKeys  = dsl.SecretFile("keys", "CSV file of API keys").Complete(dsl.Files("*.csv"))
)

var Auth = dsl.Group("auth", "Authentication",
	dsl.String("emailfrom", "Sender address for auth emails").
		Validate(validate.Email).Tags(Mailer),
	dsl.Duration("otp.ttl", "OTP validity").Default(5*time.Minute),
	dsl.Int("otp.length", "OTP length").Default(6).Range(4, 10).Mutable(),
	dsl.Duration("session.ttl", "Session lifetime").Default(24*time.Hour).Tags(Tokens),
	dsl.SecretFile("jwt.private", "JWT private key (PEM)").Default("keys/priv.pem").Tags(Tokens),
	dsl.SecretFile("jwt.public", "JWT public key (PEM)").Default("keys/pub.pem").Tags(Tokens),
	apiToken,
	apiKeys,
	dsl.Bool("mfa", "Require multi-factor authentication"),
).Rules(
	dsl.AtMostOne(apiToken, apiKeys),
)

var (
	tlsEnabled = dsl.Bool("tls.enabled", "Serve HTTPS")
	tlsCert    = dsl.Path("tls.cert", "TLS certificate").Default("ssl/cert.pem").Complete(dsl.Files("*.pem"))
	tlsKey     = dsl.SecretFile("tls.key", "TLS private key").Default("ssl/key.pem")
)

var Server = dsl.Group("server", "HTTP server",
	dsl.String("host", "Listen address").Default("0.0.0.0"),
	dsl.Int("port", "Listen port").Default(7777).Range(1, 65535).Short('p'),
	tlsEnabled, tlsCert, tlsKey,
	dsl.Duration("timeout.read", "Read timeout").Default(20*time.Second),
	dsl.Duration("timeout.write", "Write timeout").Default(60*time.Second),
	dsl.ByteSize("max_header", "Maximum request header size").Default(1 << 20),
	dsl.Int("rate.limit", "Requests per second per IP").Default(100).Mutable(),
	dsl.Int("rate.burst", "Burst capacity per IP").Default(20).Mutable(),
).FlagPrefix("").Rules(
	dsl.RelevantIf(tlsCert, tlsEnabled),
	dsl.RelevantIf(tlsKey, tlsEnabled),
)

var Scheduler = dsl.Group("scheduler", "Task scheduler",
	dsl.Location("location", "Scheduler timezone").Default("Africa/Cairo"),
	dsl.Enum("execmode", "Task execution mode",
		models.ZpTaskExecutionModeSequential,
		models.ZpTaskExecutionModeParallel,
	).Default(models.ZpTaskExecutionModeSequential),
	dsl.Int("workers", "Concurrent workers").DefaultFunc(runtime.NumCPU),
	dsl.Group("retry", "Retry policy",
		dsl.Int("max", "Maximum retries").Default(3),
		dsl.Duration("delay", "Delay between retries").Default(time.Minute),
	),
).Computed("RetryPolicy")

var boltFile = dsl.Path("bolt", "Bolt database file; also holds runtime settings").Default("bolt.db")

var Storage = dsl.Group("storage", "Local storage", boltFile)

// The application.
var App = dsl.App("zp", "ZaynPay backend",
	Log, DB, SMTP, Auth, Server, Scheduler, Storage,
).
	EnvPrefix("ZP").
	ConfigFile("config", ".", "$HOME/.zp", "/etc/zp").
	Initialisms("OTP", "JWT", "SMTP", "MFA").
	Global(Log).
	Store(boltFile)

// Commands select settings. They are config profiles, not cobra commands.
var (
	Serve = dsl.Command("Serve").Uses(DB, SMTP, Auth, Server, Scheduler)

	Migrate = dsl.Command("Migrate").Uses(DB).Args(
		dsl.Bool("down", "Roll back instead of applying"),
		dsl.Int("steps", "Number of migrations; 0 means all").Min(0),
	)

	TestEmail = dsl.Command("TestEmail").Uses(Mailer).Args(
		dsl.String("to.name", "Recipient name").Default("John Doe"),
		dsl.String("to.address", "Recipient address").Required().Validate(validate.Email),
		dsl.String("template", "Template to render").Default("new-account"),
	)
)
```

Notes on the example:

- **Groups, tags and commands are Go values.** `Uses(DB)`, `Tags(Mailer)` and
  `AtMostOne(apiToken, apiKeys)` are typed references. A misspelling doesn't compile.
- **Every key needs a description.** The generator refuses to run without one.
- **Generics are used internally but never written out.** For example, `Enum`
  infers `models.ZpTaskExecutionMode` from its arguments, so `.Default(...)` only
  accepts that type.

### 3.2 Key constructors

Every constructor is `dsl.X(name, description string, ...)` and returns a key
with type-checked modifiers.

| Constructor | Getter returns | Notes |
|-------------|----------------|-------|
| `String` | `string` | |
| `Int`, `Int64`, `Uint`, `Float` | the matching Go type | |
| `Bool` | `bool` | flag is `--x` / `--x=false` |
| `Duration` | `time.Duration` | `"90s"`, `"5m"`; bare numbers rejected (no unit ambiguity) |
| `ByteSize` | `int64` | `"1MiB"`, `"512KB"`, or bytes |
| `Enum(name, desc, values ...T)` | `T` | `T` is any type whose underlying type is a string, integer, float or bool type. A value's text is its `MarshalText`, else a string type's value, else its `String`, else its literal. Completion + docs list the values |
| `Secret` | `cfgx.Secret` | never a flag by default; also reads `<ENV>_FILE` (setting both is an error) |
| `SecretFile` | `cfgx.Secret` | the setting is a path, resolved like `Path`; contents are loaded eagerly at startup from a regular file of at most 1 MiB |
| `Path` | `string` | a path, cleaned and resolved relative to the config file's directory when set in a file |
| `File` | `[]byte` | the setting is a path, resolved like `Path`; contents are loaded eagerly from a regular file of at most 16 MiB |
| `URL` | `*url.URL` | parsed |
| `Location` | `*time.Location` | IANA name |
| `Strings`, `Ints`, `Durations` | `[]T` | flag repeatable or comma-separated; env comma-separated |
| `StringMap` | `map[string]string` | env/flag `k=v,k2=v2` |
| `Text(name, desc, zero T)` | `T` | any `T` whose pointer implements `encoding.TextUnmarshaler` (`netip.Prefix`, your own types) |
| `Group(name, desc, keys...)` | — | nesting; see §3.4 |

Lists of *objects* (e.g. `upstreams: [{host, port}, …]`) are file-only and come
after v0.1.0 (§11).

For kinds whose value is derived from text (`Path`, `SecretFile`, `File`, `URL`,
`Location`), `.Default` and `.Example` take that text: a path, a URL, an IANA
zone name (A6).

### 3.3 Key modifiers

| Modifier | Meaning |
|----------|---------|
| `.Default(v)` | typed default; for text-derived kinds, the text (§3.2) |
| `.DefaultFunc(f)` | default computed at startup by a top-level `func() T` (e.g. `runtime.NumCPU`) |
| `.Example(v)` | shown in example files instead of the default |
| `.Required()` | must be *set* (§3.5) after merging |
| `.Range(min, max)`, `.Min`, `.Max` | numeric/duration bounds |
| `.Len(min, max)`, `.Pattern(re)` | string constraints |
| `.Validate(f)` | top-level `func(T) error`; closures rejected at generate time |
| `.Tags(t...)` | add to tag interfaces |
| `.Short('p')` | short flag |
| `.Flag(name)`, `.Env(name)`, `.Key(name)` | override derived names |
| `.GoName(name)` | override derived getter name |
| `.Sources(dsl.Flag, dsl.Env, dsl.File)` | restrict where a value may come from |
| `.Alias(oldPath...)` | old file key / env / flag still accepted, with a deprecation warning. Renames only: a type or unit change needs a migration (open question §11) |
| `.Deprecated(msg)` | warn when set |
| `.Hidden()` | omitted from help and example files (still documented in `docs/CONFIG.md`) |
| `.Complete(c)` | completion: `dsl.Files(globs...)`, `dsl.Dirs()`, or a top-level func. Enums complete automatically |
| `.Reloadable()` | value is re-read when the config file changes (§7; after v0.1.0) |
| `.Mutable()` / `.Epoch(n)` | value can be changed at runtime and persisted (§4, §7) |

### 3.4 Groups

```go
dsl.Group(name, desc, keysOrGroups...)
```

| Modifier | Meaning |
|----------|---------|
| `.Tags(t...)` | every key inside joins these tags |
| `.Rules(r...)` | cross-key constraints (§3.5) |
| `.FlagPrefix(p)` | flag prefix for keys inside (default: the path; `""` gives `--port` instead of `--server-port`) |
| `.GoName(n)` | getter / interface name segment (`database` → `DB`) |
| `.Computed(method...)` | declares hand-written methods as part of this group's interface (§5.3) |

Every group automatically produces an interface named after its Go name. Nested
groups produce nested interfaces that the parent embeds.

A dotted key name such as `dsl.Bool("tls.enabled", ...)` is only a path
shorthand. It nests the file key (`tls: { enabled: … }`) and lengthens the
names, but creates no interface. Use an explicit `dsl.Group` when a sub-tree
deserves its own interface.

### 3.5 Rules

Rules take key values, so they can only reference keys that exist.

| Rule | Meaning |
|------|---------|
| `AtMostOne(a, b, ...)` | mutually exclusive |
| `ExactlyOne(a, b, ...)` | one and only one |
| `AllOrNone(a, b, ...)` | set together or not at all |
| `Requires(a, b)` | if `a` is set, `b` must be set |
| `RelevantIf(a, b)` | `a` only matters when `b` is set. When `b` isn't, `a` is neither loaded nor validated (no file is read), and setting `a` explicitly warns |

**"Set" is value-based (A4).** A key is set when its effective value is non-zero,
and a Bool is set when it's true, so `--tls-enabled=false` doesn't count. An env
var or flag that is present but empty provides the empty value, which is how an
operator clears a value from the config file. Keys in `AtMostOne`, `ExactlyOne`
and `AllOrNone` can't have non-zero defaults; that's a generate-time error.

### 3.6 Naming conventions

For key path `server.tls.cert` in app `zp`:

| Surface | Derived name | Override |
|---------|--------------|----------|
| File key | `server: { tls: { cert: … } }` | `.Key` |
| Env | `ZP_SERVER_TLS_CERT` | `.Env` |
| Flag | `--server-tls-cert` (or `--tls-cert` under `FlagPrefix("")`) | `.Flag`, `.FlagPrefix` |
| Getter | `ServerTLSCert()` | `.GoName` |

Initialisms come from a built-in list (ID, URL, TLS, HTTP, DB, …) plus
`App.Initialisms(...)`. Any collision on any surface fails generation and names
both declarations.

### 3.7 App

| Method | Meaning |
|--------|---------|
| `dsl.App(name, desc, groups...)` | the root |
| `.EnvPrefix(p)` | defaults to the upper-cased app name |
| `.ConfigFile(basename, searchPaths...)` | search paths tried in order, skipping any whose `$VAR` is unset or empty; in each, `.yaml`, `.yml` and `.json` are tried, and two matches is an error. `--config` or `ZP_CONFIG` overrides the search and must exist (A11) |
| `.Formats(dsl.YAML, dsl.JSON)` | accepted file formats (default: both) |
| `.Initialisms(...)` | extra initialisms |
| `.Global(groups...)` | settings every command gets, bound as persistent flags on the root |
| `.Store(pathKey)` | keep mutable keys in the built-in bbolt store, in the file named by `pathKey`, which must be a static `Path` key (§7.3). Every command that has the store, and the `config` subcommands, load `pathKey` and bind its flag automatically (A7) |

### 3.8 Commands

```go
dsl.Command(goName).Uses(groupsOrTags...).Args(keys...)
```

- A command is a **config profile**. It produces one generated type
  (`settings.Serve`) and one binder (`settings.ServeCommand`). It does not know
  its cobra `Use` string or its place in the tree; `main.go` decides that.
- `Uses` accepts groups and tags. The command gets those keys, plus every
  `Global` group. Its type has getters only for the keys it loads: a group it
  uses whole is embedded, computed methods included; a group it reaches only
  through a tag contributes getters for just the tagged keys (A5).
- `Args` keys are **flag-only**, belong to no group or tag interface, and get
  getters on the command type (`m.Down()`, `m.Steps()`).
- Positional arguments stay with cobra: you set rules such as
  `cobra.ExactArgs(1)` on the command, and the run function reads them with
  the generated `Args()` method (§5.2, A12).
- A command gets the store (§7.3) automatically when its keys include a mutable
  one. `.WithStore()` gives it to a command without mutable keys, for example
  one that only needs `Store().DB()`.

---

## 4. Precedence

```
static and reloadable keys:   flag  >  env  >  config file  >  default
mutable keys:        store >  flag  >  env  >  config file  >  default
```

For a mutable key, the bootstrap sources (flag, env, file, default) provide the
value only until someone writes it at runtime. After that, the stored value wins.
When a stored value shadows a flag or env value, startup logs a warning and
`config explain` shows the shadowed source. A stored value that no longer passes
the schema is ignored with a warning, and the next layer is used.

---

## 5. Generated API

### 5.1 Interfaces

```go
// zz_settings.go (excerpt)

// DB: Database connection
type DB interface {
	DBHost() string
	DBPort() int
	DBName() string
	DBUser() string
	DBPassword() cfgx.Secret
	DBSSLMode() string
	DSN() string // computed, see §5.3
}

// Mailer: Everything the mailer needs
type Mailer interface {
	SMTPHost() string
	SMTPPort() int
	SMTPUser() string
	SMTPPassword() cfgx.Secret
	SMTPAuth() string
	SMTPTLS() bool
	AuthEmailFrom() string
}

type Server interface {
	ServerHost() string
	ServerPort() int
	ServerTLSEnabled() bool // dotted key "tls.enabled": a path segment, not a sub-interface
	ServerTLSCert() string
	ServerTLSKey() cfgx.Secret
	// ...
}

// Scheduler embeds the interface of its nested dsl.Group("retry", ...)
type Scheduler interface {
	SchedulerLocation() *time.Location
	SchedulerExecMode() models.ZpTaskExecutionMode
	SchedulerWorkers() int
	SchedulerRetry
	RetryPolicy() types.RetryPolicy // computed
}

type SchedulerRetry interface {
	SchedulerRetryMax() int
	SchedulerRetryDelay() time.Duration
}
```

### 5.2 Command types and binders

```go
type Serve struct{ /* unexported */ } // implements Log, DB, SMTP, Mailer, Auth, Tokens, Server, Scheduler
type Migrate struct{ /* ... */ }      // implements Log, DB; plus Down() bool, Steps() int

func ServeCommand(cmd *cobra.Command, run func(context.Context, *Serve) error) *cobra.Command
func MigrateCommand(cmd *cobra.Command, run func(context.Context, *Migrate) error) *cobra.Command
func BindRoot(root *cobra.Command, opts ...cfgx.Option) // --config, Global flags, completion; cfgx.WithLogger(l)

// On every command type: the positional arguments, as cobra parsed them (a
// copy). Argument rules such as cobra.ExactArgs(1) are set on cmd as usual.
func (s *Serve) Args() []string

// Programmatic and test loading (no cobra):
func LoadServe(ctx context.Context, opts ...cfgx.Option) (*Serve, error)

// Only on commands that have the store (§7.3):
func (s *Serve) Store() *store.Store
func (s *Serve) Update(ctx context.Context, fn func(u *Update) error, opts ...cfgx.WriteOption) error
func (s *Serve) Close() error // binders call it after run returns; LoadServe callers must call it
```

**Scope is enforced by the compiler.** `*Migrate` doesn't implement `Mailer`, so
`mailer.New(m)` inside the migrate command doesn't compile. The same holds inside
a group: `*TestEmail` uses only the `Mailer` tag, so it has `AuthEmailFrom()`
but no other `Auth` getter.

### 5.3 Computed values

Computed values are ordinary hand-written methods on the generated group type,
in the generated package:

```go
// internal/settings/settings.go
func (g *dbGroup) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(g.DBUser(), g.DBPassword().Reveal()),
		Host:     net.JoinHostPort(g.DBHost(), strconv.Itoa(g.DBPort())),
		Path:     g.DBName(),
		RawQuery: "sslmode=" + g.DBSSLMode(),
	}
	return u.String()
}
```

The receiver type is unexported and named after the group's Go name, with the
first word lower-cased, plus `Group`: `DB` → `dbGroup`, `SMTP` → `smtpGroup`,
and the `Retry` group nested in `Scheduler` → `schedulerRetryGroup` (A8).

A command that uses the whole `DB` group embeds `dbGroup`, so `DSN()` is
promoted to it. A command that has only some of `DB`'s keys doesn't get it.
`Group.Computed("DSN")` adds the method to the `DB` interface. The generator
reads its exact signature with a type-checking pass, so any method without
parameters works, including `RetryPolicy() (types.RetryPolicy, error)`. A
missing method is a generate-time error.

### 5.4 Test helpers

```go
// Values is a plain nested struct of every key, pre-filled with defaults.
func Test(edit func(v *Values)) *All // *All implements every interface

// in a library test:
m := mailer.New(settings.Test(func(v *settings.Values) {
	v.SMTP.Host = "localhost"
	v.SMTP.Password = cfgx.NewSecret("x")
}))
```

`Test()` does no I/O. `Values` fields have the getter types (`cfgx.Secret`,
`[]byte`, `*time.Location`), and fields of `SecretFile` and `File` keys stay
empty unless the test sets them: their default paths are never read (A6).

---

## 6. Wiring

### 6.1 main.go

```go
func main() {
	root := &cobra.Command{Use: "zp", Short: "ZaynPay backend"}
	settings.BindRoot(root, cfgx.WithLogger(logger)) // optional: any *slog.Logger; default is stderr

	root.AddCommand(
		settings.ServeCommand(&cobra.Command{Use: "serve", Short: "Run the API server"},
			func(ctx context.Context, s *settings.Serve) error {
				db, err := store.Open(ctx, s)
				if err != nil {
					return err
				}
				return server.Run(ctx, s, db, mailer.New(s), scheduler.New(s), admin.New(s.Store()))
			}),
		settings.MigrateCommand(&cobra.Command{Use: "migrate"},
			func(ctx context.Context, m *settings.Migrate) error {
				return migrate.Run(ctx, m, m.Down(), m.Steps())
			}),
		settings.TestEmailCommand(&cobra.Command{Use: "test-email"}, testemail.Run),
		settings.ConfigCommand(), // see §8
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

### 6.2 A library

```go
package mailer

import "zp/internal/settings"

type Mailer struct{ cfg settings.Mailer }

func New(cfg settings.Mailer) *Mailer { return &Mailer{cfg: cfg} }

func (m *Mailer) dial() (*smtp.Client, error) {
	addr := net.JoinHostPort(m.cfg.SMTPHost(), strconv.Itoa(m.cfg.SMTPPort()))
	// ... m.cfg.SMTPPassword().Reveal() only at the point of use
}
```

A package that the generated `settings` package itself imports (e.g. `models`,
for enum types) can't import `settings`. Such a package declares its own
interface with the same method names (D7).

### 6.3 Adding a setting

1. Add one line to `schema.go`.
2. Run `go generate ./...`.
3. Use the new getter.

The flag, env var, file key, docs, example files, JSON Schema, completion and
interface method all follow.

---

## 7. Runtime semantics visible to developers

### 7.1 `cfgx.Secret`

```go
type Secret struct{ /* unexported */ }

func NewSecret(s string) Secret
func (s Secret) Reveal() string
func (s Secret) Bytes() []byte          // a copy
func (s Secret) IsSet() bool
func (s Secret) Equal(v string) bool    // constant-time
```

`Secret` is masked as `******` in `String`, `GoString`, every `fmt` verb, JSON,
YAML, `encoding.TextMarshaler` and `slog.LogValuer`. When it sits in an unexported field of a
struct that gets printed, `fmt` can't call its methods and shows an address
(`{reveal:0x4f2a80}`) instead of the mask. The secret itself still isn't
printed, and reflection-based dumpers can't read it without `unsafe` (A10).

What it can't protect against: `Reveal()` passed to a logger, `unsafe`, and core
dumps.

Secret defaults:
- not exposed as a flag (`ps` and shell history would leak it)
- `ZP_DATABASE_PASSWORD_FILE=/run/secrets/db` is accepted instead of
  `ZP_DATABASE_PASSWORD`; setting both is an error, and one trailing newline is
  trimmed
- never shown in error or warning messages: a problem names the key and the
  rule, not the value
- can't be `Mutable` in v0.1.0, since bbolt doesn't encrypt; the generator
  rejects it

### 7.2 Reloadable and Mutable keys

- **Getters always return the current value.** Consumers don't change.
- **Consumers that build long-lived objects from a setting must subscribe.** For
  example, a rate limiter built at startup won't see new values on its own. The
  API is provisional:

  ```go
  s.OnChange(func(c settings.Change) {
  	if c.Has(settings.KeyServerRateLimit, settings.KeyServerRateBurst) {
  		limiter.Reset(s.ServerRateLimit(), s.ServerRateBurst())
  	}
  })
  ```

- **Reloads and updates are all-or-nothing.** If one value fails validation, no
  value changes.
- **Mutable keys ship in v1** (§7.3). Reloadable keys come after v0.1.0. The
  getter side above doesn't change for either, so neither breaks a consumer.

### 7.3 The store and admin access

bbolt is built in. The schema names the database file with `App.Store(pathKey)`
(§3.7).

**Lifecycle.**
- A command that has the store opens the file while loading its config, before
  `run` is called, because stored values are part of loading. The binder closes
  it after `run` returns; `LoadX` callers call `Close()`.
- bbolt lets one process open a file at a time. Inside the process, cfgx is the
  only opener, and the app shares the handle through `Store().DB()`.
- In another process (for example `zp config explain` while `zp serve` runs),
  cfgx waits up to a timeout. Then the `config` subcommands carry on without
  stored values and say so, and anything that needs the store fails with
  "store is held by a running process".

**Typed writes**, for application code:

```go
err := s.Update(ctx, func(u *settings.Update) error {
	u.SetServerRateLimit(200)
	u.ResetAuthOTPLength() // remove the override; the bootstrap value applies again
	return nil
}, cfgx.By("admin@zp"), cfgx.Reason("Black Friday traffic"))
```

**Dynamic access**, for a settings dashboard or other generic code. Inject it like
any other dependency: `admin.New(s.Store())`.

```go
package store // github.com/epistemer/cfgx/store

func (s *Store) DB() *bbolt.DB    // shared handle; never close it
func (s *Store) Bucket() []byte   // name of cfgx's reserved top-level bucket
func (s *Store) Keys() []KeyInfo  // every key of the command
func (s *Store) Get(key string) (Setting, error)
func (s *Store) List() []Setting
func (s *Store) Update(ctx context.Context, values map[string]string, opts ...cfgx.WriteOption) error
func (s *Store) Reset(ctx context.Context, keys []string, opts ...cfgx.WriteOption) error
func (s *Store) History(key string, limit int) ([]Record, error)

type KeyInfo struct {
	Key, Description, Type string
	Mutable                bool
	Default                string          // canonical text
	Enum                   []string        // allowed values, if any
	Schema                 json.RawMessage // JSON Schema of the value, for rendering forms
	Tags                   []string
}

type Setting struct {
	KeyInfo
	Value    string   // canonical text; secrets masked
	Source   string   // "store", "flag --port", "env ZP_DATABASE_HOST", "file config.yaml:7", "default"
	Shadowed []string // sources hidden by a stored value
	Change   *Record  // last stored change, if any
	Invalid  string   // why a stored value was ignored, if it was
}

type Record struct {
	Value     string
	Epoch     int
	UpdatedAt time.Time
	UpdatedBy string
	Reason    string
}
```

Rules:
- **Writes are text.** `Store.Update` takes each value in its canonical text form
  (`"5m"`, `"parallel"`) and parses it with the same generated code as every
  other source. Typed and dynamic writes share one pipeline: validate the whole
  candidate (rules included), write it with history in one transaction, swap the
  snapshot, then fire `OnChange`. Only mutable keys are accepted. Any error
  means nothing changes.
- **Reads cover every key of the command**, static ones included
  (`Mutable: false`). A dashboard can show the full picture but edit only what's
  mutable. Defaults are never stored, which is why reading the bucket alone
  can't list the settings.
- **The reserved bucket belongs to cfgx.** The app may create its own buckets
  through `DB()` but must not write inside `Bucket()`. bbolt can't enforce that,
  so it's a contract. A record that fails to decode is ignored with a warning
  (§4).
- **Runtime settings are per instance.** Each replica has its own bbolt file. A
  shared store is post-v0.1.0 work.
- **No secrets in the store** in v0.1.0, since bbolt doesn't encrypt.

---

## 8. Generated `config` subcommand

```
zp config init       write config.example.yaml as config.yaml
zp config print      effective config as YAML (secrets masked)
zp config explain    every key: value, source, default
zp config validate   load and validate; exit non-zero on problems
zp config schema     print JSON Schema
zp config env        print the env vars this binary understands
```

Each subcommand covers every key in the schema, and `validate` then requires
every command's required keys. An optional command name, as typed on the command
line, narrows it to that command's keys: `zp config validate migrate`. The
subcommands see default, file, env and store values, never other commands'
flags, and never check command-only `Args` (A9).

`explain` output:

```
KEY                VALUE         SOURCE
server.port        9090          env ZP_SERVER_PORT
database.host      db.internal   env ZP_DATABASE_HOST
database.password  ******        file /etc/zp/config.yaml:7
auth.otp.length    8             store (2026-11-02, admin@zp: "PCI review")
scheduler.workers  16            default runtime.NumCPU()
```

Startup errors list every problem with its source:

```
zp: invalid configuration (3 problems)
  database.port: 70000 is out of range 1..65535 (env ZP_DATABASE_PORT)
  scheduler.execmode: "paralel" is not one of sequential, parallel; did you mean "parallel"? (config.yaml:14:13)
  auth: token and keys are mutually exclusive (token: env ZP_AUTH_TOKEN, keys: config.yaml:22:9)
warning: unknown key "databse" in config.yaml:3:1; did you mean "database"?
warning: database.ssl_mode is deprecated, use database.sslmode (config.yaml:9:3)
```

---

## 9. `cfgx` tool

| Command | Purpose |
|---------|---------|
| `cfgx init` | scaffold a project: cobra root, settings package, schema, go:generate, `go tool` entry |
| `cfgx gen ./schema` | generate code and artifacts |
| `cfgx gen --check` | CI: fail if generated files are stale |
| `cfgx diff <old cfgx.lock.json>` | config changelog: added, removed, retyped and re-defaulted keys, flagged as breaking or not |

---

## 10. Explicitly out of scope

- Positional arguments (cobra handles them).
- Remote config sources (Consul, etcd, Vault) for v1.
- Building the cobra command tree from the schema.
- Owning resources: config describes a database, it doesn't open one. The one
  exception is the bbolt store (§7.3), which cfgx must open itself because stored
  values are part of loading.

---

## 11. Former open questions (resolved 2026-11-07)

| # | Question | Resolution |
|---|----------|------------|
| 1 | Mutable precedence | The store wins over everything (§4) |
| 2 | Invalid stored value at startup | Fall back to the next layer, warn, and show it in `explain` |
| 3 | Epochs vs migrations | Epochs only; migrations only if a real type change needs one |
| 4 | Lists of objects | Generated read-only item types with getters; after v1 |
| 5 | File layering | First file found wins; overlays later |
| 6 | TOML | Not in v1 |
| 7 | Module path | `github.com/epistemer/cfgx` |
| 8 | Test helper scope | One `Test(...) *All` |

---

## 12. Amendments

| Date | Change | Kind |
|------|--------|------|
| 2026-11-07 | §11's deferred questions resolved during brainstorming; §4 now defines precedence for mutable keys | Resolution of deferred questions |
| 2026-11-07 | Module path set to `github.com/epistemer/cfgx`; milestone notes in §3.3 and §7.2 updated to the v0.1.0 scope (Mutable in, Reloadable after) | Editorial, no interface change |
| 2026-11-07 | **A1.** bbolt is built in: `App.Store(pathKey)`, `Command.WithStore()`, generated `Store()`, `Update()` and `Close()`, and the `cfgx/store` package with its admin API (§2, §3.7, §3.8, §5.2, §7.3, §10). Raised by the needs of a settings dashboard. A global `runtime.Store` with raw `DB`/`Keys`/`Bucket` access was considered and rejected: raw writes skip validation, history and live updates; a `*bbolt.Bucket` is only valid inside its transaction; the bucket doesn't list unstored defaults; and the name clashes with Go's `runtime` package. | Amendment |
| 2026-11-07 | **A2.** Logging: cfgx logs through `log/slog`, and `BindRoot(root, cfgx.WithLogger(l))` replaces the default stderr logger (§5.2, §6.1). Apps that use logrus bridge it; the `cfgx init` scaffold does. | Amendment |
| 2026-11-07 | **A3.** The `config` subcommand covers the whole app by default, with an optional command argument (§8). | Amendment |
| 2026-11-07 | **A4.** Rules are value-based: "set" means non-zero (true for a Bool); an empty env var or flag clears a file value; exclusive rules forbid non-zero defaults; `RelevantIf` skips loading an irrelevant key (§3.3, §3.5). From [review 0001](reviews/0001-gemini-doc-review.md), F03 and F04. Chosen over keeping provenance-based rules with shadowing. | Amendment |
| 2026-11-07 | **A5.** A command's type has getters only for keys it loads: whole groups are embedded, groups reached through tags get forwarding getters (§3.8, §5.2, §5.3). Review 0001, F01. | Amendment |
| 2026-11-07 | **A6.** `.Default` and `.Example` take text for text-derived kinds; `Test()` does no I/O (§3.2, §3.3, §5.4). Review 0001, F02. | Amendment |
| 2026-11-07 | **A7.** The `App.Store` key is loaded by every command that has the store, and by the `config` subcommands (§3.7). Review 0001, F05. | Amendment |
| 2026-11-07 | **A8.** Computed methods: receiver naming rule, any signature without parameters, resolved by type-checking (§5.3). Review 0001, F06. | Amendment |
| 2026-11-07 | **A9.** `config` subcommands: command-line names; default, file, env and store values only; no command-only `Args`; corrected `explain` example (§8). Review 0001, F08. | Amendment |
| 2026-11-07 | **A10.** Secret hardening: no secret values in messages; `X` with `X_FILE` is an error; regular files with size limits; no mutable secrets; accurate wording for unexported fields (§3.2, §7.1). Review 0001, F12. | Amendment |
| 2026-11-07 | **A11.** Paths and discovery: `SecretFile` and `File` resolve like `Path`; an explicit config path must exist; unset variables skip a search path; `.yaml`, `.yml`, `.json` order, and two matches is an error (§3.2, §3.7). Review 0001, F13. | Amendment |
| 2026-11-08 | **A12.** Every generated command type has `Args() []string`, returning the positional arguments cobra parsed, so run functions keep the signature `func(ctx, *Serve) error` (§3.8, §5.2). Chosen by epistemer over adding an `args` parameter to every run function. | Amendment |
