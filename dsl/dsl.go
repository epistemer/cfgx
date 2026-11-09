package dsl

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/ir"
)

// The registry. Apps and commands register themselves when the schema
// package initializes, so the generator needs no variable names.
var (
	apps     []*AppDef
	commands []*CommandDef
	tags     []*TagDef
)

func caller(skip int) ir.Pos {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return ir.Pos{}
	}
	return ir.Pos{File: file, Line: line}
}

// AppDef is the root of a schema. Create it with App.
type AppDef struct {
	name, desc, envPrefix string
	groups                []*GroupDef
	configFile            string
	searchPaths           []string
	formats               []Format
	global                []*GroupDef
	initialisms           []string
	pos                   ir.Pos
}

// App declares the application and its groups. A schema declares exactly one.
func App(name, desc string, groups ...*GroupDef) *AppDef {
	a := &AppDef{name: name, desc: desc, groups: groups, pos: caller(1)}
	apps = append(apps, a)
	return a
}

// EnvPrefix sets the prefix of every env var. It defaults to the upper-cased
// app name.
func (a *AppDef) EnvPrefix(p string) *AppDef {
	a.envPrefix = p
	return a
}

// ConfigFile makes the app read a config file named basename plus .yaml,
// .yml or .json. The search paths are tried in order, and the first one with
// a match wins; $VAR and ${VAR} are expanded, and a path that uses an unset
// or empty variable is skipped. Two matches in one directory are an error,
// and finding nothing is fine. --config, or the env var <PREFIX>_CONFIG,
// names the file instead, which must then exist.
func (a *AppDef) ConfigFile(basename string, searchPaths ...string) *AppDef {
	a.configFile = basename
	a.searchPaths = append([]string(nil), searchPaths...)
	return a
}

// Format is a config file format.
type Format string

// The config file formats.
const (
	YAML Format = "yaml" // .yaml and .yml
	JSON Format = "json" // .json
)

// Formats restricts the accepted config file formats. The default is both.
func (a *AppDef) Formats(formats ...Format) *AppDef {
	a.formats = append(a.formats, formats...)
	return a
}

// Global gives every command the keys of groups, and binds their flags as
// persistent flags on the root command. The groups must also be passed to
// App.
func (a *AppDef) Global(groups ...*GroupDef) *AppDef {
	a.global = append(a.global, groups...)
	return a
}

// Initialisms adds words that Go names write in capitals, such as "OTP" or
// "JWT", to the built-in list (ID, URL, TLS, HTTP, DB, ...).
func (a *AppDef) Initialisms(words ...string) *AppDef {
	a.initialisms = append(a.initialisms, words...)
	return a
}

// GroupDef is a named set of keys and nested groups. Create it with Group.
type GroupDef struct {
	name, desc string
	keys       []KeyDef
	groups     []*GroupDef
	goName     string
	flagPrefix *string
	tags       []*TagDef
	rules      []*RuleDef
	computed   []string
	pos        ir.Pos
}

// Member is what a group contains: keys and nested groups.
type Member interface {
	member()
}

// Group declares a group of keys and nested groups. Every group becomes a Go
// interface; a nested group's interface is embedded in its parent's.
func Group(name, desc string, members ...Member) *GroupDef {
	g := &GroupDef{name: name, desc: desc, pos: caller(1)}
	for _, m := range members {
		switch m := m.(type) {
		case *GroupDef:
			g.groups = append(g.groups, m) // a nil *GroupDef is reported by Export
		case KeyDef:
			if isNil(m) {
				m = nil // a nil *StringKey would panic in base(); report it instead
			}
			g.keys = append(g.keys, m)
		default:
			// A nil key or group: Export reports it with the group's position.
			g.keys = append(g.keys, nil)
		}
	}
	return g
}

func (g *GroupDef) member() {}

// GoName sets the group's segment of Go names: dsl.Group("database", ...).GoName("DB")
// gives the interface DB and getters such as DBHost(). File keys, env vars and
// flags still use the group's name.
func (g *GroupDef) GoName(name string) *GroupDef {
	g.goName = name
	return g
}

