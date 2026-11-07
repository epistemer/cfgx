package funcref

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/epistemer/cfgx/spikes/s3-funcref/fns"
	"github.com/epistemer/cfgx/spikes/s3-funcref/internal/ifns"
	"github.com/epistemer/cfgx/spikes/s3-funcref/schema"
	yaml "github.com/epistemer/cfgx/spikes/s3-funcref/yaml.v3"
)

const (
	mod       = "github.com/epistemer/cfgx/spikes/s3-funcref"
	schemaPkg = mod + "/schema"
)

func TestResolve(t *testing.T) {
	v := &fns.V{}
	cases := []struct {
		name string
		f    any
		want string // empty: must be rejected
	}{
		{"top-level", fns.Email, mod + "/fns.Email"},
		{"internal package", ifns.Internal, mod + "/internal/ifns.Internal"},
		{"dot in last path element", yaml.Dotted, mod + "/yaml.v3.Dotted"},
		{"stdlib", strings.ToUpper, "strings.ToUpper"},
		{"stdlib, assembly-backed", atomic.AddInt32, "sync/atomic.AddInt32"},
		{"stdlib, math", math.Sqrt, "math.Sqrt"},
		{"unexported", fns.Unexported, ""},
		{"closure", fns.Make(), ""},
		{"package-level closure", fns.PackageClosure, ""},
		{"method value", v.Check, ""},
		{"pointer method value", v.PCheck, ""},
		{"method expression", fns.V.Check, ""},
		{"pointer method expression", (*fns.V).PCheck, ""},
		{"generic instance", fns.Max[int], ""},
		{"schema package", schema.Local, ""},
	}
	for _, c := range cases {
		raw, err := Raw(c.f)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		ref, err := Resolve(c.f, schemaPkg)
		t.Logf("%-26s raw=%q\n%28s ref=%q err=%v", c.name, raw, "", ref.String(), err)
		if c.want == "" {
			if err == nil {
				t.Errorf("%s: accepted %s, want a rejection", c.name, ref)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := ref.String(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestSplit(t *testing.T) {
	cases := []struct{ raw, pkg, sym string }{
		{"strings.ToUpper", "strings", "ToUpper"},
		{"example.com/a.b/c.F", "example.com/a.b/c", "F"},
		{"gopkg.in/yaml%2ev3.Unmarshal", "gopkg.in/yaml.v3", "Unmarshal"},
		{"example.com/p.(*T).M", "example.com/p", "(*T).M"},
		{"example.com/p.Max[...]", "example.com/p", "Max[...]"},
		{"example.com/p.G[example.com/q.T]", "example.com/p", "G[example.com/q.T]"},
	}
	for _, c := range cases {
		pkg, sym, err := Split(c.raw)
		if err != nil || pkg != c.pkg || sym != c.sym {
			t.Errorf("Split(%q) = %q, %q, %v; want %q, %q", c.raw, pkg, sym, err, c.pkg, c.sym)
		}
	}
}

// TestEmittedReferencesCompile writes a program that imports every accepted
// reference under a generated alias, as generated code will, and builds it.
func TestEmittedReferencesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	accepted := []any{fns.Email, ifns.Internal, yaml.Dotted, strings.ToUpper, atomic.AddInt32}
	var imports, uses strings.Builder
	for i, f := range accepted {
		ref, err := Resolve(f, schemaPkg)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&imports, "\tx%d %q\n", i, ref.PkgPath)
		fmt.Fprintf(&uses, "\t_ = x%d.%s\n", i, ref.Name)
	}
	src := "package main\n\nimport (\n" + imports.String() + ")\n\nfunc main() {\n" + uses.String() + "}\n"

	// Inside the module, so internal/ imports are allowed.
	dir, err := os.MkdirTemp(".", "gen_tmp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("go", "build", "-o", os.DevNull, "./"+filepath.Base(dir)).CombinedOutput()
	if err != nil {
		t.Fatalf("generated code doesn't build: %v\n%s\n%s", err, out, src)
	}
}
