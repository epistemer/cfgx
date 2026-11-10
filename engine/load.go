package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/node"
	"github.com/epistemer/cfgx/internal/storage"
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
	// LayerStore is a mutable key's stored value, which wins over the
	// rest (interface §4).
	LayerStore
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
	case LayerStore:
		return "store"
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
	// Partial skips Required and the rules, so config print and explain
	// can show a configuration that isn't complete yet. Values that don't
	// parse are still problems.
	Partial bool
	// Store opens the app's store and layers its records over the
	// mutable keys; Result.Store is then the open store, for the caller to
	// close.
	Store StoreMode
}

// StoreMode is how loading uses the store.
type StoreMode int

const (
	// NoStore leaves the store alone.
	NoStore StoreMode = iota
	// StoreReadWrite opens the store, creating its file if need be, and
	// fails if it can't.
	StoreReadWrite
)

// Result is what Load found.
type Result[V any] struct {
	Values *V // nil if loading failed
	// Origins lists, for each key in the app's table, every layer that
	// provided a value, lowest first; nil for keys that weren't loaded.
	Origins  [][]Origin
	File     string // the config file that was read; "" if none
	Warnings []Warning
	// Irrelevant marks the keys a RelevantIf condition left out.
	Irrelevant map[int]bool
	// Store is the open store, if Options.Store asked for it; the caller
	// closes it. Records are the stored records of the loaded keys, by
	// key index, and Invalid says why a stored value was ignored.
	Store   storage.Backend
	Records map[int]storage.Record
	Invalid map[int]string
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
	l := &loader[V]{a: a, res: res, partial: o.Partial}

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
	l.loaded = map[int]bool{}
	for _, i := range idx {
		l.loaded[i] = true
	}
	l.state = map[int]keyState{}
	// RelevantIf conditions the command doesn't load itself are loaded too,
	// but only to decide relevance: they aren't checked for Required and
	// take no part in rules.
	conds := conditions(l.a, idx)
	l.condOnly = map[int]bool{}
	for _, c := range conds {
		l.condOnly[c] = true
	}
	// Keys load after the keys their RelevantIf conditions name. The
	// generator rejects cycles, so every pass makes progress.
	pending := append(append([]int(nil), idx...), conds...)
	if o.Store != NoStore && a.OpenStore != nil {
		// The store's file is a key's value: load it first, then open
		// the store, so its records can be layered over the rest.
		l.loadKey(v, a.StoreKey, fs, lookup)
		pending = slices.DeleteFunc(pending, func(i int) bool { return i == a.StoreKey })
		if l.state[a.StoreKey] == loadedOK {
			b, err := a.openStore(v, false)
			if err != nil {
				return res, err
			}
			res.Store = b
			recs, bad, err := b.Records()
			if err != nil {
				b.Close()
				res.Store = nil
				return res, err
			}
			l.records, l.badRecords = recs, bad
		}
	}
	for len(pending) > 0 {
		var later []int
		for _, i := range pending {
			if l.waiting(i) {
				later = append(later, i)
				continue
			}
			l.loadKey(v, i, fs, lookup)
		}
		if len(later) == len(pending) {
			for _, i := range later {
				l.state[i] = irrelevant // unreachable with generated tables
			}
			break
		}
		pending = later
	}
	if len(l.retire) > 0 && res.Store != nil {
		if err := res.Store.Apply(l.retire); err != nil {
			l.warn(fmt.Sprintf("can't move values from older epochs to history: %v", err))
		}
	}
	res.Irrelevant = map[int]bool{}
	for i, st := range l.state {
		if st == irrelevant {
			res.Irrelevant[i] = true
		}
	}
	if !o.Partial {
		l.checkRules(v)
	}
	if len(l.errs) > 0 {
		sort.SliceStable(l.errs, func(i, j int) bool { return l.errs[i].Key < l.errs[j].Key })
		return res, l.errs
	}
	res.Values = v
	return res, nil
}

type loader[V any] struct {
	a        *App[V]
	res      *Result[V]
	file     *configFile
	errs     cfgx.Errors
	badMaps  map[string]bool // file paths already reported as not being maps
	loaded   map[int]bool    // the command's keys
	condOnly map[int]bool    // RelevantIf conditions loaded only for relevance
	state    map[int]keyState
	partial  bool // Options.Partial
	// The store's records and the ones that failed to decode, by path.
	records    map[string]storage.Record
	badRecords map[string]error
	// retire removes records from older epochs once keys have loaded.
	retire []storage.Change
}

