// Package engine is the runtime behind generated code. Applications don't
// use it directly: the generated settings package does.
package engine

import (
	"fmt"
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
	// FileEnv makes <Env>_FILE name a file holding the value, minus one
	// trailing newline (Secret). Setting both is a problem.
	FileEnv bool
	// File makes the value a path, and the key's value that file's contents
	// (SecretFile, File). DefaultText is then the default path.
	File bool
	// Resolve makes a relative path from the config file relative to the
	// file's directory (Path, A11). File keys always resolve what they read.
	Resolve bool
	// MaxFileSize caps what FileEnv and File read, in bytes.
	MaxFileSize int64
	// RelevantIf lists keys that must all be set for this key to matter.
	// Otherwise it keeps its default, isn't validated, and setting it warns.
	RelevantIf []int

	// Set parses raw into the field of v. An empty raw sets the zero value,
	// which is how an empty env var or flag clears a value. List and map
	// keys have SetItems instead.
	Set func(v *V, raw string) error
	// SetItems parses the items of a list or map key (Strings, Ints,
	// Durations: one value each; StringMap: key=value each) into the field
	// of v, replacing it. The engine makes the items: each flag occurrence,
	// an env var, or a single value in the config file is split as one CSV
	// record; a list in the config file gives them as they are, and a map
	// gives key=value items (Map keys).
	SetItems func(v *V, items []string) error
	// Map makes SetItems take key=value items, and the config file a map.
	Map bool
	// Default stores the default in v. If it is nil, a non-empty DefaultText
	// is the default's raw text, which is parsed with Set, like any layer's
	// value, only if no other layer sets the key: that is how File keys
	// read their default path, and how kinds whose value isn't a Go literal
	// (URL, Location) and paths, cleaned for the OS they run on, get theirs.
	Default func(v *V)
	// IsSet reports whether the key's value is non-zero (true for a bool),
	// which is what "set" means for Required and rules (A4).
	IsSet func(v *V) bool
	// Show is the key's value as config print and explain show it: plain
	// Go values that encode as JSON, with a secret masked, a URL's password
	// redacted and a file's contents summed up by size.
	Show func(v *V) any
	// Check runs the key's constraints, then its Validate function, on a
	// value that is set; nil if it has none.
	Check func(v *V) error
	// DefaultFrom names the function DefaultFunc calls, for --help.
	DefaultFrom string
	// Hidden leaves the flag out of --help.
	Hidden bool
	// Complete is how the flag's value completes in the shell; nil means an
	// enum's values, or else the shell's default, file names.
	Complete *Completion
	// NoFile keeps the key out of the config file (Sources): a value there
	// is ignored, with a warning.
	NoFile bool
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
	// ExampleConfig and JSONSchema are the generated example config file
	// and its JSON Schema, for the config subcommand (M6.4).
	ExampleConfig, JSONSchema string
	// Profiles are the commands' key lists by name, for the config
	// subcommand.
	Profiles map[string][]int
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
// It keeps every occurrence too: list and map keys use them all.
type rawValue struct {
	typ, raw string
	all      []string
}

func (v *rawValue) String() string     { return v.raw }
func (v *rawValue) Set(s string) error { v.raw = s; v.all = append(v.all, s); return nil }
func (v *rawValue) Type() string       { return v.typ }

// occurrences returns every value f was given, in order.
func occurrences(f *pflag.Flag) []string {
	if rv, ok := f.Value.(*rawValue); ok {
		return rv.all
	}
	return []string{f.Value.String()}
}

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
		} else if k.DefaultFrom != "" {
			usage += " (default from " + k.DefaultFrom + ")"
		}
		f := fs.VarPF(&rawValue{typ: k.Type}, k.Flag, k.Short, usage)
		if k.Bool {
			f.NoOptDefVal = "true"
		}
		f.Hidden = k.Hidden
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

// TestDefaults stores every key's default in v, for a generated Test: its
// Default, or its default text parsed with Set. A File or SecretFile key's
// default is a path, which isn't read. A default that doesn't parse, such
// as a zone the system doesn't know, panics.
func TestDefaults[V any](a *App[V], v *V) {
	for i := range a.Keys {
		k := &a.Keys[i]
		switch {
		case k.Default != nil:
			k.Default(v)
		case k.DefaultText != "" && !k.File && k.Set != nil:
			if err := k.Set(v, k.DefaultText); err != nil {
				panic(fmt.Sprintf("cfgx: default of %s: %v", k.Path, err))
			}
		}
	}
}
