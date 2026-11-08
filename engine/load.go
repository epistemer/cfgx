package engine

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/node"
	"github.com/epistemer/cfgx/internal/suggest"
	"github.com/spf13/pflag"
)

// Layer is a source of values, lowest first.
type Layer int

// The layers, in precedence order: each overrides the ones before it.
const (
	LayerDefault Layer = iota
	LayerFile
	LayerEnv
	LayerFlag
)

// Origin is one layer that provided a key's value, and where: a file
// position, an env var or a flag.
type Origin struct {
	Layer Layer
	Where string // "config.yaml:3:9", "ZP_DATABASE_PORT", "--database-port"; "" for the default
}

func (o Origin) String() string {
	switch o.Layer {
	case LayerDefault:
		return "default"
	case LayerEnv:
		return "env " + o.Where
	case LayerFlag:
		return "flag " + o.Where
	}
	return o.Where
}

// Warning is something wrong that doesn't stop loading, such as an unknown
// key in the config file.
type Warning struct {
	Msg   string
	Attrs []slog.Attr
}

// Options adjust Load.
type Options struct {
	// LookupEnv replaces os.LookupEnv, for tests.
	LookupEnv func(string) (string, bool)
	// Environ replaces os.Environ, for tests.
	Environ func() []string
}

// Result is what Load found.
type Result[V any] struct {
	Values *V // nil if loading failed
	// Origins lists, for each key in the app's table, every layer that
	// provided a value, lowest first; nil for keys that weren't loaded.
	Origins  [][]Origin
	File     string // the config file that was read; "" if none
	Warnings []Warning
}

// envCaseInsensitive is true where env var names are case-insensitive, so
// the scan for unknown names compares them upper-cased.
var envCaseInsensitive = runtime.GOOS == "windows"

// Load resolves the keys at idx. Each key starts at its default; then the
// config file, env vars and flags that were passed override it, in that
// order. Only the winning value is parsed, so a bad value that is overridden
// doesn't matter. Every problem is reported at once, as cfgx.Errors; a
// config file that can't be found or read is a plain error.
//
// Unknown keys in the file and unknown env vars with the app's prefix are
// warnings. They are checked against every key of the app, so a key that
// only another command uses is never unknown.
func Load[V any](a *App[V], fs *pflag.FlagSet, idx []int, o Options) (*Result[V], error) {
	lookup, environ := o.LookupEnv, o.Environ
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if environ == nil {
		environ = os.Environ
	}
	res := &Result[V]{Origins: make([][]Origin, len(a.Keys))}
	l := &loader[V]{a: a, res: res}

	if a.ConfigFile != "" {
		// A --config that was passed decides, even if empty: the env var is
		// a lower layer. An empty value means "search as usual".
		var explicit, from string
		flagSet := false
		if fs != nil {
			if f := fs.Lookup("config"); f != nil && f.Changed {
				flagSet = true
				explicit, from = f.Value.String(), "--config"
			}
		}
		if !flagSet {
			if p, ok := lookup(a.configEnv()); ok {
				explicit, from = p, a.configEnv()
			}
		}
		path, err := findConfig(a, explicit, from, lookup)
		if err != nil {
			return res, err
		}
		if path != "" {
			cf, err := readConfig(path)
			if err != nil {
				return res, err
			}
			l.file = cf
			res.File = path
			l.unknownFileKeys()
		}
	}
	l.unknownEnv(environ())

	v := new(V)
	for _, i := range idx {
		l.loadKey(v, i, fs, lookup)
	}
	if len(l.errs) > 0 {
		sort.SliceStable(l.errs, func(i, j int) bool { return l.errs[i].Key < l.errs[j].Key })
		return res, l.errs
	}
	res.Values = v
	return res, nil
}

type loader[V any] struct {
	a       *App[V]
	res     *Result[V]
	file    *configFile
	errs    cfgx.Errors
	badMaps map[string]bool // file paths already reported as not being maps
}

func (l *loader[V]) warn(msg string, attrs ...slog.Attr) {
	l.res.Warnings = append(l.res.Warnings, Warning{Msg: msg, Attrs: attrs})
}

func (l *loader[V]) at(p node.Pos) string {
	return fmt.Sprintf("%s:%d:%d", l.file.path, p.Line, p.Col)
}

// candidate is one layer's value for a key. bad, if set, is why the file's
// value can't be used; it is reported only if this layer wins.
type candidate struct {
	raw    string
	origin Origin
	bad    *cfgx.Problem
}

func (l *loader[V]) loadKey(v *V, i int, fs *pflag.FlagSet, lookup func(string) (string, bool)) {
	k := &l.a.Keys[i]
	var origins []Origin
	if k.Default != nil {
		k.Default(v)
		origins = append(origins, Origin{Layer: LayerDefault})
	}
	var win *candidate
	if l.file != nil {
		if c := l.fromFile(k.Path); c != nil {
			win = c
			origins = append(origins, c.origin)
		}
	}
	if k.Env != "" {
		if raw, ok := lookup(k.Env); ok {
			win = &candidate{raw: raw, origin: Origin{LayerEnv, k.Env}}
			origins = append(origins, win.origin)
		}
	}
	if fs != nil && k.Flag != "" {
		if f := fs.Lookup(k.Flag); f != nil && f.Changed {
			win = &candidate{raw: f.Value.String(), origin: Origin{LayerFlag, "--" + k.Flag}}
			origins = append(origins, win.origin)
		}
	}
	l.res.Origins[i] = origins

	if win != nil {
		if win.bad != nil {
			if !l.badMaps[win.bad.Key] {
				if l.badMaps == nil {
					l.badMaps = map[string]bool{}
				}
				l.badMaps[win.bad.Key] = true
				l.errs = append(l.errs, *win.bad)
			}
			return
		}
		if err := k.Set(v, win.raw); err != nil {
			l.errs = append(l.errs, cfgx.Problem{Key: k.Path, Message: err.Error(), Source: win.origin.String()})
			return
		}
	}
	if k.Required && !k.IsSet(v) {
		l.errs = append(l.errs, l.required(k, win))
	}
}