// conditions returns the RelevantIf conditions of the keys at idx, directly
// or through other conditions, that idx doesn't contain.
func conditions[V any](a *App[V], idx []int) []int {
	in := map[int]bool{}
	for _, i := range idx {
		in[i] = true
	}
	var out []int
	queue := append([]int(nil), idx...)
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for _, c := range a.Keys[i].RelevantIf {
			if !in[c] {
				in[c] = true
				out = append(out, c)
				queue = append(queue, c)
			}
		}
	}
	return out
}

// keyState is how loading a key went.
type keyState int

const (
	loadedOK   keyState = iota + 1
	failed              // its value is bad; rules that use it are skipped
	irrelevant          // a RelevantIf condition isn't set; it counts as unset
)

// waiting reports whether a condition of key i hasn't been loaded yet.
func (l *loader[V]) waiting(i int) bool {
	for _, c := range l.a.Keys[i].RelevantIf {
		if l.state[c] == 0 {
			return true
		}
	}
	return false
}

// relevant reports whether every RelevantIf condition of key k is set, and
// if not, names the first that isn't and whether it merely failed to load.
func (l *loader[V]) relevant(v *V, k *Key[V]) (ok bool, cond string, condFailed bool) {
	for _, c := range k.RelevantIf {
		ck := &l.a.Keys[c]
		switch {
		case l.state[c] == failed:
			return false, ck.Path, true
		case l.state[c] != loadedOK || !ck.IsSet(v):
			return false, ck.Path, false
		}
	}
	return true, "", false
}

func (l *loader[V]) warn(msg string, attrs ...slog.Attr) {
	l.res.Warnings = append(l.res.Warnings, Warning{Msg: msg, Attrs: attrs})
}

func (l *loader[V]) at(p node.Pos) string {
	return fmt.Sprintf("%s:%d:%d", l.file.path, p.Line, p.Col)
}

// candidate is one layer's value for a key. bad, if set, is why the
// value can't be used; it is reported only if this layer wins. If read is
// set, raw is a path, and the value is that file's contents.
type candidate struct {
	raw    string
	origin Origin
	keyPos node.Pos
	bad    *cfgx.Problem
	read   bool
	trim   bool // remove one trailing newline from what is read
	// For list and map keys: occ holds a flag's occurrences, each split as
	// CSV like raw; items, if exact, are a config file list's or map's
	// items, used as they are.
	occ   []string
	items []string
	exact bool
}

// listItems returns the items c gives a list or map key.
func (c *candidate) listItems() ([]string, error) {
	if c.exact {
		return c.items, nil
	}
	raws := c.occ
	if raws == nil {
		raws = []string{c.raw}
	}
	var items []string
	for _, r := range raws {
		its, err := SplitCSV(r)
		if err != nil {
			return nil, err
		}
		items = append(items, its...)
	}
	return items, nil
}

// apply parses c, whose text is raw, into k's field of v.
func apply[V any](k *Key[V], v *V, c *candidate, raw string) error {
	if k.SetItems == nil {
		return k.Set(v, raw)
	}
	items, err := c.listItems()
	if err != nil {
		return err
	}
	return k.SetItems(v, items)
}

// sets reports whether c would set key k (A4). A path counts as set if it
// isn't empty: the file isn't read for this. A value with a problem, such
// as X and X_FILE both set, counts as set.
func sets[V any](k *Key[V], c *candidate) bool {
	if c.bad != nil {
		return true
	}
	if c.read {
		return c.raw != ""
	}
	return setsValue(k, c)
}

