// Package opts holds what cfgx.Option values set. It is internal so that the
// option type stays opaque: applications only call the With functions.
package opts

import "log/slog"

// Root is what BindRoot's options set.
type Root struct {
	// Logger receives cfgx's warnings; nil means the default handler, which
	// prints "warning: ..." lines on the command's stderr.
	Logger *slog.Logger
}
