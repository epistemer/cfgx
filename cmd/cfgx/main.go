// Command cfgx generates typed configuration code from a cfgx schema.
//
// Usage, from the package that will hold the generated code:
//
//	//go:generate go run github.com/epistemer/cfgx/cmd/cfgx gen ./schema
//
// It writes zz_<package>.go next to the go:generate line, and
// zz_<package>_i18n.go when the schema uses App.I18n. See
// docs/interface.md.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/epistemer/cfgx/internal/gen"
	"github.com/epistemer/cfgx/internal/ir"
	"github.com/epistemer/cfgx/internal/loader"
	"github.com/epistemer/cfgx/internal/lockdiff"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr))
}

const usage = `usage: cfgx gen [-check] [-timeout d] [-artifacts dir] <schema package>
       cfgx diff [-fail-breaking] <old cfgx.lock.json> [<new cfgx.lock.json>]
       cfgx init [-cfgx dir] <module path>

Runs the schema package and writes zz_<package>.go in the current directory
(and zz_<package>_i18n.go with App.I18n), and the example config, its JSON
Schema, .env.example, docs/CONFIG.md and cfgx.lock.json under the module root, or under -artifacts dir ("-" for none).
With -check, it writes nothing and fails if any of them is stale.

diff lists what changed between two lock files, breaking changes first; the
new one defaults to the module's cfgx.lock.json. With -fail-breaking, it
exits 1 when a change is breaking.

init creates a new cobra application with a cfgx settings package in the
current directory.
`

func run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "diff" {
		return diff(ctx, args[1:], os.Stdout, stderr)
	}
	if len(args) > 0 && args[0] == "init" {
		return initCmd(ctx, args[1:], os.Stdout, stderr)
	}
	if len(args) == 0 || args[0] != "gen" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("cfgx gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 2*time.Minute, "stop running the schema after this long")
	check := fs.Bool("check", false, "write nothing; fail if a generated file is stale")
	artifacts := fs.String("artifacts", "", "where to write the example config, JSON Schema, docs and lock file (default: the module root; - for none)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err := generate(ctx, fs.Arg(0), *timeout, *artifacts, *check, stderr); err != nil {
		fmt.Fprintln(stderr, "cfgx:", err)
		return 1
	}
	return 0
}

func generate(ctx context.Context, pattern string, timeout time.Duration, artifactDir string, check bool, stderr io.Writer) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	pkg, err := packageName(ctx, wd)
	if err != nil {
		return err
	}
	app, err := loader.Load(ctx, pattern, wd, timeout)
	if err != nil {
		return err
	}
	model, err := gen.Build(app, pkg)
	if err != nil {
		return fmt.Errorf("the schema has problems:\n%v", err)
	}
	for _, w := range model.Warnings {
		fmt.Fprintf(stderr, "cfgx: warning: %s: %s\n", w.Pos, w.Msg)
	}
	if err := gen.ResolveRefs(ctx, model, wd); err != nil {
		return fmt.Errorf("the schema has problems:\n%v", err)
	}
	files, err := gen.Artifacts(model)
	if err != nil {
		return err
	}
	out := filepath.Join(wd, "zz_"+pkg+".go")
	if err := gen.ResolveComputed(ctx, model, wd, out); err != nil {
		return fmt.Errorf("the schema has problems:\n%v", err)
	}
	src, err := gen.Render(model)
	if err != nil {
		return err
	}
	all := map[string][]byte{out: src}
	// The labels file exists only with App.I18n. One that cfgx wrote
	// before the schema dropped it becomes a stub without the i18n import:
	// go generate has already listed the package's files, so removing one
	// would fail it.
	labels := filepath.Join(wd, "zz_"+pkg+"_i18n.go")
	if model.I18n {
		if all[labels], err = gen.RenderI18n(model); err != nil {
			return err
		}
	} else if old, err := os.ReadFile(labels); err == nil && bytes.HasPrefix(old, []byte(generatedHeader)) {
		all[labels] = []byte(generatedHeader + "\n// The schema no longer uses App.I18n: delete this file.\n\npackage " + pkg + "\n")
		fmt.Fprintf(stderr, "cfgx: warning: the schema no longer uses App.I18n; delete %s\n", filepath.Base(labels))
	}
	// App.Labels: the same labels as a labels/v1 file, which genx turns
	// into Go. A JSON file can't be a stub, so one the schema no longer
	// asks for is left, with a warning.
	data := filepath.Join(wd, "zz_"+pkg+".labels.json")
	if model.Labels {
		root, err := moduleRoot(ctx, wd)
		if err != nil {
			return err
		}
		dir, err := filepath.Rel(root, wd)
		if err != nil {
			return err
		}
		if all[data], err = gen.RenderLabels(model, filepath.ToSlash(dir)); err != nil {
			return err
		}
	} else if _, err := os.Stat(data); err == nil {
		fmt.Fprintf(stderr, "cfgx: warning: the schema no longer uses App.Labels; delete %s\n", filepath.Base(data))
	}
	if artifactDir != "-" {
		if artifactDir == "" {
			if artifactDir, err = moduleRoot(ctx, wd); err != nil {
				return err
			}
		}
		for name, data := range files {
			all[filepath.Join(artifactDir, filepath.FromSlash(name))] = data
		}
	}
	paths := make([]string, 0, len(all))
	for p := range all {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if check {
		var stale []string
		for _, p := range paths {
			if old, err := os.ReadFile(p); err != nil || !bytes.Equal(old, all[p]) {
				if rel, err := filepath.Rel(wd, p); err == nil {
					p = rel
				}
				stale = append(stale, p)
			}
		}
		if len(stale) > 0 {
			return fmt.Errorf("generated files are stale: %s; run go generate", strings.Join(stale, ", "))
		}
		return nil
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := writeIfChanged(p, all[p]); err != nil {
			return err
		}
	}
	return nil
}