// FlagPrefix sets the prefix of the flags of the group's keys. It defaults to
// the group's path (--server-port); FlagPrefix("") gives --port. Nested groups
// start from their parent's prefix.
func (g *GroupDef) FlagPrefix(prefix string) *GroupDef {
	g.flagPrefix = &prefix
	return g
}

// Tags adds every key in the group, nested groups included, to tag
// interfaces.
func (g *GroupDef) Tags(tags ...*TagDef) *GroupDef {
	g.tags = append(g.tags, tags...)
	return g
}

// Computed adds hand-written methods to the group's interface. Write each in
// the settings package, on the group's type: dsl.Group("database", ...) with
// GoName("DB") gives *dbGroup, and a nested Retry group in Scheduler gives
// *schedulerRetryGroup. Any method without parameters works; the generator
// reads its exact signature by type-checking the package.
func (g *GroupDef) Computed(methods ...string) *GroupDef {
	g.computed = append(g.computed, methods...)
	return g
}

// Rules adds cross-key constraints. They are value-based (A4): a key is set
// when its value is non-zero, and a Bool when it is true. A command checks a
// rule only if it loads every key in it.
func (g *GroupDef) Rules(rules ...*RuleDef) *GroupDef {
	g.rules = append(g.rules, rules...)
	return g
}

// RuleDef is a cross-key constraint. Create it with AtMostOne, ExactlyOne,
// AllOrNone, Requires or RelevantIf, and add it with Group.Rules.
type RuleDef struct {
	kind ir.RuleKind
	keys []KeyDef
	pos  ir.Pos
}

func rule(kind ir.RuleKind, keys ...KeyDef) *RuleDef {
	return &RuleDef{kind: kind, keys: keys, pos: caller(2)}
}

// AtMostOne allows at most one of keys to be set. Its keys can't have
// non-zero defaults.
func AtMostOne(keys ...KeyDef) *RuleDef { return rule(ir.RuleAtMostOne, keys...) }

// ExactlyOne requires exactly one of keys to be set. Its keys can't have
// non-zero defaults.
func ExactlyOne(keys ...KeyDef) *RuleDef { return rule(ir.RuleExactlyOne, keys...) }

// AllOrNone requires keys to be set together or not at all. Its keys can't
// have non-zero defaults.
func AllOrNone(keys ...KeyDef) *RuleDef { return rule(ir.RuleAllOrNone, keys...) }

// Requires makes b required when a is set.
func Requires(a, b KeyDef) *RuleDef { return rule(ir.RuleRequires, a, b) }

// RelevantIf makes a matter only when b is set. When b isn't, a is neither
// loaded nor validated, and setting it warns. Every command that loads a
// also loads b.
func RelevantIf(a, b KeyDef) *RuleDef { return rule(ir.RuleRelevantIf, a, b) }

// TagDef is a cross-cutting set of keys. Create it with Tag.
type TagDef struct {
	name, desc string
	pos        ir.Pos
}

// Tag declares a tag. name is the Go interface it produces, such as Mailer;
// it lists the getters of every key tagged with it, from any group. A command
// that uses a tag gets getters for just those keys.
func Tag(name, desc string) *TagDef {
	t := &TagDef{name: name, desc: desc, pos: caller(1)}
	tags = append(tags, t)
	return t
}

func (t *TagDef) usable() {}

// KeyDef is implemented by every key constructor's result.
type KeyDef interface {
	Member
	base() *keyBase
}

type keyBase struct {
	name, desc string
	kind       ir.Kind
	def        *string
	required   bool
	tags       []*TagDef
	aliases    []string
	deprecated string
	typ        *ir.Type       // enums: the values' Go type
	enum       []ir.EnumValue // enums: the values
	problems   []string
	short      rune
	flag, env  string
	fileKey    string
	goName     string
	sources    []string
	hidden     bool
	example    *string
	checks     []ir.Check
	validate   *ir.FuncRef
	defFunc    *ir.FuncRef
	// finish checks what can only be checked once every modifier has run,
	// such as a default against Range; each returns a problem or "".
	finish []func() string
	pos    ir.Pos
}

func (k *keyBase) base() *keyBase { return k }
func (k *keyBase) member()        {}

