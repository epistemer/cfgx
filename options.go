package cfgx

import (
	"log/slog"

	"github.com/epistemer/cfgx/internal/opts"
)

// Option adjusts how generated code binds and loads configuration. Pass
// options to the generated BindRoot, or to a generated LoadX, which loads a
// command's configuration without cobra.
type Option func(*opts.Root)

// WithLogger sends cfgx's warnings, such as unknown keys in the config file,
// to l instead of printing them on the command's stderr. Warnings are logged
// at slog.LevelWarn.
func WithLogger(l *slog.Logger) Option {
	return func(r *opts.Root) { r.Logger = l }
}

// WithArgs gives LoadX the command line, without the program and command
// names: flags are parsed as the command's would be, and the arguments
// that aren't flags are what the command type's Args returns. Bind-time
// options, it has no effect on BindRoot.
func WithArgs(args ...string) Option {
	return func(r *opts.Root) { r.Args, r.ArgsSet = append([]string(nil), args...), true }
}

// WithEnv gives LoadX the environment instead of the process's: env vars,
// the variable naming the config file, and $VARs in the config file's
// search paths all come from env.
func WithEnv(env map[string]string) Option {
	return func(r *opts.Root) {
		r.Env, r.EnvSet = make(map[string]string, len(env)), true
		for k, v := range env {
			r.Env[k] = v
		}
	}
}

// WithFile gives LoadX the config file, as --config does: it must exist,
// and no search is made. A --config in WithArgs wins.
func WithFile(path string) Option {
	return func(r *opts.Root) { r.File = path }
}