func (l *loader[V]) loadKey(v *V, i int, fs *pflag.FlagSet, lookup func(string) (string, bool)) {
	k := &l.a.Keys[i]
	var origins []Origin
	var win *candidate
	if k.Default != nil {
		k.Default(v)
		origins = append(origins, Origin{Layer: LayerDefault})
	}
	if k.Default == nil && k.DefaultText != "" {
		win = &candidate{raw: k.DefaultText, origin: Origin{Layer: LayerDefault}}
		origins = append(origins, win.origin)
	}
	if !k.FlagOnly && !k.NoFile && l.file != nil {
		if c := l.fromFile(k, k.Path); c != nil {
			win = c
			origins = append(origins, c.origin)
			for _, a := range k.Aliases {
				if a.Path != "" {
					if ac := l.fromFile(k, a.Path); ac != nil {
						where := l.at(ac.keyPos)
						l.warn(fmt.Sprintf("%s is ignored: %s is set too", a.Path, k.Path),
							slog.String("key", k.Path), slog.String("source", where))
					}
				}
			}
		} else {
			for _, a := range k.Aliases {
				if a.Path != "" {
					if ac := l.fromFile(k, a.Path); ac != nil {
						win = ac
						origins = append(origins, ac.origin)
						where := l.at(ac.keyPos)
						l.warn(fmt.Sprintf("%s is deprecated, use %s (%s)", a.Path, k.Path, where),
							slog.String("key", k.Path), slog.String("source", where))
						break
					}
				}
			}
		}
	}
	if !k.FlagOnly && k.Env != "" {
		if c := envValue(k, k.Env, lookup); c != nil {
			win = c
			origins = append(origins, win.origin)
			for _, a := range k.Aliases {
				if a.Env != "" {
					if ac := envValue(k, a.Env, lookup); ac != nil {
						l.warn(fmt.Sprintf("%s is ignored: %s is set too", ac.origin.Where, c.origin.Where),
							slog.String("key", k.Path), slog.String("source", ac.origin.String()))
					}
				}
			}
		} else {
			for _, a := range k.Aliases {
				if a.Env != "" {
					if ac := envValue(k, a.Env, lookup); ac != nil {
						win = ac
						origins = append(origins, win.origin)
						l.warn(fmt.Sprintf("%s is deprecated, use %s", ac.origin.Where, strings.Replace(ac.origin.Where, a.Env, k.Env, 1)),
							slog.String("key", k.Path), slog.String("source", win.origin.String()))
						break
					}
				}
			}
		}
	}
	if fs != nil && k.Flag != "" {
		f := fs.Lookup(k.Flag)
		if f != nil && f.Changed {
			win = &candidate{raw: f.Value.String(), occ: occurrences(f), origin: Origin{LayerFlag, "--" + k.Flag}}
			origins = append(origins, win.origin)
			for _, a := range k.Aliases {
				if a.Flag != "" {
					if af := fs.Lookup(a.Flag); af != nil && af.Changed {
						orig := Origin{LayerFlag, "--" + a.Flag}
						l.warn(fmt.Sprintf("--%s is ignored: --%s is set too", a.Flag, k.Flag),
							slog.String("key", k.Path), slog.String("source", orig.String()))
					}
				}
			}
		} else {
			for _, a := range k.Aliases {
				if a.Flag != "" {
					if af := fs.Lookup(a.Flag); af != nil && af.Changed {
						win = &candidate{raw: af.Value.String(), occ: occurrences(af), origin: Origin{LayerFlag, "--" + a.Flag}}
						origins = append(origins, win.origin)
						l.warn(fmt.Sprintf("--%s is deprecated, use --%s", a.Flag, k.Flag),
							slog.String("key", k.Path), slog.String("source", win.origin.String()))
						break
					}
				}
			}
		}
	}
	if k.Mutable && (l.records != nil || l.badRecords != nil) {
		win, origins = l.storeLayer(k, i, win, origins)
	}
	l.res.Origins[i] = origins
	if win != nil && k.File {
		win.read = true
	}
	// set is the candidate that sets the key, if any: a default parsed at
	// load time is a candidate, but not one that setting warnings are about.
	set := win
	if set != nil && set.origin.Layer == LayerDefault {
		set = nil
	}

	if ok, cond, condFailed := l.relevant(v, k); !ok {
		// Not loaded: the default stays, nothing is read or checked. A
		// default parsed at load time is applied if it parses; a File
		// key's default path isn't read.
		l.state[i] = irrelevant
		if k.Default == nil && !k.File && k.DefaultText != "" {
			_ = k.Set(v, k.DefaultText)
		}
		if set != nil && !condFailed && sets(k, set) {
			where := set.origin.String()
			l.warn(fmt.Sprintf("%s is set (%s) but ignored: it only matters when %s is set", k.Path, where, cond),
				slog.String("key", k.Path), slog.String("source", where))
		}
		return
	}
	if set != nil && k.Deprecated != "" && sets(k, set) {
		l.warn(fmt.Sprintf("%s is deprecated: %s (%s)", k.Path, k.Deprecated, set.origin.String()),
			slog.String("key", k.Path), slog.String("source", set.origin.String()))
	}
	l.state[i] = failed
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
		raw := win.raw
		if k.Resolve && raw != "" {
			raw = l.resolve(win)
		}
		if win.read && raw != "" {
			var err error
			if raw, err = l.readFile(k, win); err != nil {
				l.errs = append(l.errs, cfgx.Problem{Key: k.Path, Message: err.Error(), Source: win.origin.String()})
				return
			}
		}
		if err := apply(k, v, win, raw); err != nil {
			l.errs = append(l.errs, cfgx.Problem{Key: k.Path, Message: err.Error(), Source: win.origin.String()})
			return
		}
	}
	if k.Check != nil && k.IsSet(v) {
		if err := k.Check(v); err != nil {
			src := "default"
			if win != nil {
				src = win.origin.String()
			}
			l.errs = append(l.errs, cfgx.Problem{Key: k.Path, Message: err.Error(), Source: src})
			return
		}
	}
	if k.Required && !k.IsSet(v) && !l.condOnly[i] && !l.partial {
		if win != nil && win.read && win.raw != "" {
			l.errs = append(l.errs, cfgx.Problem{Key: k.Path, Message: "is required, but the file " + l.resolve(win) + " is empty", Source: win.origin.String()})
			return
		}
		l.errs = append(l.errs, l.required(k, win))
		return
	}
	l.state[i] = loadedOK
}