// key holds what every key type shares: the modifiers every kind has, each
// returning the concrete key type K so calls chain. T is the Go type a
// default is given in; text turns one into the canonical text the IR
// records. G is the getter's type, which Validate and DefaultFunc see.
// Schemas never name it: they call String, Int and the other constructors.
type key[K any, T any, G any] struct {
	keyBase
	self K
	text func(T) string
	dv   *T // the default, for checks
}

func (k *key[K, T, G]) init(self K, name, desc string, kind ir.Kind, text func(T) string, pos ir.Pos) {
	k.keyBase = keyBase{name: name, desc: desc, kind: kind, pos: pos}
	k.self, k.text = self, text
}

// Default sets the key's default.
func (k *key[K, T, G]) Default(v T) K {
	s := k.text(v)
	k.def, k.dv = &s, &v
	return k.self
}

// Required makes loading fail when the key isn't set: when its value is the
// zero value (empty, 0, false) after every source is merged. Only the
// commands that load the key check it.
func (k *key[K, T, G]) Required() K {
	k.required = true
	return k.self
}

// Tags adds the key to tag interfaces.
func (k *key[K, T, G]) Tags(tags ...*TagDef) K {
	k.tags = append(k.tags, tags...)
	return k.self
}

// Alias keeps old names working: each old path's file key, env var and flag
// are still accepted, with a deprecation warning. Renames only.
func (k *key[K, T, G]) Alias(oldPaths ...string) K {
	k.aliases = append(k.aliases, oldPaths...)
	return k.self
}

// Deprecated makes setting the key print a warning with msg.
func (k *key[K, T, G]) Deprecated(msg string) K {
	k.deprecated = msg
	return k.self
}

// StringKey is a string setting. Create it with String.
type StringKey struct {
	key[*StringKey, string, string]
}

// String declares a string key.
func String(name, desc string) *StringKey {
	k := &StringKey{}
	k.init(k, name, desc, ir.KindString, func(v string) string { return v }, caller(1))
	return k
}

// IntKey is an int setting. Create it with Int.
type IntKey struct{ ordered[*IntKey, int, int] }

// Int declares an int key.
func Int(name, desc string) *IntKey {
	k := &IntKey{}
	k.init(k, name, desc, ir.KindInt, strconv.Itoa, caller(1))
	return k
}

// BoolKey is a bool setting. Create it with Bool.
type BoolKey struct{ key[*BoolKey, bool, bool] }

// Bool declares a bool key; its flag is --x, or --x=false.
func Bool(name, desc string) *BoolKey {
	k := &BoolKey{}
	k.init(k, name, desc, ir.KindBool, strconv.FormatBool, caller(1))
	return k
}

// Int64Key is an int64 setting. Create it with Int64.
type Int64Key struct {
	ordered[*Int64Key, int64, int64]
}

// Int64 declares an int64 key.
func Int64(name, desc string) *Int64Key {
	k := &Int64Key{}
	k.init(k, name, desc, ir.KindInt64, func(v int64) string { return strconv.FormatInt(v, 10) }, caller(1))
	return k
}

// UintKey is a uint setting. Create it with Uint.
type UintKey struct{ ordered[*UintKey, uint, uint] }

// Uint declares a uint key.
func Uint(name, desc string) *UintKey {
	k := &UintKey{}
	k.init(k, name, desc, ir.KindUint, func(v uint) string { return strconv.FormatUint(uint64(v), 10) }, caller(1))
	return k
}

// FloatKey is a float64 setting. Create it with Float.
type FloatKey struct {
	ordered[*FloatKey, float64, float64]
}

// Float declares a float64 key.
func Float(name, desc string) *FloatKey {
	k := &FloatKey{}
	k.init(k, name, desc, ir.KindFloat, func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }, caller(1))
	return k
}

// DurationKey is a time.Duration setting. Create it with Duration.
type DurationKey struct {
	ordered[*DurationKey, time.Duration, time.Duration]
}

// Duration declares a time.Duration key. Values need a unit, such as 90s or
// 5m: a bare number other than 0 is rejected.
func Duration(name, desc string) *DurationKey {
	k := &DurationKey{}
	k.init(k, name, desc, ir.KindDuration, time.Duration.String, caller(1))
	return k
}

// ByteSizeKey is a size in bytes, an int64. Create it with ByteSize.
type ByteSizeKey struct {
	ordered[*ByteSizeKey, int64, int64]
}

