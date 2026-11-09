// Package engine is the runtime behind generated code. Applications don't
// use it directly: the generated settings package does.
package engine

import (
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/spf13/pflag"
)

// GeneratedV1 is referenced by generated code, so code from a generator that
// doesn't match this package fails to compile instead of misbehaving.
const GeneratedV1 = 1

// Alias is an old name for a key across file, env, and flag.
type Alias struct {
	Path, Env, Flag string
}

// Key describes one setting. Generated code builds a table of them.
type Key[V any] struct {
	Path        string // file key, e.g. database.port
	Env         string // env var, e.g. ZP_DATABASE_PORT
	Flag        string // flag name, e.g. database-port
	Short       string // one-letter short flag, or ""
	Type        string // label in --help: int, string, bool
	Desc        string
	DefaultText string   // shown in --help; "" for none
	Quote       bool     // quote DefaultText in --help (strings)
	Bool        bool     // a bool flag: --x means --x=true
	Required    bool     // must be set (non-zero) after merging
	Global      bool     // in a Global group: its flag is a persistent flag on the root
	FlagOnly    bool     // flag-only: never looked up in config file or env
	Enum        []string // an enum's values, in order; listed in --help
	Aliases     []Alias
	Deprecated  string
	// RelevantIf lists keys that must all be set for this key to matter.
	// Otherwise it keeps its default, isn't validated, and setting it warns.
	RelevantIf []int

	// Set parses raw into the field of v. An empty raw sets the zero value,
	// which is how an empty env var or flag clears a value.
	Set func(v *V, raw string) error
	// Default stores the default in v; nil if the key has none.
	Default func(v *V)
	// IsSet reports whether the key's value is non-zero (true for a bool),
	// which is what "set" means for Required and rules (A4).
	IsSet func(v *V) bool
}

// App describes the whole application. Generated code declares one.
type App[V any] struct {
	EnvPrefix string // e.g. ZP; env vars are ZP_...
	// ConfigFile is the config file's basename without extension; "" means
	// the app reads no config file, and has no --config flag.
	ConfigFile  string
	SearchPaths []string // directories tried in order; $VARs are expanded
	Formats     []string // "yaml", "json"; nil means both
	Keys        []Key[V] // every key of the app
	Global      []int    // indexes of the keys in Global groups
	Rules       []Rule   // cross-key rules
}

// RuleKind is a kind of cross-key rule (interface §3.5).
type RuleKind int

// The rules a loaded configuration is checked against. RelevantIf isn't
// one: it is Key.RelevantIf, because it decides what is loaded.
const (
	AtMostOne RuleKind = iota + 1
	ExactlyOne
	AllOrNone
	Requires // Keys[0] requires Keys[1]
)

// Rule is a cross-key rule declared on a group. A command checks it only if
// it loads every key in it.
type Rule struct {
	Kind  RuleKind
	Group string // the declaring group's path; problems are reported under it
	Keys  []int
}

// configEnv is the env var that names the config file.
func (a *App[V]) configEnv() string { return a.EnvPrefix + "_CONFIG" }

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
		if len(k.Enum) > 0 {
			usage += "; one of " + strings.Join(k.Enum, ", ")
		}
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
		for _, a := range k.Aliases {
			if a.Flag == "" || fs.Lookup(a.Flag) != nil {
				continue
			}
			af := fs.VarPF(&rawValue{typ: k.Type}, a.Flag, "", "deprecated: use --"+k.Flag)
			if k.Bool {
				af.NoOptDefVal = "true"
			}
			_ = fs.MarkHidden(a.Flag)
		}
	}
}
