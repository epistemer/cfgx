package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/opts"
	"github.com/spf13/cobra"
)

// roots maps each root command passed to BindRoot to its options. Binders
// find them at run time through cmd.Root(), so there is no global logger and
// several roots can coexist in tests.
var roots sync.Map // *cobra.Command → *opts.Root

// BindRoot prepares the root command: it records the options, and adds the
// --config flag and the flags of Global keys as persistent flags.
func BindRoot[V any](root *cobra.Command, a *App[V], options ...cfgx.Option) {
	r := &opts.Root{}
	for _, o := range options {
		if o != nil {
			o(r)
		}
	}
	roots.Store(root, r)
	if a.ConfigFile != "" {
		root.PersistentFlags().Var(&rawValue{typ: "path"}, "config", a.configUsage())
		var exts []string
		for _, f := range a.Formats {
			if f == "yaml" {
				exts = append(exts, "yaml", "yml")
			} else {
				exts = append(exts, f)
			}
		}
		if len(exts) == 0 {
			exts = []string{"yaml", "yml", "json"}
		}
		_ = root.RegisterFlagCompletionFunc("config", fileExts(exts))
	}
	BindFlags(root.PersistentFlags(), a.Keys, a.Global)
	bindCompletions(root, a.Keys, a.Global)
}

// configUsage describes --config, with where the search looks.
func (a *App[V]) configUsage() string {
	u := "config file; overrides " + a.configEnv()
	if len(a.SearchPaths) == 0 {
		return u
	}
	return fmt.Sprintf("%s (default: %s%s in %s)", u, a.ConfigFile, a.accepted(), orList(a.SearchPaths))
}

func orList(s []string) string {
	if len(s) == 1 {
		return s[0]
	}
	return strings.Join(s[:len(s)-1], ", ") + " or " + s[len(s)-1]
}

// Command binds the keys at idx to cmd and sets its RunE: load, build the
// command's configuration with the positional arguments, run with
// cmd.Context(). A configuration problem prints the problems, without the
// usage text, and RunE returns the error, so Execute does too.
func Command[V, C any](cmd *cobra.Command, a *App[V], idx []int, build func(*V, []string) *C, run func(context.Context, *C) error) *cobra.Command {
	var local []int
	for _, i := range append(append([]int(nil), idx...), conditions(a, idx)...) {
		if !a.Keys[i].Global {
			local = append(local, i)
		}
	}
	BindFlags(cmd.Flags(), a.Keys, local)
	bindCompletions(cmd, a.Keys, local)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		r, bound := roots.Load(cmd.Root())
		if !bound && (a.ConfigFile != "" || len(a.Global) > 0) {
			cmd.SilenceUsage = true
			return fmt.Errorf("cfgx: BindRoot wasn't called on the root command %q, so --config and the global flags are missing", cmd.Root().Name())
		}
		var ro *opts.Root
		if bound {
			ro = r.(*opts.Root)
		}
		res, err := Load(a, cmd.Flags(), idx, Options{})
		logger := loggerFor(ro, cmd.ErrOrStderr())
		if err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", cmd.Root().Name(), err)
			logWarnings(cmd.Context(), logger, res.Warnings)
			return err
		}
		logWarnings(cmd.Context(), logger, res.Warnings)
		return run(cmd.Context(), build(res.Values, args))
	}
	return cmd
}

func loggerFor(r *opts.Root, stderr io.Writer) *slog.Logger {
	if r != nil && r.Logger != nil {
		return r.Logger
	}
	return slog.New(&warnHandler{w: stderr})
}

func logWarnings(ctx context.Context, l *slog.Logger, ws []Warning) {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, w := range ws {
		l.LogAttrs(ctx, slog.LevelWarn, w.Msg, w.Attrs...)
	}
}

// warnHandler is the default slog handler: "warning: <message>" lines, with
// no time and no attributes (A2).
type warnHandler struct {
	mu sync.Mutex
	w  io.Writer
}

func (h *warnHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelWarn }

func (h *warnHandler) Handle(_ context.Context, r slog.Record) error {
	prefix := "warning: "
	if r.Level >= slog.LevelError {
		prefix = "error: "
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, prefix+r.Message+"\n")
	return err
}

func (h *warnHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *warnHandler) WithGroup(string) slog.Handler      { return h }
