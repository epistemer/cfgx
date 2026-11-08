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
	"strings"
	"time"

	"github.com/epistemer/cfgx/internal/gen"
	"github.com/epistemer/cfgx/internal/loader"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr))
}

const usage = `usage: cfgx gen [-timeout d] <schema package>

Runs the schema package and writes zz_<package>.go in the current directory.
`

func run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "gen" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("cfgx gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 2*time.Minute, "stop running the schema after this long")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err := generate(ctx, fs.Arg(0), *timeout); err != nil {
		fmt.Fprintln(stderr, "cfgx:", err)
		return 1
	}
	return 0
}

func generate(ctx context.Context, pattern string, timeout time.Duration) error {
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
	out := filepath.Join(wd, "zz_"+pkg+".go")
	if err := gen.ResolveComputed(ctx, model, wd, out); err != nil {
		return fmt.Errorf("the schema has problems:\n%v", err)
	}
	src, err := gen.Render(model)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(out); err == nil && bytes.Equal(old, src) {
		return nil // unchanged; leave the file and its timestamp alone
	}
	return os.WriteFile(out, src, 0o644)
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