// ByteSize declares a size key. Values are bytes, or a whole number with a
// unit: 512KB (1000s) or 1MiB (1024s).
func ByteSize(name, desc string) *ByteSizeKey {
	k := &ByteSizeKey{}
	k.init(k, name, desc, ir.KindByteSize, func(v int64) string { return strconv.FormatInt(v, 10) }, caller(1))
	return k
}

// SecretKey is a secret string setting, read as a cfgx.Secret. Create it
// with Secret.
type SecretKey struct {
	key[*SecretKey, string, cfgx.Secret]
}

// Secret declares a secret key. It has no flag, since a flag's value shows
// in ps and shell history. Besides <ENV>, <ENV>_FILE may name a file that
// holds the value, minus one trailing newline; setting both is an error.
// Messages never show the value.
func Secret(name, desc string) *SecretKey {
	k := &SecretKey{}
	k.init(k, name, desc, ir.KindSecret, func(v string) string { return v }, caller(1))
	return k
}

// SecretFileKey is a secret read from a file. Create it with SecretFile.
type SecretFileKey struct {
	key[*SecretFileKey, string, cfgx.Secret]
}

// SecretFile declares a key whose setting is a path, and whose value, a
// cfgx.Secret, is that file's contents, exactly. The file is read while
// loading, only by commands that load the key and only when it is relevant
// (RelevantIf). It must be a regular file of at most 1 MiB. A relative path
// in the config file is relative to the config file's directory; elsewhere,
// to the working directory. Default takes a path.
func SecretFile(name, desc string) *SecretFileKey {
	k := &SecretFileKey{}
	k.init(k, name, desc, ir.KindSecretFile, func(v string) string { return v }, caller(1))
	return k
}

// PathKey is a path setting, a string. Create it with Path.
type PathKey struct{ key[*PathKey, string, string] }

// Path declares a path key. Its value is cleaned, and a relative path in
// the config file is made relative to the config file's directory; a
// relative path from an env var, a flag or the default stays relative to
// the working directory. The file isn't opened. Default takes a path.
func Path(name, desc string) *PathKey {
	k := &PathKey{}
	k.init(k, name, desc, ir.KindPath, func(v string) string { return v }, caller(1))
	return k
}

// FileKey is a file's contents, a []byte. Create it with File.
type FileKey struct{ key[*FileKey, string, []byte] }

// File declares a key whose setting is a path, and whose value is that
// file's contents, exactly. The file is read while loading, only by
// commands that load the key and only when it is relevant (RelevantIf). It
// must be a regular file of at most 16 MiB. Paths resolve as for Path; an
// empty file counts as not set. Default takes a path.
func File(name, desc string) *FileKey {
	k := &FileKey{}
	k.init(k, name, desc, ir.KindFile, func(v string) string { return v }, caller(1))
	return k
}

// URLKey is a URL setting, a *url.URL. Create it with URL.
type URLKey struct{ key[*URLKey, string, *url.URL] }

// URL declares a URL key. Values must be absolute URLs, with a scheme such
// as https://; unset, the value is nil. Messages show a URL with its
// password redacted. Default takes the URL's text.
func URL(name, desc string) *URLKey {
	k := &URLKey{}
	k.init(k, name, desc, ir.KindURL, func(v string) string { return v }, caller(1))
	return k
}

// LocationKey is a time zone setting, a *time.Location. Create it with
// Location.
type LocationKey struct {
	key[*LocationKey, string, *time.Location]
}

// Location declares a time zone key. Values are IANA names, such as
// Europe/Paris, or UTC or Local; unset, the value is nil. Zones are loaded
// from the system's zoneinfo, so an image without it, such as one built
// FROM scratch, needs import _ "time/tzdata" in main. Default takes the
// zone's name, and is loaded when the key is.
func Location(name, desc string) *LocationKey {
	k := &LocationKey{}
	k.init(k, name, desc, ir.KindLocation, func(v string) string { return v }, caller(1))
	return k
}

// Usable is something a command can use: a group, nested or not, or a tag.
type Usable interface {
	usable()
}

func (g *GroupDef) usable() {}

// CommandDef is a config profile. Create it with Command.
type CommandDef struct {
	name string
	uses []Usable
	args []KeyDef
	pos  ir.Pos
}

