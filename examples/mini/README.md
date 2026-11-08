# examples/mini

`mini` is the walking-skeleton example for `cfgx`.

## What it shows

- **Schema declaration** (`internal/settings/schema/schema.go`): two groups (`server` and `database`) covering `string`, `int`, and `bool` keys with and without defaults, and two command profiles (`Serve`, which uses both groups, and `Migrate`, which uses only `database`).
- **Code generation** (`internal/settings/settings.go` and `internal/settings/zz_settings.go`): `go generate ./...` runs `cfgx gen ./schema` and writes per-group interfaces (`settings.Server`, `settings.Database`), per-command config types (`*settings.Serve`, `*settings.Migrate`), and cobra binders (`settings.ServeCommand`, `settings.MigrateCommand`, `settings.BindRoot`).
- **Config files and global settings**: `mini` reads `mini.yaml`, `mini.yml` or `mini.json` from the working directory or `~/.config/mini` (or the file `--config` or `MINI_CONFIG` names). The `log` group is global, so every command gets `--log-verbose` as a persistent flag.
- **Cobra wiring and precedence** (`main.go`): values resolve in order `default < config file < env < flag`, where flags count only when explicitly passed on the command line. Unknown keys in the file and unknown `MINI_*` env vars are warnings with "did you mean" suggestions.

## How to run it

From `examples/mini`:

```sh
# Regenerate internal/settings/zz_settings.go from the schema:
go generate ./...

# Run with defaults:
go run . serve

# Override via environment variables and flags (flag beats env, env beats default):
MINI_SERVER_PORT=9090 MINI_DATABASE_URL=postgres://localhost/mini go run . serve --server-port 3000 --server-tls

# Read a config file:
printf 'server:\n  port: 9000\n' > mini.yaml && go run . serve

# Run the migrate command (only binds and loads database flags/settings):
go run . migrate --database-url postgres://localhost/mini --database-timeout 30

# Inspect generated flags and defaults:
go run . serve --help
go run . migrate --help
```
