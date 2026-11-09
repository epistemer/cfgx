package gen

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/epistemer/cfgx/internal/ir"
)

// FuncModel is a function the generated code calls: a Validate or
// DefaultFunc reference (ADR 0004).
type FuncModel struct {
	Path, Name string
	Alias      string // the import's name in the generated file
	// Self is true for a function in the generated package itself, which
	// is called without an import.
	Self bool
	pos  ir.Pos
}

// Expr is how the generated code names the function.
func (f *FuncModel) Expr() string {
	if f.Self {
		return f.Name
	}
	return f.Alias + "." + f.Name
}

// PatternModel is a compiled Pattern, a package-level variable.
type PatternModel struct{ Var, Re string }

// pkgRef is a package the generated code imports for a key, whose imports
// ResolveRefs checks.
type pkgRef struct {
	path, what string
	pos        ir.Pos
	fn         *FuncModel // nil for a type's package
	km         *KeyModel
}

// guessAlias is an import name for path before its package's name is known:
// the last element, up to its first dot, made an identifier.
func guessAlias(p string) string {
	base := path.Base(p)
	if i := strings.IndexByte(base, '.'); i > 0 {
		base = base[:i]
	}
	base = strings.Map(func(r rune) rune {
		if r == '-' || r == '.' {
			return '_'
		}
		return r
	}, base)
	if !token.IsIdentifier(base) || base == "_" {
		return "pkg"
	}
	return base
}

// keyExtras fills in km's checks and function references from k, after
// kindInfo. Problems go to bad, at the line the schema passed them.
func (m *Model) keyExtras(km *KeyModel, k ir.Key, imp *importer, bad func(ir.Pos, string, ...any)) {
	spec := kinds[k.Kind]
	for _, c := range k.Checks {
		switch c.Kind {
		case ir.CheckMin, ir.CheckMax:
			if spec.canon == nil || !orderedKinds[k.Kind] {
				bad(k.Pos, "%s: Min, Max and Range need a number or duration key", km.Path)
				continue
			}
			text, lit, err := spec.canon(c.Args[0])
			if err != nil {
				bad(k.Pos, "%s: bound %q: %v", km.Path, c.Args[0], err)
				continue
			}
			fn := "AtLeast"
			if c.Kind == ir.CheckMax {
				fn = "AtMost"
			}
			km.Checks = append(km.Checks, fmt.Sprintf("engine.%s(v.%s, %s, %q)", fn, km.Field, lit, text))
		case ir.CheckLen:
			km.Checks = append(km.Checks, fmt.Sprintf("engine.Length(v.%s, %s, %s)", km.Field, c.Args[0], c.Args[1]))
		case ir.CheckPattern:
			re := imp.name("regexp", "regexp")
			pm := PatternModel{Var: fmt.Sprintf("pattern%s%d", km.Field, len(km.Patterns)+1), Re: fmt.Sprintf("%s.MustCompile(%q)", re, `^(?:`+c.Args[0]+`)$`)}
			km.Patterns = append(km.Patterns, pm)
			km.Checks = append(km.Checks, fmt.Sprintf("engine.Match(v.%s, %s, %q)", km.Field, pm.Var, c.Args[0]))
		default:
			bad(k.Pos, "%s: unknown check %q (a cfgx bug)", km.Path, c.Kind)
		}
	}
	ref := func(r *ir.FuncRef, what string) *FuncModel {
		switch {
		case r.Problem != "":
			bad(r.Pos, "%s: %s: %s", km.Path, what, r.Problem)
			return nil
		case r.Path == "main":
			bad(r.Pos, "%s: %s: %s.%s is in package main, which the generated code can't import", km.Path, what, r.Path, r.Name)
			return nil
		case r.Path == imp.schema:
			bad(r.Pos, "%s: %s: %s is in the schema package, which the generated code can't import; move it to another package", km.Path, what, r.Name)
			return nil
		}
		f := &FuncModel{Path: r.Path, Name: r.Name, Alias: imp.name(r.Path, ""), pos: r.Pos}
		m.refs = append(m.refs, pkgRef{path: r.Path, what: what + " function " + r.Name, pos: r.Pos, fn: f, km: km})
		return f
	}
	if k.Validate != nil {
		km.Validate = ref(k.Validate, "Validate")
	}
	if k.DefaultFunc != nil {
		if f := ref(k.DefaultFunc, "DefaultFunc"); f != nil {
			km.defaultFunc = f
			km.HasDefault, km.DefaultText = true, ""
			km.DefaultLit = f.Expr() + "()"
			km.DefaultFrom = path.Base(f.Path) + "." + f.Name
		}
	}
	if t := k.Type; t != nil && t.Path != "" {
		m.refs = append(m.refs, pkgRef{path: t.Path, what: "type " + t.Pkg + "." + t.Name, pos: k.Pos, km: km})
	}
}