// Command declares a config profile. goName names the generated type
// (Serve gives settings.Serve and settings.ServeCommand).
func Command(goName string) *CommandDef {
	c := &CommandDef{name: goName, pos: caller(1)}
	commands = append(commands, c)
	return c
}

// Uses adds groups to the command.
func (c *CommandDef) Uses(items ...Usable) *CommandDef {
	c.uses = append(c.uses, items...)
	return c
}

// Args adds flag-only arguments to the command.
func (c *CommandDef) Args(keys ...KeyDef) *CommandDef {
	c.args = append(c.args, keys...)
	return c
}

// Export writes the IR of everything the schema declared. The generator's
// temporary main calls it; schemas don't.
func Export(w io.Writer) error {
	switch len(apps) {
	case 0:
		return errors.New("the schema declares no dsl.App")
	case 1:
	default:
		return fmt.Errorf("the schema declares %d dsl.App values (at %s and %s); declare exactly one", len(apps), apps[0].pos, apps[1].pos)
	}
	a := apps[0]
	out := &ir.App{Version: ir.Version, Name: a.name, Desc: a.desc, EnvPrefix: a.envPrefix,
		ConfigFile: a.configFile, SearchPaths: a.searchPaths, Pos: a.pos}
	for _, f := range a.formats {
		out.Formats = append(out.Formats, string(f))
	}
	out.Initialisms = a.initialisms
	// paths maps every group reachable from the App to its dotted path.
	paths := map[*GroupDef]string{}
	keyPaths := map[*keyBase]string{}
	keyGroups := map[*keyBase]string{}
	type pending struct {
		r     *RuleDef
		group string
	}
	var rules []pending
	var export func(g *GroupDef, parent string) (ir.Group, error)
	export = func(g *GroupDef, parent string) (ir.Group, error) {
		path := g.name
		if parent != "" {
			path = parent + "." + g.name
		}
		if prev, dup := paths[g]; dup {
			return ir.Group{}, fmt.Errorf("%s: group %q is used twice, as %s and as %s; a group has one place in the tree", g.pos, g.name, prev, path)
		}
		paths[g] = path
		gt, err := tagNames(g.tags, g.pos)
		if err != nil {
			return ir.Group{}, err
		}
		ig := ir.Group{Name: g.name, Desc: g.desc, GoName: g.goName, FlagPrefix: g.flagPrefix, Tags: gt, Computed: g.computed, Pos: g.pos}
		for _, kd := range g.keys {
			if kd == nil {
				return ir.Group{}, fmt.Errorf("%s: group %q lists a nil key or group", g.pos, g.name)
			}
			k := kd.base()
			keyPaths[k] = path + "." + k.name
			keyGroups[k] = path
			kt, err := tagNames(k.tags, k.pos)
			if err != nil {
				return ir.Group{}, err
			}
			ig.Keys = append(ig.Keys, k.export(kt))
		}
		for _, r := range g.rules {
			if r == nil {
				return ir.Group{}, fmt.Errorf("%s: group %q lists a nil rule", g.pos, g.name)
			}
			rules = append(rules, pending{r, path})
		}
		for _, c := range g.groups {
			if c == nil {
				return ir.Group{}, fmt.Errorf("%s: group %q lists a nil group", g.pos, g.name)
			}
			ic, err := export(c, path)
			if err != nil {
				return ir.Group{}, err
			}
			ig.Groups = append(ig.Groups, ic)
		}
		return ig, nil
	}
	for _, g := range a.groups {
		if g == nil {
			return fmt.Errorf("%s: dsl.App lists a nil group", a.pos)
		}
		ig, err := export(g, "")
		if err != nil {
			return err
		}
		out.Groups = append(out.Groups, ig)
	}
	for _, p := range rules {
		rl := ir.Rule{Kind: p.r.kind, Group: p.group, Pos: p.r.pos}
		for _, kd := range p.r.keys {
			if isNil(kd) {
				return fmt.Errorf("%s: %s lists a nil key", p.r.pos, p.r.kind)
			}
			path, ok := keyPaths[kd.base()]
			if !ok {
				return fmt.Errorf("%s: %s lists key %q, which isn't in a group passed to dsl.App", p.r.pos, p.r.kind, kd.base().name)
			}
			rl.Keys = append(rl.Keys, path)
		}
		out.Rules = append(out.Rules, rl)
	}
	for _, g := range a.global {
		if g == nil {
			return fmt.Errorf("%s: dsl.App.Global lists a nil group", a.pos)
		}
		path, ok := paths[g]
		if !ok {
			out.GlobalOrphans = append(out.GlobalOrphans, ir.Ref{Name: g.name, Pos: g.pos})
			continue
		}
		out.Global = append(out.Global, path)
	}
	argCommands := map[*keyBase]string{}
	for _, c := range commands {
		ic := ir.Command{Name: c.name, Pos: c.pos, Uses: []string{}}
		for _, u := range c.uses {
			if t, ok := u.(*TagDef); ok && t != nil {
				ic.Tags = append(ic.Tags, t.name)
				continue
			}
			g, ok := u.(*GroupDef)
			if !ok || g == nil {
				return fmt.Errorf("%s: command %s uses a nil group or tag", c.pos, c.name)
			}
			path, ok := paths[g]
			if !ok {
				ic.Orphans = append(ic.Orphans, ir.Ref{Name: g.name, Pos: g.pos})
				continue
			}
			ic.Uses = append(ic.Uses, path)
		}
		for _, kd := range c.args {
			if isNil(kd) {
				return fmt.Errorf("%s: command %s lists a nil key in Args", c.pos, c.name)
			}
			k := kd.base()
			if group, ok := keyGroups[k]; ok {
				return fmt.Errorf("%s: key %q in command %s is also in group %q", c.pos, k.name, c.name, group)
			}
			if prevCmd, ok := argCommands[k]; ok {
				if prevCmd == c.name {
					return fmt.Errorf("%s: command %s lists key %q in Args twice", c.pos, c.name, k.name)
				}
				return fmt.Errorf("%s: key %q is used in Args of both %s and %s", c.pos, k.name, prevCmd, c.name)
			}
			argCommands[k] = c.name
			kt, err := tagNames(k.tags, k.pos)
			if err != nil {
				return err
			}
			ic.Args = append(ic.Args, k.export(kt))
		}
		out.Commands = append(out.Commands, ic)
	}
	for _, t := range tags {
		out.Tags = append(out.Tags, ir.Tag{Name: t.name, Desc: t.desc, Pos: t.pos})
	}
	return ir.Encode(w, out)
}

