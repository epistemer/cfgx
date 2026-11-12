# cfgx

Schema-first, type-safe configuration for Go command-line apps built on
[cobra](https://github.com/spf13/cobra). You declare every setting once, in
a Go schema. `go generate` turns it into typed getters, interfaces, flags,
env vars, config-file keys, validation, docs and example files.

> **Status: v0.1.** It works, it's tested on Linux, macOS and Windows, and
> the [examples](examples) run. The API may still change before v1.0.0.

- **One declaration per setting.** Its getter, flag, env var, config-file
  key, default, validation and docs all come from one line of the schema.
- **Each command gets only its own settings.** `migrate` can't read the SMTP
  password, and the compiler stops you from passing its configuration to
  a mailer.
- **Libraries take interfaces, not config structs.** A mailer depends on
  `settings.Mailer`, a generated interface of exactly the getters it needs.
- **Errors at the right time.** A typo in the schema fails `go generate`.
  A bad value fails at startup with every problem listed at once. A
  misspelled env var or file key gets a warning that suggests the right
  name.
- **Secrets stay secret.** A `cfgx.Secret` prints `******` through fmt,
  JSON, YAML, slog and reflection.
- **Settings that change at runtime.** Mutable keys are kept in a built-in
  [bbolt](https://github.com/etcd-io/bbolt) store, with history, change
  notifications and an admin API for dashboards.

## Quickstart

You need Go 1.26 or later. `cfgx init` creates a cobra app with a settings
package, in an empty directory:

```bash
mkdir hello && cd hello
go run github.com/epistemer/cfgx/cmd/cfgx@latest init example.com/hello
./hello serve
```

`hello serve` listens on `localhost:8080`, and `curl localhost:8080`
answers `hello from hello`. Stop it with Ctrl+C. The schema is in
`internal/settings/schema/schema.go`.

**Add a setting.** In the schema's `Server` group, add the `greeting` line:

```go
var Server = dsl.Group("server", "HTTP server",
	dsl.String("addr", "Address to listen on").Default("localhost:8080"),
	dsl.String("greeting", "What the server says").Default("hello"), // new
	dsl.Duration("shutdown_timeout", "How long to wait for requests to finish when stopping").Default(10*time.Second),
)
```

**Generate the code**, which adds a getter, `ServerGreeting()`:

```bash
go generate ./...
```

**Use it.** In `serve.go`, the handler for `/` prints a fixed string.
Make it print the setting instead:

```go
mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, s.ServerGreeting()) // was: fmt.Fprintln(w, "hello from hello")
})
```

**Set it** with a flag (`--server-greeting`), an env var
(`HELLO_SERVER_GREETING`) or the config file (`server.greeting` in
`hello.yaml`). Flags win over env vars, which win over the file:

```bash
go build .
printf 'server:\n  greeting: from the file\n' > hello.yaml
HELLO_SERVER_GREETING="from env" ./hello serve
```

Now `curl localhost:8080` answers `from env`. Stop the server and run it
with `--server-greeting "from a flag"` to see the flag win.

**See where each value came from**, without starting the server:

```console
$ HELLO_SERVER_GREETING="from env" ./hello config explain
KEY                      VALUE           SOURCE                     DEFAULT
log.level                info            default                    info
log.format               text            default                    text
server.addr              localhost:8080  default                    localhost:8080
server.greeting          from env        env HELLO_SERVER_GREETING  hello
server.shutdown_timeout  10s             default                    10s
```

A misspelled env var or file key gets a warning that suggests the right
name, such as `unknown environment variable HELLO_SERVER_GRETING; did you
mean HELLO_SERVER_GREETING?`

`go generate` also rewrote `hello.example.yaml`, `hello.schema.json`,
`.env.example`, `docs/CONFIG.md` and `cfgx.lock.json`, so the docs can't
fall behind the code.

## A larger schema

[examples/zp](examples/zp) is a backend with three commands. Its generated
[docs/CONFIG.md](examples/zp/docs/CONFIG.md) shows what the docs look like.
An excerpt of its schema:

```go
var Mailer = dsl.Tag("Mailer", "Everything the mailer needs")

var DB = dsl.Group("database", "Database connection",
	dsl.String("host", "Database server host").Default("localhost"),
	dsl.Int("port", "Database server port").Default(5432).Range(1, 65535),
	dsl.String("name", "Database name").Required(),
	dsl.Secret("password", "Database password").Required(),
	dsl.Enum("sslmode", "SSL mode", "disable", "require", "verify-full").
		Default("disable").Alias("database.ssl_mode"),
).GoName("DB").Computed("DSN")

var SMTP = dsl.Group("smtp", "Outgoing mail",
	dsl.String("host", "SMTP server host").Required(),
	dsl.Int("port", "SMTP server port").Default(587),
	dsl.Secret("password", "SMTP password"),
).Tags(Mailer)

var App = dsl.App("zp", "ZaynPay backend", Log, DB, SMTP, Auth, Server, Scheduler, Storage).
	EnvPrefix("ZP").
	ConfigFile("config", ".", "$HOME/.zp", "/etc/zp").
	Global(Log)

var (
	Serve     = dsl.Command("Serve").Uses(DB, SMTP, Auth, Server, Scheduler)
	Migrate   = dsl.Command("Migrate").Uses(DB).Args(dsl.Int("steps", "Number of migrations; 0 means all").Min(0))
	TestEmail = dsl.Command("TestEmail").Uses(Mailer)
)
```

Each command becomes a type with exactly its own getters, and a binder for
its cobra command:

```go
root.AddCommand(
	settings.ServeCommand(&cobra.Command{Use: "serve"}, func(ctx context.Context, s *settings.Serve) error {
		m := mailer.New(s) // mailer.New takes a settings.Mailer
		...
	}),
	settings.MigrateCommand(&cobra.Command{Use: "migrate"}, func(ctx context.Context, m *settings.Migrate) error {
		// m.SMTPHost() doesn't exist, and mailer.New(m) doesn't compile.
		...
	}),
	settings.ConfigCommand(), // config init, print, explain, validate, schema, env
)
```

The key types are `Bool`, `Int`, `Int64`, `Uint`, `Float`, `String`,
`Duration`, `ByteSize`, `URL`, `Location`, `Path`, `File`, `Secret`,
`SecretFile`, `Strings`, `Ints`, `Durations`, `StringMap`, `Enum` and
`Text`. Text covers any type that implements `encoding.TextUnmarshaler`.
Rules such as `AtMostOne`, `Requires` and `RelevantIf` relate keys to each
other. The [dsl reference](https://pkg.go.dev/github.com/epistemer/cfgx/dsl)
lists every modifier.

## Settings that change at runtime

Mark a key `Mutable` and name the store file with `App.Store`. A command
that loads a mutable key can then update it while it runs:

```go
var bolt = dsl.Path("bolt", "Store file").Default("app.db")

var Rate = dsl.Group("rate", "Rate limits",
	dsl.Int("limit", "Requests per second").Default(100).Min(1).Mutable(),
)

var App = dsl.App("app", "My app", Rate, dsl.Group("storage", "Storage", bolt)).Store(bolt)
```

The generated code then has `RateLimit()`, `SetRateLimit` and
`KeyRateLimit`:

```go
s.OnChange(func(c settings.Change) {
	if c.Has(settings.KeyRateLimit) {
		limiter.SetLimit(s.RateLimit())
	}
})
err := s.Update(ctx, func(u *settings.Update) error {
	u.SetRateLimit(250)
	return nil
}, cfgx.By("admin@example.com"), cfgx.Reason("launch traffic"))
```

Writes are validated against the whole configuration, stored with their
history in one transaction, and survive restarts. A stored value wins over
flags, env vars and the file, and `config explain` says who set it and
when. `s.Store()` is an admin API over every setting, for a settings page
or an HTTP endpoint: see
[package store](https://pkg.go.dev/github.com/epistemer/cfgx/store).

For a dashboard in more than one language, add `.I18n()` to the App.
`cfgx gen` then also writes translation keys of
[epistemer/i18n](https://github.com/epistemer/i18n) for the label of every
group, key and enum value: `settings.L10nRateLimit`, or
`settings.L10n(k.Key).T(ctx)` for each key `Store().Keys()` lists. The
labels' English text is the schema's descriptions.

## Testing

The generated `Test` returns every getter and interface with defaults,
changed as you like, without reading files, env vars or flags:

```go
all := settings.Test(func(v *settings.Values) { v.SMTP.Host = "localhost" })
m := mailer.New(all)
```

`settings.LoadServe(ctx, cfgx.WithArgs(...), cfgx.WithEnv(...), cfgx.WithFile(...))`
loads a command's configuration without cobra.

## Performance

Measured against viper on the same inputs, on a 2015 laptop (Intel Core
i5-5300U) with Go 1.27.3. [examples/zp/bench](examples/zp/bench) has the code,
what each benchmark does, and how to run them.

| | cfgx | viper |
|---|---|---|
| Load zp's `serve` configuration, 39 keys | 0.31 ms, 869 allocs | 0.56 ms, 1145 allocs |
| Load 100 keys | 0.46 ms, 1701 allocs | 1.85 ms, 2606 allocs |

`go generate` (`cfgx gen`) on zp's schema takes 1.4 s with a warm Go build
cache. With a cold cache it takes 20 s, about what a cold `go build` of the
app takes (18 s).

## Documentation

- Package docs: [cfgx](https://pkg.go.dev/github.com/epistemer/cfgx),
  [dsl](https://pkg.go.dev/github.com/epistemer/cfgx/dsl),
  [store](https://pkg.go.dev/github.com/epistemer/cfgx/store).
- [docs/interface.md](docs/interface.md) is the full design: every
  construct, what it generates and why.
- [docs/implementation.md](docs/implementation.md) is how it's built, and
  [docs/adr](docs/adr) has the decisions.
- [examples/mini](examples/mini) is the smallest app;
  [examples/zp](examples/zp) is a fuller one.

## License

MIT. See [LICENSE](LICENSE).