// setsValue reports whether raw would set key k (A4): an empty value or
// false doesn't. It parses into a scratch value, so the real one is
// untouched; a value that doesn't parse counts as set.
func setsValue[V any](k *Key[V], c *candidate) bool {
	scratch := new(V)
	if err := apply(k, scratch, c, c.raw); err != nil {
		return true
	}
	return k.IsSet(scratch)
}

// checkRules checks every rule whose keys the command loaded, unless one of
// them failed to load, which is already a problem. Keys that aren't
// relevant count as unset.
func (l *loader[V]) checkRules(v *V) {
	for _, r := range l.a.Rules {
		skip := false
		var set, unset []int
		for _, i := range r.Keys {
			switch {
			case !l.loaded[i] || l.state[i] == failed:
				skip = true
			case l.state[i] == loadedOK && l.a.Keys[i].IsSet(v):
				set = append(set, i)
			default:
				unset = append(unset, i)
			}
		}
		if skip {
			continue
		}
		var msg string
		switch r.Kind {
		case AtMostOne:
			if len(set) > 1 {
				if len(r.Keys) == 2 {
					msg = l.names(r, r.Keys) + " are mutually exclusive"
				} else {
					msg = "at most one of " + l.names(r, r.Keys) + " may be set"
				}
			}
		case ExactlyOne:
			switch {
			case len(set) == 0:
				msg = "one of " + l.names(r, r.Keys) + " must be set"
			case len(set) > 1:
				msg = "only one of " + l.names(r, r.Keys) + " may be set"
			}
		case AllOrNone:
			if len(set) > 0 && len(unset) > 0 {
				verb := " isn't set"
				if len(unset) > 1 {
					verb = " aren't set"
				}
				msg = l.names(r, r.Keys) + " must be set together or not at all; " + l.names(r, unset) + verb
			}
		case Requires:
			if len(set) == 1 && set[0] == r.Keys[0] {
				msg = l.name(r, r.Keys[0]) + " requires " + l.name(r, r.Keys[1])
			}
		}
		if msg == "" {
			continue
		}
		var src []string
		for _, i := range set {
			og := l.res.Origins[i]
			src = append(src, l.name(r, i)+": "+og[len(og)-1].String())
		}
		l.errs = append(l.errs, cfgx.Problem{Key: r.Group, Message: msg, Source: strings.Join(src, ", ")})
	}
}

// name is key i's path relative to the rule's group.
func (l *loader[V]) name(r Rule, i int) string {
	p := l.a.Keys[i].Path
	if strings.HasPrefix(p, r.Group+".") {
		return p[len(r.Group)+1:]
	}
	return p
}

