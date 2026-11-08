package gen

import (
	"context"
	"fmt"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// fixedImports are the packages the generated file imports, by path, under
// these names. context and slices are imported only when there are commands.
func fixedImports(m *Model) map[string]string {
	f := map[string]string{
		"github.com/epistemer/cfgx":        "cfgx",
		"github.com/epistemer/cfgx/engine": "engine",
		"github.com/spf13/cobra":           "cobra",
	}
	if len(m.Commands) > 0 {
		f["context"], f["slices"] = "context", "slices"
	}
	for _, imp := range m.Imports {
		f[imp.Path] = imp.Alias
	}
	return f
}

// HasComputed reports whether any group declares computed methods.
func (m *Model) HasComputed() bool {
	for _, g := range m.Groups {
		if len(g.Computed) > 0 {
			return true
		}
	}
	return false
}

// ResolveComputed fills in the signatures of computed methods (A8). It
// renders the file without them, type-checks the package in dir with that
// file in place of outFile (an overlay: nothing is written), and reads each
// method from its group's type. Any method without parameters is allowed; a
// missing method, a promoted one, or parameters are problems.
func ResolveComputed(ctx context.Context, m *Model, dir, outFile string) error {
	if !m.HasComputed() {
		return nil
	}
	stub, err := Render(m)
	if err != nil {
		return err
	}
	// go/packages matches overlays by absolute path.
	if outFile, err = filepath.Abs(outFile); err != nil {
		return err
	}
	cfg := &packages.Config{
		Context: ctx,
		Mode:    packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:     dir,
		Overlay: map[string][]byte{outFile: stub},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return fmt.Errorf("type-checking the settings package for computed methods: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Types == nil {
		return fmt.Errorf("type-checking the settings package for computed methods: found %d packages in %s", len(pkgs), dir)
	}
	pkg := pkgs[0]

	q := newQualifier(pkg.Types, fixedImports(m))
	var probs Problems
	for _, g := range m.Groups {
		if len(g.Computed) == 0 {
			continue
		}
		obj, _ := pkg.Types.Scope().Lookup(g.Type).(*types.TypeName)
		if obj == nil {
			probs = append(probs, Problem{g.pos, fmt.Sprintf("group %q: type %s is missing from the type-checked package (a cfgx bug)", g.Path, g.Type)})
			continue
		}
		ms := types.NewMethodSet(types.NewPointer(obj.Type()))
		for _, c := range g.Computed {
			sel := ms.Lookup(pkg.Types, c.Name)
			switch {
			case sel == nil:
				probs = append(probs, Problem{c.Pos, fmt.Sprintf("group %q declares computed method %s, but *%s has no method %s; write func (g *%s) %s() ... in package %s", g.Path, c.Name, g.Type, c.Name, g.Type, c.Name, pkg.Name)})
				continue
			case len(sel.Index()) > 1:
				probs = append(probs, Problem{c.Pos, fmt.Sprintf("computed method %s of group %q is promoted from a nested group; declare it on *%s itself, or list it on the nested group", c.Name, g.Path, g.Type)})
				continue
			}
			sig := sel.Type().(*types.Signature)
			if sig.Params().Len() > 0 {
				probs = append(probs, Problem{c.Pos, fmt.Sprintf("computed method %s of group %q takes parameters; computed methods take none", c.Name, g.Path)})
				continue
			}
			c.Sig = c.Name + "()" + results(sig.Results(), q.qualify)
		}
	}
	if len(probs) > 0 {
		var errs []string
		for _, e := range pkg.Errors {
			errs = append(errs, e.Error())
			if len(errs) == 5 {
				break
			}
		}
		if len(errs) > 0 {
			probs = append(probs, Problem{Msg: "the settings package also has errors, which may be why:\n  " + strings.Join(errs, "\n  ")})
		}
		return probs
	}
	m.Imports = append(m.Imports, q.imports()...)
	sort.Slice(m.Imports, func(i, j int) bool { return m.Imports[i].Path < m.Imports[j].Path })
	return nil
}

// results renders a method's results as they follow "Name()".
func results(t *types.Tuple, q types.Qualifier) string {
	switch {
	case t.Len() == 0:
		return ""
	case t.Len() == 1 && t.At(0).Name() == "":
		return " " + types.TypeString(t.At(0).Type(), q)
	}
	return " " + types.TypeString(t, q)
}

// qualifier names packages in signatures. Packages other than the settings
// package itself get an import, under their package name unless that is
// taken in the file or the package, then with a number added.
type qualifier struct {
	self  *types.Package
	fixed map[string]string // path → name, imported anyway
	names map[string]string // path → name in the generated file
	taken map[string]bool
}

func newQualifier(self *types.Package, fixed map[string]string) *qualifier {
	q := &qualifier{self: self, fixed: fixed, names: map[string]string{}, taken: map[string]bool{}}
	for _, name := range self.Scope().Names() {
		q.taken[name] = true
	}
	for _, name := range fixed {
		q.taken[name] = true
	}
	return q
}

func (q *qualifier) qualify(p *types.Package) string {
	if p == q.self {
		return ""
	}
	if name, ok := q.names[p.Path()]; ok {
		return name
	}
	if name, ok := q.fixed[p.Path()]; ok {
		q.names[p.Path()] = name
		return name
	}
	name := p.Name()
	for i := 2; q.taken[name]; i++ {
		name = p.Name() + strconv.Itoa(i)
	}
	q.taken[name] = true
	q.names[p.Path()] = name
	return name
}

// imports lists the packages signatures need beyond the fixed ones, sorted.
func (q *qualifier) imports() []ImportModel {
	var out []ImportModel
	for path, name := range q.names {
		if _, fixed := q.fixed[path]; !fixed {
			out = append(out, ImportModel{Alias: name, Path: path})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