// export returns the key's IR, with its tags already resolved.
func (k *keyBase) export(tags []string) ir.Key {
	var aliases []string
	if len(k.aliases) > 0 {
		aliases = append([]string(nil), k.aliases...)
	}
	problems := append([]string(nil), k.problems...)
	for _, f := range k.finish {
		if p := f(); p != "" {
			problems = append(problems, p)
		}
	}
	if k.def != nil && k.defFunc != nil {
		problems = append(problems, "has both Default and DefaultFunc; keep one")
	}
	return ir.Key{
		Name:        k.name,
		Desc:        k.desc,
		Kind:        k.kind,
		Default:     k.def,
		Required:    k.required,
		Tags:        tags,
		Aliases:     aliases,
		Deprecated:  k.deprecated,
		Type:        k.typ,
		Enum:        k.enum,
		Short:       shortText(k.short),
		Flag:        k.flag,
		Env:         k.env,
		FileKey:     k.fileKey,
		GoName:      k.goName,
		Sources:     k.sources,
		Hidden:      k.hidden,
		Example:     k.example,
		Checks:      k.checks,
		Validate:    k.validate,
		DefaultFunc: k.defFunc,
		Problems:    problems,
		Pos:         k.pos,
	}
}

// isNil reports whether kd is nil or holds a nil pointer, whose base()
// would panic.
func isNil(kd KeyDef) bool {
	if kd == nil {
		return true
	}
	v := reflect.ValueOf(kd)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// tagNames names tags, rejecting nil ones.
func tagNames(ts []*TagDef, pos ir.Pos) ([]string, error) {
	var out []string
	for _, t := range ts {
		if t == nil {
			return nil, fmt.Errorf("%s: Tags lists a nil tag", pos)
		}
		out = append(out, t.name)
	}
	return out, nil
}
