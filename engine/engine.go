package engine

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/epistemer/cfgx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GeneratedV1 is referenced by generated code, so code from a generator that
// doesn't match this package fails to compile instead of misbehaving.
const GeneratedV1 = 1

// Key describes one setting. Generated code builds a table of them.
type Key[V any] struct {
	Path        string // file key, e.g. database.port
	Env         string // env var, e.g. ZP_DATABASE_PORT
	Flag        string // flag name, e.g. database-port
	Short       string // one-letter short flag, or ""
	Type        string // label in --help: int, string, bool
	Desc        string
	DefaultText string // shown in --help; "" for none
	Quote       bool   // quote DefaultText in --help (strings)
	Bool        bool   // a bool flag: --x means --x=true

	// Set parses raw into the field of v. An empty raw sets the zero value,
	// which is how an empty env var or flag clears a value.
	Set func(v *V, raw string) error
	// Default stores the default in v; nil if the key has none.
	Default func(v *V)
}

// Snapshot holds the current values. Every getter reads through it.
type Snapshot[V any] struct{ p atomic.Pointer[V] }

// Load returns the current values.
func (s *Snapshot[V]) Load() *V { return s.p.Load() }

// Store replaces the current values.
func (s *Snapshot[V]) Store(v *V) { s.p.Store(v) }

// rawValue is the pflag.Value of every flag: it records the text and nothing
// else. pflag never applies a default and never rejects a value (ADR 0003).
type rawValue struct{ typ, raw string }

func (v *rawValue) String() string     { return v.raw }
func (v *rawValue) Set(s string) error { v.raw = s; return nil }
func (v *rawValue) Type() string       { return v.typ }

// BindFlags registers the flags of the keys at idx on fs. The default goes
// into the usage text; DefValue stays empty (ADR 0003).
func BindFlags[V any](fs *pflag.FlagSet, keys []Key[V], idx []int) {
	for _, i := range idx {
		k := &keys[i]
		if k.Flag == "" {
			continue
		}
		usage := k.Desc
		if d := k.DefaultText; d != "" && !(k.Bool && d == "false") {
			if k.Quote {
				d = strconv.Quote(d)
			}
			usage += " (default " + d + ")"
		}
		f := fs.VarPF(&rawValue{typ: k.Type}, k.Flag, k.Short, usage)
		if k.Bool {
			f.NoOptDefVal = "true"
		}
	}
}

// Options adjust Load.
type Options struct {
	// LookupEnv replaces os.LookupEnv, for tests.
	LookupEnv func(string) (string, bool)
}

// Load resolves the keys at idx. For each key the highest layer present
// wins: a flag that was passed, then an env var, then the default. Only the
// winning value is parsed, so a bad value that is overridden doesn't matter.
// Every problem is reported at once, as cfgx.Errors.
func Load[V any](fs *pflag.FlagSet, keys []Key[V], idx []int, opts Options) (*V, error) {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	v := new(V)
	var errs cfgx.Errors
	for _, i := range idx {
		k := &keys[i]
		if k.Default != nil {
			k.Default(v)
		}
		var raw, source string
		var found bool
		if k.Env != "" {
			if r, ok := lookup(k.Env); ok {
				raw, source, found = r, "env "+k.Env, true
			}
		}
		if fs != nil && k.Flag != "" {
			if f := fs.Lookup(k.Flag); f != nil && f.Changed {
				raw, source, found = f.Value.String(), "flag --"+k.Flag, true
			}
		}
		if found {
			if err := k.Set(v, raw); err != nil {
				errs = append(errs, cfgx.Problem{Key: k.Path, Message: err.Error(), Source: source})
			}
		}
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(a, b int) bool { return errs[a].Key < errs[b].Key })
		return nil, errs
	}
	return v, nil
}

// Command binds the keys at idx to cmd and sets its RunE: load, build the
// command's configuration with the positional arguments, run. Configuration
// errors don't print the usage.
func Command[V, C any](cmd *cobra.Command, keys []Key[V], idx []int, build func(*V, []string) *C, run func(context.Context, *C) error) *cobra.Command {
	BindFlags(cmd.Flags(), keys, idx)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v, err := Load(cmd.Flags(), keys, idx, Options{})
		if err != nil {
			cmd.SilenceUsage = true
			return err
		}
		return run(cmd.Context(), build(v, args))
	}
	return cmd
}

// ParseString stores raw.
func ParseString(raw string, dst *string) error {
	*dst = raw
	return nil
}

// ParseInt parses a base-10 integer; empty means 0.
func ParseInt(raw string, dst *int) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, strconv.IntSize)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return fmt.Errorf("%q is out of range for an integer", raw)
		}
		return fmt.Errorf("%q is not an integer", raw)
	}
	*dst = int(n)
	return nil
}

// ParseBool parses true or false (and the other forms strconv accepts);
// empty means false.
func ParseBool(raw string, dst *bool) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		*dst = false
		return nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("%q is not a boolean; use true or false", raw)
	}
	*dst = b
	return nil
}
