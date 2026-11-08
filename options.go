package cfgx

import (
	"log/slog"

	"github.com/epistemer/cfgx/internal/opts"
)

// Option adjusts how generated code binds and loads configuration. Pass
// options to the generated BindRoot.
type Option func(*opts.Root)

// WithLogger sends cfgx's warnings, such as unknown keys in the config file,
// to l instead of printing them on the command's stderr. Warnings are logged
// at slog.LevelWarn.
func WithLogger(l *slog.Logger) Option {
	return func(r *opts.Root) { r.Logger = l }
}
