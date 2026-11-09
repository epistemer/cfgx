// Package lockdiff compares two cfgx.lock.json files and classifies each
// change by what it means for the people who configure the app (M6.3).
package lockdiff

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/epistemer/cfgx/internal/ir"
)

// Class is how a change affects existing configurations.
type Class int

// The classes, most serious first.
const (
	// Breaking: a configuration that worked may now fail, or be ignored.
	Breaking Class = iota
	// Behaviour: it still loads, but an app may act differently.
	Behaviour
	// Safe: nothing that worked changes.
	Safe
)

func (c Class) String() string {
	return [...]string{"Breaking", "Behaviour changes", "Safe"}[c]
}

// Change is one difference between two lock files.
type Change struct {
	Class Class
	Key   string // a key's path, or "Command --flag" for an argument
	Msg   string
}

func (c Change) String() string { return c.Key + ": " + c.Msg }

// Parse reads a lock file. It accepts lock files of any IR version: fields
// it doesn't know are ignored.
func Parse(data []byte) (*ir.App, error) {
	var a ir.App
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("not a cfgx lock file: %v", err)
	}
	return &a, nil
}

// keys flattens an app's keys by path; command arguments are "Command --flag".
func keys(a *ir.App) map[string]ir.Key {
	out := map[string]ir.Key{}
	var walk func(prefix string, g ir.Group)
	walk = func(prefix string, g ir.Group) {
		path := g.Name
		if prefix != "" {
			path = prefix + "." + g.Name
		}
		for _, k := range g.Keys {
			out[path+"."+k.Name] = k
		}
		for _, c := range g.Groups {
			walk(path, c)
		}
	}
	for _, g := range a.Groups {
		walk("", g)
	}
	for _, c := range a.Commands {
		for _, k := range c.Args {
			// By declared name, so a Flag override that changes is a
			// change to the same argument.
			out[c.Name+" --"+strings.NewReplacer(".", "-", "_", "-").Replace(k.Name)] = k
		}
	}
	return out
}

// text shows a default: quoted, or "no default".
func text(p *string) string {
	if p == nil {
		return "no default"
	}
	return strconv.Quote(*p)
}

func sameDefault(a, b *string) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

func enumTexts(k ir.Key) []string {
	out := make([]string, len(k.Enum))
	for i, e := range k.Enum {
		out[i] = e.Text
	}
	return out
}

func typeName(k ir.Key) string {
	if k.Type != nil {
		if k.Type.Path != "" {
			return string(k.Kind) + " " + k.Type.Path + "." + k.Type.Name
		}
		return string(k.Kind) + " " + k.Type.Name
	}
	return string(k.Kind)
}

func sources(k ir.Key) []string {
	if k.Sources == nil {
		return []string{"env", "file", "flag"}
	}
	s := slices.Clone(k.Sources)
	sort.Strings(s)
	return s
}