// orderedKinds are the kinds Min, Max and Range apply to.
var orderedKinds = map[ir.Kind]bool{
	ir.KindInt: true, ir.KindInt64: true, ir.KindUint: true, ir.KindFloat: true,
	ir.KindDuration: true, ir.KindByteSize: true,
}

// dslPath is the package generated code must never import, directly or not.
const dslPath = "github.com/epistemer/cfgx/dsl"

// ResolveRefs checks the packages the generated code imports for keys:
// functions' and enum and Text types' (ADR 0004), with one go list call in
// dir, the generated package's directory. A package must not be main or
// import cfgx/dsl, and the generated package must be allowed to import it
// under Go's internal/ rule. A function in the generated package itself is
// called without an import.
func ResolveRefs(ctx context.Context, m *Model, dir string) error {
	if len(m.refs) == 0 {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Match lists the command-line patterns a package matched: "." marks
	// the generated package, whatever symlinks its directory goes through.
	args := []string{"list", "-e", "-f", "{{join .Match \" \"}}\t{{.ImportPath}}\t{{.Name}}\t{{join .Deps \" \"}}", "."}
	seen := map[string]bool{}
	for _, r := range m.refs {
		if !seen[r.path] {
			seen[r.path] = true
			args = append(args, r.path)
		}
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = abs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking the packages the schema refers to: go list: %v\n%s", err, stderr.Bytes())
	}
	type pkgInfo struct {
		name string
		deps []string
	}
	pkgs := map[string]pkgInfo{}
	self := ""
	// Lines aren't trimmed: a package without dependencies ends in a tab.
	for _, line := range strings.Split(strings.TrimRight(string(out), "\r\n"), "\n") {
		f := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 4)
		if len(f) < 4 {
			continue
		}
		if slices.Contains(strings.Fields(f[0]), ".") {
			self = f[1]
		}
		pkgs[f[1]] = pkgInfo{name: f[2], deps: strings.Fields(f[3])}
	}
	if self == "" {
		return fmt.Errorf("checking the packages the schema refers to: go list didn't report the package in %s", abs)
	}
	var probs Problems
	selfRef := false
	for _, r := range m.refs {
		p := pkgs[r.path]
		switch {
		case r.path == self && r.fn == nil:
			probs = append(probs, Problem{r.pos, fmt.Sprintf("%s: %s is declared in the generated package %s, which can't import itself; declare it in another package", r.km.Path, r.what, self)})
			continue
		case r.path == self:
			r.fn.Self, selfRef = true, true
			if r.km.defaultFunc == r.fn {
				r.km.DefaultLit = r.fn.Expr() + "()"
			}
			continue
		case p.name == "":
			probs = append(probs, Problem{r.pos, fmt.Sprintf("%s: %s is in %s, which go list can't load from %s", r.km.Path, r.what, r.path, abs)})
			continue
		case p.name == "main":
			probs = append(probs, Problem{r.pos, fmt.Sprintf("%s: %s is in package main (%s), which the generated code can't import", r.km.Path, r.what, r.path)})
			continue
		case r.path == dslPath || contains(p.deps, dslPath):
			probs = append(probs, Problem{r.pos, fmt.Sprintf("%s: %s is in %s, which imports %s; generated code must never import it, so move the %s to a package that doesn't", r.km.Path, r.what, r.path, dslPath, strings.Fields(r.what)[0])})
			continue
		case !internalOK(self, r.path):
			probs = append(probs, Problem{r.pos, fmt.Sprintf("%s: %s is in %s, an internal package the generated package %s can't import", r.km.Path, r.what, r.path, self)})
			continue
		}
		m.imp.pkgs[r.path] = p.name
		if r.fn != nil && r.km.defaultFunc == r.fn {
			r.km.DefaultFrom = p.name + "." + r.fn.Name
		}
	}
	if len(probs) > 0 {
		return probs
	}
	if selfRef {
		// Only functions can be in the generated package (types there are
		// rejected above), and they need no import.
		delete(m.imp.used, self)
	}
	m.Imports = m.imp.imports(len(m.Commands) > 0)
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// internalOK reports whether from may import to under Go's internal/ rule:
// a path with an internal element is importable only from within the tree
// rooted at the parent of its last internal element, as the go command
// checks it.
func internalOK(from, to string) bool {
	elems := strings.Split(to, "/")
	for i := len(elems) - 1; i >= 0; i-- {
		if elems[i] != "internal" {
			continue
		}
		// A standard library internal package (internal/abi) is for the
		// standard library only.
		parent := strings.Join(elems[:i], "/")
		return parent != "" && (from == parent || strings.HasPrefix(from, parent+"/"))
	}
	return true
}