// generatedHeader starts every Go file cfgx generates.
const generatedHeader = "// Code generated by cfgx. DO NOT EDIT.\n"

// writeIfChanged writes data to path unless it already holds exactly that,
// so an unchanged file keeps its timestamp.
func writeIfChanged(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

// moduleRoot is the directory of the go.mod that governs dir.
func moduleRoot(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	gomod := strings.TrimSpace(string(out))
	if err != nil || gomod == "" || gomod == os.DevNull {
		return "", fmt.Errorf("can't find the module of %s for the generated files; pass -artifacts", dir)
	}
	return filepath.Dir(gomod), nil
}

// packageName is the package the generated file joins: $GOPACKAGE under
// go generate, otherwise asked of the go command.
func packageName(ctx context.Context, dir string) (string, error) {
	if p := os.Getenv("GOPACKAGE"); p != "" {
		return p, nil
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-f", "{{.Name}}", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	name := strings.TrimSpace(string(out))
	if err != nil || name == "" {
		return "", fmt.Errorf("can't tell which package %s is; run cfgx through go generate, or from a directory with Go files", dir)
	}
	return name, nil
}

// diff runs cfgx diff.
func diff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cfgx diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	failBreaking := fs.Bool("fail-breaking", false, "exit 1 if a change is breaking")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	newPath := fs.Arg(1)
	if newPath == "" {
		wd, err := os.Getwd()
		if err == nil {
			var root string
			if root, err = moduleRoot(ctx, wd); err == nil {
				newPath = filepath.Join(root, "cfgx.lock.json")
			}
		}
		if err != nil {
			fmt.Fprintln(stderr, "cfgx:", err)
			return 1
		}
	}
	var apps [2]*ir.App
	for i, p := range []string{fs.Arg(0), newPath} {
		data, err := os.ReadFile(p)
		if err == nil {
			apps[i], err = lockdiff.Parse(data)
		}
		if err != nil {
			fmt.Fprintf(stderr, "cfgx: %s: %v\n", p, err)
			return 1
		}
	}
	changes := lockdiff.Diff(apps[0], apps[1])
	fmt.Fprint(stdout, lockdiff.Report(changes))
	if *failBreaking && len(changes) > 0 && changes[0].Class == lockdiff.Breaking {
		return 1
	}
	return 0
}