// Diff lists the changes from old to new, most serious first, then by key.
func Diff(old, new *ir.App) []Change {
	var out []Change
	add := func(c Class, key, format string, args ...any) {
		out = append(out, Change{c, key, fmt.Sprintf(format, args...)})
	}
	if old.EnvPrefix != new.EnvPrefix {
		add(Breaking, "app", "env prefix changed from %s to %s, so every env var is renamed", old.EnvPrefix, new.EnvPrefix)
	}
	if old.ConfigFile != new.ConfigFile {
		add(Breaking, "app", "config file name changed from %q to %q", old.ConfigFile, new.ConfigFile)
	}
	if !slices.Equal(old.SearchPaths, new.SearchPaths) {
		add(Behaviour, "app", "config file search paths changed from %v to %v", old.SearchPaths, new.SearchPaths)
	}
	ok, nk := keys(old), keys(new)
	// A key whose old path is now an alias of another was renamed.
	renamed := map[string]string{}
	for path, k := range nk {
		for _, al := range k.Aliases {
			renamed[al] = path
		}
	}
	matched := map[string]bool{} // new paths an old key maps to
	for path, o := range ok {
		n, still := nk[path]
		if still {
			matched[path] = true
		} else if to, r := renamed[path]; r {
			// Compare with the key it became; report under the old path.
			add(Behaviour, path, "renamed to %s; the old name still works, with a warning", to)
			n, matched[to] = nk[to], true
		} else {
			add(Breaking, path, "removed")
			continue
		}
		// F3: an old name dropped stops working.
		for _, al := range o.Aliases {
			if !slices.Contains(n.Aliases, al) {
				if _, now := nk[al]; !now {
					add(Breaking, path, "old name %s no longer works", al)
				}
			}
		}
		if typeName(o) != typeName(n) {
			add(Breaking, path, "type changed from %s to %s", typeName(o), typeName(n))
			continue
		}
		oe, ne := enumTexts(o), enumTexts(n)
		for _, v := range oe {
			if !slices.Contains(ne, v) {
				add(Breaking, path, "value %q removed", v)
			}
		}
		for _, v := range ne {
			if !slices.Contains(oe, v) {
				add(Safe, path, "value %q added", v)
			}
		}
		switch {
		case !o.Required && n.Required:
			add(Breaking, path, "now required")
		case o.Required && !n.Required:
			add(Safe, path, "no longer required")
		}
		for _, f := range []struct{ what, o, n string }{
			{"env var", o.Env, n.Env}, {"flag", o.Flag, n.Flag}, {"config file key", o.FileKey, n.FileKey}, {"short flag", o.Short, n.Short},
		} {
			switch {
			case f.o == f.n:
			case f.what == "short flag" && f.o == "":
				add(Safe, path, "short flag -%s added", f.n)
			default:
				add(Breaking, path, "%s override changed from %q to %q", f.what, f.o, f.n)
			}
		}
		oldSrc, newSrc := sources(o), sources(n)
		for _, s := range oldSrc {
			if !slices.Contains(newSrc, s) {
				add(Breaking, path, "can no longer come from the %s", map[string]string{"env": "environment", "file": "config file", "flag": "command line"}[s])
			}
		}
		if !sameDefault(o.Default, n.Default) {
			add(Behaviour, path, "default changed from %s to %s", text(o.Default), text(n.Default))
		}
		if (o.DefaultFunc == nil) != (n.DefaultFunc == nil) || o.DefaultFunc != nil && *o.DefaultFunc != *n.DefaultFunc {
			add(Behaviour, path, "default function changed from %s to %s", funcName(o.DefaultFunc), funcName(n.DefaultFunc))
		}
		if !slices.Equal(checkTexts(o), checkTexts(n)) {
			add(Behaviour, path, "constraints changed from %s to %s", list(checkTexts(o)), list(checkTexts(n)))
		}
		if (o.Validate == nil) != (n.Validate == nil) || o.Validate != nil && *o.Validate != *n.Validate {
			add(Behaviour, path, "validation changed from %s to %s", funcName(o.Validate), funcName(n.Validate))
		}
		if o.Deprecated == "" && n.Deprecated != "" {
			add(Behaviour, path, "deprecated: %s", n.Deprecated)
		}
		if o.Desc != n.Desc {
			add(Safe, path, "description changed")
		}
	}
	for path, n := range nk {
		if matched[path] {
			continue
		}
		if n.Required {
			add(Breaking, path, "added, and required")
		} else {
			add(Safe, path, "added")
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Msg < out[j].Msg
	})
	return out
}

func funcName(f *ir.FuncRef) string {
	if f == nil {
		return "none"
	}
	return f.Path + "." + f.Name
}

func checkTexts(k ir.Key) []string {
	out := make([]string, len(k.Checks))
	for i, c := range k.Checks {
		out[i] = c.Kind + "(" + strings.Join(c.Args, ", ") + ")"
	}
	return out
}

func list(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, " ")
}

// Report writes the changes under one heading per class.
func Report(changes []Change) string {
	if len(changes) == 0 {
		return "No changes.\n"
	}
	var b strings.Builder
	last := Class(-1)
	for _, c := range changes {
		if c.Class != last {
			if last != -1 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s:\n", c.Class)
			last = c.Class
		}
		fmt.Fprintf(&b, "  %s\n", c)
	}
	return b.String()
}
