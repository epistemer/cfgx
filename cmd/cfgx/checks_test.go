package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/epistemer/cfgx/internal/gen"
	"github.com/epistemer/cfgx/internal/loader"
)

// TestSchemaChecks runs every schema in testdata/checks through the real
// DSL, loader and generator (roadmap M5.7). Each schema marks the lines its
// problems must point at with a comment:
//
//	dsl.String("host", ""), // want "key server.host needs a description"
//
// Every problem and warning must match a want comment on its line, by
// substring, and every want comment must be matched.
func TestSchemaChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs every schema")
	}
	dirs, err := filepath.Glob(filepath.Join("testdata", "checks", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no schemas: %v", err)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			abs, err := filepath.Abs(dir)
			if err != nil {
				t.Fatal(err)
			}
			wants := readWants(t, filepath.Join(abs, "schema.go"))
			if len(wants) == 0 {
				t.Fatal("schema.go has no want comments")
			}
			for _, f := range findings(t, abs) {
				line, ok := wants[f.line]
				if !ok || !strings.Contains(f.msg, line.want) {
					t.Errorf("unexpected problem at line %d: %s", f.line, f.msg)
					continue
				}
				line.matched = true
			}
			for n, w := range wants {
				if !w.matched {
					t.Errorf("line %d: no problem containing %q", n, w.want)
				}
			}
		})
	}
}

type want struct {
	want    string
	matched bool
}

var wantRE = regexp.MustCompile(`// want "((?:[^"\\]|\\.)*)"`)

func readWants(t *testing.T, file string) map[int]*want {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	wants := map[int]*want{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if m := wantRE.FindStringSubmatch(sc.Text()); m != nil {
			s, err := strconv.Unquote(`"` + m[1] + `"`)
			if err != nil {
				t.Fatalf("line %d: bad want: %v", n, err)
			}
			wants[n] = &want{want: s}
		}
	}
	return wants
}

type finding struct {
	line int
	msg  string
}

// problemRE matches "schema.go:LINE: message" in errors from the schema
// program, such as dsl.Export's.
var problemRE = regexp.MustCompile(`schema\.go:(\d+): (.*)`)

// findings loads the schema in dir and returns every problem and warning in
// schema.go, by line.
func findings(t *testing.T, dir string) []finding {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	app, err := loader.Load(ctx, ".", dir, time.Minute)
	if err != nil {
		var out []finding
		for _, l := range strings.Split(err.Error(), "\n") {
			if m := problemRE.FindStringSubmatch(l); m != nil {
				n, _ := strconv.Atoi(m[1])
				out = append(out, finding{n, m[2]})
			}
		}
		if len(out) == 0 {
			t.Fatalf("the schema failed without a schema.go position: %v", err)
		}
		return out
	}
	m, err := gen.Build(app, "settings")
	var probs gen.Problems
	if err != nil {
		probs = err.(gen.Problems)
	} else {
		probs = m.Warnings
	}
	var out []finding
	for _, p := range probs {
		if filepath.Base(p.Pos.File) != "schema.go" {
			t.Errorf("problem outside schema.go: %s: %s", p.Pos, p.Msg)
			continue
		}
		out = append(out, finding{p.Pos.Line, p.Msg})
	}
	return out
}
