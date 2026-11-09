// Command cfgx generates typed configuration code from a cfgx schema.
//
// Usage, from the package that will hold the generated code:
//
//	//go:generate go run github.com/epistemer/cfgx/cmd/cfgx gen ./schema
//
// It writes zz_<package>.go next to the go:generate line. See
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
	"github.com/epistemer/cfgx/internal/loader"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr))
}

const usage = `usage: cfgx gen [-timeout d] [-artifacts dir] <schema package>

Runs the schema package and writes zz_<package>.go in the current directory,
and the example config, its JSON Schema, .env.example, docs/CONFIG.md and
cfgx.lock.json under the module root, or under -artifacts dir ("-" for none).
`

func run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "gen" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("cfgx gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 2*time.Minute, "stop running the schema after this long")
	artifacts := fs.String("artifacts", "", "where to write the example config, JSON Schema, docs and lock file (default: the module root; - for none)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err := generate(ctx, fs.Arg(0), *timeout, *artifacts, stderr); err != nil {
		fmt.Fprintln(stderr, "cfgx:", err)
		return 1
	}
	return 0
}

func generate(ctx context.Context, pattern string, timeout time.Duration, artifactDir string, stderr io.Writer) error {
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
	if err := writeIfChanged(out, src); err != nil {
		return err
	}
	if artifactDir == "-" {
		return nil
	}
	if artifactDir == "" {
		if artifactDir, err = moduleRoot(ctx, wd); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(artifactDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := writeIfChanged(path, files[name]); err != nil {
			return err
		}
	}
	return nil
}

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
