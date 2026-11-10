# examples/zp

The zp example from [docs/interface.md §3.1](../../docs/interface.md): a
backend with three commands, each loading only the configuration it uses.
Its generated [docs/CONFIG.md](docs/CONFIG.md) shows what cfgx's docs look
like.

- `internal/settings/schema` is the schema; `internal/settings/zz_settings.go`
  is generated from it by `go generate ./...`, and `internal/settings/settings.go`
  holds the computed `DSN()` and `RetryPolicy()`.
- `config.example.yaml`, `config.schema.json`, `.env.example`,
  `docs/CONFIG.md` and `cfgx.lock.json` are generated too, at the module root.
- `internal/mailer`, `internal/server` and `internal/db` stand in for zp's
  libraries. Each takes a generated interface (`settings.Mailer`,
  `settings.Server`, `settings.DB`), never a config struct.
- `scope_test.go` shows that the compiler enforces each command's scope:
  `mailer.New` doesn't accept `*settings.Migrate`, and `*settings.TestEmail`
  has `AuthEmailFrom()` but not `AuthJWTPrivate()`.

Differences from §3.1:
- `auth.emailfrom` has `.GoName("EmailFrom")`, so its getter is
  `AuthEmailFrom()`, as §5.1 has it, while the file key stays `emailfrom`.
- No `.Mutable()` and no `App.Store`. The end-to-end script
  [admin.txtar](../../internal/e2e/testdata/script/admin.txtar) shows both,
  in a server with an HTTP admin endpoint.

Try it:

```bash
cd examples/zp
ZP_DATABASE_NAME=zp ZP_DATABASE_PASSWORD=secret go run . migrate --steps 2
go run . serve --help
```

The repository's tests build, vet and test this module, and fail if
`zz_settings.go` is stale (`internal/e2e/examples_test.go`).

[bench](bench) compares loading this configuration with cfgx and with
viper.
