package dsl

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"

	"github.com/epistemer/cfgx/internal/ir"
)

// The registry. Apps and commands register themselves when the schema
// package initializes, so the generator needs no variable names.
var (
	apps     []*AppDef
	commands []*CommandDef
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

// GroupDef is a named set of keys. Create it with Group.
type GroupDef struct {
	name, desc string
	keys       []KeyDef
	pos        ir.Pos
}

// Group declares a group of keys.
func Group(name, desc string, keys ...KeyDef) *GroupDef {
	return &GroupDef{name: name, desc: desc, keys: keys, pos: caller(1)}
}

// KeyDef is implemented by every key constructor's result.
type KeyDef interface {
	base() *keyBase
}

type keyBase struct {
	name, desc string
	kind       ir.Kind
	def        *string
	required   bool
	pos        ir.Pos
}

func (k *keyBase) base() *keyBase { return k }

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

// Usable is something a command can use: a group. Tags join in M5.
type Usable interface {
	usableName() string
}

func (g *GroupDef) usableName() string { return g.name }

// CommandDef is a config profile. Create it with Command.
type CommandDef struct {
	name string
	uses []Usable
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
	inApp := map[*GroupDef]bool{}
	for _, g := range a.groups {
		if g == nil {
			return fmt.Errorf("%s: dsl.App lists a nil group", a.pos)
		}
		inApp[g] = true
		ig := ir.Group{Name: g.name, Desc: g.desc, Pos: g.pos}
		for _, kd := range g.keys {
			if kd == nil {
				return fmt.Errorf("%s: group %q lists a nil key", g.pos, g.name)
			}
			k := kd.base()
			ig.Keys = append(ig.Keys, ir.Key{Name: k.name, Desc: k.desc, Kind: k.kind, Default: k.def, Required: k.required, Pos: k.pos})
		}
		out.Groups = append(out.Groups, ig)
	}
	for _, g := range a.global {
		if g == nil {
			return fmt.Errorf("%s: dsl.App.Global lists a nil group", a.pos)
		}
		if !inApp[g] {
			out.GlobalOrphans = append(out.GlobalOrphans, ir.Ref{Name: g.name, Pos: g.pos})
			continue
		}
		out.Global = append(out.Global, g.name)
	}
	for _, c := range commands {
		ic := ir.Command{Name: c.name, Pos: c.pos, Uses: []string{}}
		for _, u := range c.uses {
			g, ok := u.(*GroupDef)
			if !ok || g == nil {
				return fmt.Errorf("%s: command %s uses a nil group", c.pos, c.name)
			}
			if !inApp[g] {
				ic.Orphans = append(ic.Orphans, ir.Ref{Name: g.name, Pos: g.pos})
				continue
			}
			ic.Uses = append(ic.Uses, g.usableName())
		}
		out.Commands = append(out.Commands, ic)
	}
	return ir.Encode(w, out)
}
