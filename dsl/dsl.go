package dsl

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strconv"

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
	pos        ir.Pos
}

func (k *keyBase) base() *keyBase { return k }
func (k *keyBase) member()        {}

// Tags adds the key to tag interfaces.
func (k *StringKey) Tags(tags ...*TagDef) *StringKey {
	k.tags = append(k.tags, tags...)
	return k
}

// Required makes loading fail when the key isn't set: when its value is
// empty or zero (false for a Bool) after every source is merged. Only the
// commands that load the key check it.
func (k *StringKey) Required() *StringKey {
	k.required = true
	return k
}

// StringKey is a string setting. Create it with String.
type StringKey struct{ keyBase }

// String declares a string key.
func String(name, desc string) *StringKey {
	return &StringKey{keyBase{name: name, desc: desc, kind: ir.KindString, pos: caller(1)}}
}

// Default sets the key's default.
func (k *StringKey) Default(v string) *StringKey {
	k.def = &v
	return k
}

// Tags adds the key to tag interfaces.
func (k *IntKey) Tags(tags ...*TagDef) *IntKey {
	k.tags = append(k.tags, tags...)
	return k
}

// Required makes loading fail when the key isn't set: when its value is
// empty or zero (false for a Bool) after every source is merged. Only the
// commands that load the key check it.
func (k *IntKey) Required() *IntKey {
	k.required = true
	return k
}

// IntKey is an int setting. Create it with Int.
type IntKey struct{ keyBase }

// Int declares an int key.
func Int(name, desc string) *IntKey {
	return &IntKey{keyBase{name: name, desc: desc, kind: ir.KindInt, pos: caller(1)}}
}

// Default sets the key's default.
func (k *IntKey) Default(v int) *IntKey {
	s := strconv.Itoa(v)
	k.def = &s
	return k
}

// Tags adds the key to tag interfaces.
func (k *BoolKey) Tags(tags ...*TagDef) *BoolKey {
	k.tags = append(k.tags, tags...)
	return k
}

// Required makes loading fail when the key isn't set: when its value is
// empty or zero (false for a Bool) after every source is merged. Only the
// commands that load the key check it.
func (k *BoolKey) Required() *BoolKey {
	k.required = true
	return k
}

// BoolKey is a bool setting. Create it with Bool.
type BoolKey struct{ keyBase }

// Bool declares a bool key.
func Bool(name, desc string) *BoolKey {
	return &BoolKey{keyBase{name: name, desc: desc, kind: ir.KindBool, pos: caller(1)}}
}

// Default sets the key's default.
func (k *BoolKey) Default(v bool) *BoolKey {
	s := strconv.FormatBool(v)
	k.def = &s
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
		ig := ir.Group{Name: g.name, Desc: g.desc, GoName: g.goName, FlagPrefix: g.flagPrefix, Tags: gt, Pos: g.pos}
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
			ig.Keys = append(ig.Keys, ir.Key{Name: k.name, Desc: k.desc, Kind: k.kind, Default: k.def, Required: k.required, Tags: kt, Pos: k.pos})
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
			ic.Args = append(ic.Args, ir.Key{
				Name:     k.name,
				Desc:     k.desc,
				Kind:     k.kind,
				Default:  k.def,
				Required: k.required,
				Tags:     kt,
				Pos:      k.pos,
			})
		}
		out.Commands = append(out.Commands, ic)
	}
	for _, t := range tags {
		out.Tags = append(out.Tags, ir.Tag{Name: t.name, Desc: t.desc, Pos: t.pos})
	}
	return ir.Encode(w, out)
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