// required is the problem for a required key that isn't set.
func (l *loader[V]) required(k *Key[V], win *candidate) cfgx.Problem {
	if win != nil {
		// Never quote the value: for a secret it could be the secret.
		msg := "is required, but is empty"
		if strings.TrimSpace(win.raw) != "" {
			msg = "is required, but is set to the zero value"
		}
		return cfgx.Problem{Key: k.Path, Message: msg, Source: win.origin.String()}
	}
	var ways []string
	if k.Flag != "" {
		ways = append(ways, "--"+k.Flag)
	}
	if k.Env != "" {
		ways = append(ways, k.Env)
	}
	if l.a.ConfigFile != "" {
		ways = append(ways, k.Path+" in the config file")
	}
	msg := "is required"
	switch len(ways) {
	case 0:
	case 1:
		msg += "; set " + ways[0]
	default:
		msg += "; set " + strings.Join(ways[:len(ways)-1], ", ") + " or " + ways[len(ways)-1]
	}
	return cfgx.Problem{Key: k.Path, Message: msg}
}

// fromFile finds path in the config file. A null value counts as absent.
func (l *loader[V]) fromFile(path string) *candidate {
	segs := strings.Split(path, ".")
	cur := l.file.root
	for n, seg := range segs {
		p := cur.Get(seg)
		if p == nil || p.Value.Kind == node.Null {
			return nil
		}
		val := p.Value
		origin := Origin{LayerFile, l.at(val.Pos)}
		if n == len(segs)-1 {
			switch val.Kind {
			case node.Map:
				return &candidate{origin: origin, bad: &cfgx.Problem{Key: path, Message: "expected a single value, found a map", Source: origin.String()}}
			case node.List:
				return &candidate{origin: origin, bad: &cfgx.Problem{Key: path, Message: "expected a single value, found a list", Source: origin.String()}}
			}
			return &candidate{raw: val.Scalar, origin: origin}
		}
		if val.Kind != node.Map {
			prefix := strings.Join(segs[:n+1], ".")
			found := "a single value"
			if val.Kind == node.List {
				found = "a list"
			}
			return &candidate{origin: origin, bad: &cfgx.Problem{Key: prefix, Message: "expected a map of settings, found " + found, Source: origin.String()}}
		}
		cur = val
	}
	return nil
}

// trie holds the app's key paths, to find unknown keys in the file.
type trie struct {
	kids map[string]*trie
	leaf bool
}

func (l *loader[V]) unknownFileKeys() {
	root := &trie{}
	for _, k := range l.a.Keys {
		t := root
		for _, seg := range strings.Split(k.Path, ".") {
			if t.kids == nil {
				t.kids = map[string]*trie{}
			}
			next := t.kids[seg]
			if next == nil {
				next = &trie{}
				t.kids[seg] = next
			}
			t = next
		}
		t.leaf = true
	}
	l.walkUnknown(root, l.file.root, "")
}

func (l *loader[V]) walkUnknown(t *trie, n *node.Node, prefix string) {
	for _, p := range n.Map {
		kid := t.kids[p.Key]
		if kid == nil {
			names := make([]string, 0, len(t.kids))
			for name := range t.kids {
				names = append(names, name)
			}
			sort.Strings(names)
			where := l.at(p.KeyPos)
			msg := fmt.Sprintf("unknown key %q in %s", prefix+p.Key, where)
			if s, ok := suggest.Closest(p.Key, names); ok {
				msg += fmt.Sprintf("; did you mean %q?", prefix+s)
			}
			l.warn(msg, slog.String("key", prefix+p.Key), slog.String("source", where))
			continue
		}
		if !kid.leaf && p.Value.Kind == node.Map {
			l.walkUnknown(kid, p.Value, prefix+p.Key+".")
		}
	}
}

// unknownEnv warns about env vars that start with the app's prefix but
// match no key.
func (l *loader[V]) unknownEnv(environ []string) {
	fold := func(s string) string { return s }
	if envCaseInsensitive {
		fold = strings.ToUpper
	}
	known := map[string]bool{}
	var names []string
	add := func(name string) {
		if name != "" && !known[fold(name)] {
			known[fold(name)] = true
			names = append(names, name)
		}
	}
	for _, k := range l.a.Keys {
		add(k.Env)
	}
	if l.a.ConfigFile != "" {
		add(l.a.configEnv())
	}
	sort.Strings(names)
	folded := make([]string, len(names))
	for i, n := range names {
		folded[i] = fold(n)
	}
	prefix := fold(l.a.EnvPrefix + "_")
	var unknown []string
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if name == "" || !strings.HasPrefix(fold(name), prefix) || known[fold(name)] {
			continue
		}
		unknown = append(unknown, name)
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		msg := "unknown environment variable " + name
		if s, ok := suggest.Closest(fold(name), folded); ok {
			for i, f := range folded {
				if f == s {
					s = names[i]
					break
				}
			}
			msg += "; did you mean " + s + "?"
		}
		l.warn(msg, slog.String("env", name))
	}
}