// names lists keys as "a and b" or "a, b and c".
func (l *loader[V]) names(r Rule, keys []int) string {
	ns := make([]string, len(keys))
	for j, i := range keys {
		ns[j] = l.name(r, i)
	}
	if len(ns) == 1 {
		return ns[0]
	}
	return strings.Join(ns[:len(ns)-1], ", ") + " and " + ns[len(ns)-1]
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
	if !k.FlagOnly && k.Env != "" {
		ways = append(ways, k.Env)
		if k.FileEnv {
			ways = append(ways, k.Env+"_FILE")
		}
	}
	if !k.FlagOnly && !k.NoFile && l.a.ConfigFile != "" {
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

// fromFile finds path, one of k's names, in the config file. A null value
// counts as absent.
func (l *loader[V]) fromFile(k *Key[V], path string) *candidate {
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
			if k.SetItems != nil {
				return l.fileItems(k, path, p, origin)
			}
			switch val.Kind {
			case node.Map:
				return &candidate{origin: origin, keyPos: p.KeyPos, bad: &cfgx.Problem{Key: path, Message: "expected a single value, found a map", Source: origin.String()}}
			case node.List:
				return &candidate{origin: origin, keyPos: p.KeyPos, bad: &cfgx.Problem{Key: path, Message: "expected a single value, found a list", Source: origin.String()}}
			}
			return &candidate{raw: val.Scalar, origin: origin, keyPos: p.KeyPos}
		}
		if val.Kind != node.Map {
			prefix := strings.Join(segs[:n+1], ".")
			found := "a single value"
			if val.Kind == node.List {
				found = "a list"
			}
			return &candidate{origin: origin, keyPos: p.KeyPos, bad: &cfgx.Problem{Key: prefix, Message: "expected a map of settings, found " + found, Source: origin.String()}}
		}
		cur = val
	}
	return nil
}

// fileItems is the candidate of a list or map key whose config file value
// is p: a single value, split like an env var's; a list of single values;
// or, for a map key, a map of single values.
func (l *loader[V]) fileItems(k *Key[V], path string, p *node.Pair, origin Origin) *candidate {
	val := p.Value
	c := &candidate{origin: origin, keyPos: p.KeyPos}
	bad := func(msg string, at node.Pos) *candidate {
		o := Origin{LayerFile, l.at(at)}
		c.bad = &cfgx.Problem{Key: path, Message: msg, Source: o.String()}
		return c
	}
	want := "a list"
	if k.Map {
		want = "a map"
	}
	scalar := func(n *node.Node, what string) (string, bool) {
		switch n.Kind {
		case node.Scalar:
			return n.Scalar, true
		case node.Null:
			bad(what+" is null", n.Pos)
		case node.Map:
			bad(what+" is a map; expected a single value", n.Pos)
		default:
			bad(what+" is a list; expected a single value", n.Pos)
		}
		return "", false
	}
	switch val.Kind {
	case node.Scalar:
		c.raw = val.Scalar
		return c
	case node.List:
		if k.Map {
			return bad("expected a map, found a list", val.Pos)
		}
		c.exact, c.items = true, []string{}
		for i, it := range val.List {
			s, ok := scalar(it, fmt.Sprintf("item %d", i+1))
			if !ok {
				return c
			}
			c.items = append(c.items, s)
		}
		return c
	case node.Map:
		if !k.Map {
			return bad("expected "+want+", found a map", val.Pos)
		}
		c.exact, c.items = true, []string{}
		for _, e := range val.Map {
			if strings.Contains(e.Key, "=") {
				return bad(fmt.Sprintf("map key %q contains =", e.Key), e.KeyPos)
			}
			s, ok := scalar(e.Value, fmt.Sprintf("the value of %q", e.Key))
			if !ok {
				return c
			}
			c.items = append(c.items, e.Key+"="+s)
		}
		return c
	}
	return nil
}

// trie holds the app's key paths, to find unknown keys in the file.
type trie struct {
	kids   map[string]*trie
	leaf   bool
	noFile bool // a key that can't be set in the config file (Sources)
}

func (l *loader[V]) unknownFileKeys() {
	root := &trie{}
	addPath := func(path string, noFile bool) {
		t := root
		for _, seg := range strings.Split(path, ".") {
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
		t.noFile = t.noFile || noFile
	}
	for _, k := range l.a.Keys {
		if k.FlagOnly {
			continue
		}
		addPath(k.Path, k.NoFile)
		for _, a := range k.Aliases {
			if a.Path != "" {
				addPath(a.Path, k.NoFile)
			}
		}
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
		if kid.leaf && kid.noFile {
			where := l.at(p.KeyPos)
			l.warn(fmt.Sprintf("%s in %s is ignored: it can't be set in the config file", prefix+p.Key, where),
				slog.String("key", prefix+p.Key), slog.String("source", where))
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
		if k.FlagOnly {
			continue
		}
		add(k.Env)
		for _, a := range k.Aliases {
			add(a.Env)
		}
		if k.FileEnv {
			add(k.Env + "_FILE")
			for _, a := range k.Aliases {
				if a.Env != "" {
					add(a.Env + "_FILE")
				}
			}
		}
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

// storeLayer puts a mutable key's stored value over win, the winner of the
// other layers. A stored value that doesn't parse, or fails the key's
// checks, is ignored with a warning, and kept in the store. One that hides
// a flag or env var says so.
func (l *loader[V]) storeLayer(k *Key[V], i int, win *candidate, origins []Origin) (*candidate, []Origin) {
	next := "using the default"
	switch {
	case win == nil && k.Default == nil:
		next = "leaving it unset"
	case win != nil && win.origin.Layer != LayerDefault:
		next = "using " + win.origin.String()
	}
	if err, bad := l.badRecords[k.Path]; bad {
		l.warn(fmt.Sprintf("%s: the stored value is ignored (%v); %s", k.Path, err, next),
			slog.String("key", k.Path), slog.String("source", "store"))
		l.invalid(i, err.Error())
		return win, origins
	}
	rec, ok := l.records[k.Path]
	if !ok {
		return win, origins
	}
	// A record from an older epoch was for what the key used to mean: it
	// moves to history. One from a newer epoch is for a newer program.
	switch {
	case rec.Epoch < k.Epoch:
		l.retire = append(l.retire, storage.Change{Key: k.Path, History: storage.Record{
			Epoch: k.Epoch, UpdatedAt: time.Now().UTC(), UpdatedBy: "cfgx", Reset: true,
			Reason: fmt.Sprintf("epoch %d discards values from epoch %d", k.Epoch, rec.Epoch),
		}})
		l.warn(fmt.Sprintf("%s: the stored value %s is from epoch %d, and the key is at epoch %d now; it moves to history, %s",
			k.Path, rec.Raw, rec.Epoch, k.Epoch, next), slog.String("key", k.Path), slog.String("source", "store"))
		return win, origins
	case rec.Epoch > k.Epoch:
		l.warn(fmt.Sprintf("%s: the stored value is ignored (it is from epoch %d, newer than this program's %d); %s",
			k.Path, rec.Epoch, k.Epoch, next), slog.String("key", k.Path), slog.String("source", "store"))
		l.invalid(i, fmt.Sprintf("from epoch %d, newer than %d", rec.Epoch, k.Epoch))
		return win, origins
	}
	if l.res.Records == nil {
		l.res.Records = map[int]storage.Record{}
	}
	l.res.Records[i] = rec
	c := &candidate{raw: rec.Raw, origin: Origin{Layer: LayerStore}}
	scratch := new(V)
	err := apply(k, scratch, c, c.raw)
	switch {
	case err != nil:
	case k.Required && !k.IsSet(scratch):
		// Empty stays empty: a required key can't take it from the store.
		err = errors.New("is required, but the stored value is empty")
	case k.Check != nil && k.IsSet(scratch):
		err = k.Check(scratch)
	}
	if err != nil {
		l.warn(fmt.Sprintf("%s: the stored value is ignored (%v); %s", k.Path, err, next),
			slog.String("key", k.Path), slog.String("source", "store"))
		l.invalid(i, err.Error())
		return win, origins
	}
	if win != nil && (win.origin.Layer == LayerEnv || win.origin.Layer == LayerFlag) {
		who := rec.UpdatedAt.Format("2006-01-02")
		if rec.UpdatedBy != "" {
			who = rec.UpdatedBy + ", " + who
		}
		l.warn(fmt.Sprintf("%s: stored value %s (%s) overrides %s=%s", k.Path, rec.Raw, who, win.origin, win.raw),
			slog.String("key", k.Path), slog.String("source", "store"))
	}
	return c, append(origins, c.origin)
}

func (l *loader[V]) invalid(i int, msg string) {
	if l.res.Invalid == nil {
		l.res.Invalid = map[int]string{}
	}
	l.res.Invalid[i] = msg
}
