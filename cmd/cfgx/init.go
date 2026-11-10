package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"text/template"

	modmodule "golang.org/x/mod/module"
)

// skeleton holds the files cfgx init writes (ADR 0007), as templates.
//
//go:embed skeleton
var skeleton embed.FS

const cfgxModule = "github.com/epistemer/cfgx"

// initData fills the skeleton's templates.
type initData struct {
	Module    string // the module path, example.com/hello
	Bin       string // the program's name, hello
	Name      string // the app's name in the schema, hello
	EnvPrefix string // HELLO
}

var majorRE = regexp.MustCompile(`^v[0-9]+$`)

// newInitData derives the names from a module path: the program is named
// after its last element, as go build names it.
func newInitData(module string) (initData, error) {
	if err := modmodule.CheckImportPath(module); err != nil {
		return initData{}, fmt.Errorf("%q isn't a module path, such as example.com/hello: %v", module, err)
	}
	elems := strings.Split(module, "/")
	bin := elems[len(elems)-1]
	if len(elems) > 1 && majorRE.MatchString(bin) {
		bin = elems[len(elems)-2]
	}
	if bin == "" || bin == "." || bin == ".." {
		return initData{}, fmt.Errorf("%q has no name for the program", module)
	}
	// The app's name gives the env prefix: lower-case letters and digits,
	// in words joined by single dashes, starting with a letter.
	name := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(bin), "-"), "-")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "app-" + name
		name = strings.TrimSuffix(name, "-")
	}
	return initData{Module: module, Bin: bin, Name: name, EnvPrefix: strings.ToUpper(strings.ReplaceAll(name, "-", "_"))}, nil
}

// render executes every template of the skeleton: the files to write, by
// path relative to the new module's root.
func render(d initData) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := fs.WalkDir(skeleton, "skeleton", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		src, err := skeleton.ReadFile(p)
		if err != nil {
			return err
		}
		t, err := template.New(p).Option("missingkey=error").Parse(string(src))
		if err != nil {
			return err
		}
		var b bytes.Buffer
		if err := t.Execute(&b, d); err != nil {
			return err
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, "skeleton/"), ".tmpl")
		if rel == "gitignore" {
			rel = ".gitignore" // embed leaves out files whose names start with a dot
		}
		out[rel] = b.Bytes()
		return nil
	})
	return out, err
}

const initUsage = `usage: cfgx init [-cfgx dir] <module path>

Creates a cobra application with a cfgx settings package in the current
directory (docs/adr/0007-init-skeleton.md): it writes the files, then runs
go mod init, adds cfgx as a tool, generates the settings code and builds
it. It refuses to overwrite a file that exists.

-cfgx dir uses the cfgx module in dir, through a replace directive, for
working on cfgx itself.
`

func initCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("cfgx init", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, initUsage) }
	local := fl.String("cfgx", "", "use the cfgx module in this directory")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if fl.NArg() != 1 {
		fmt.Fprint(stderr, initUsage)
		return 2
	}
	if err := initModule(ctx, fl.Arg(0), *local, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "cfgx init:", err)
		return 1
	}
	return 0
}

func initModule(ctx context.Context, module, local string, stdout, stderr io.Writer) error {
	d, err := newInitData(module)
	if err != nil {
		return err
	}
	files, err := render(d)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	// What go mod and cfgx gen write, too.
	made := []string{"go.mod", "go.sum", "internal/settings/zz_settings.go", d.Bin + ".example.yaml",
		d.Bin + ".schema.json", ".env.example", "docs/CONFIG.md", "cfgx.lock.json"}
	var exist []string
	for _, p := range append(made, paths...) {
		if _, err := os.Lstat(filepath.FromSlash(p)); err == nil {
			exist = append(exist, p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if len(exist) > 0 {
		return fmt.Errorf("refusing to overwrite %s", strings.Join(exist, ", "))
	}
	if local != "" {
		if local, err = filepath.Abs(local); err != nil {
			return err
		}
	}
	// If a step fails, everything init made goes, so it can run again.
	before, err := tree(".")
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			undo(before)
		}
	}()
	for _, p := range paths {
		f := filepath.FromSlash(p)
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f, files[p], 0o644); err != nil {
			return err
		}
	}
	steps := [][]string{{"mod", "init", module}}
	if local != "" {
		steps = append(steps, []string{"mod", "edit",
			"-require=" + cfgxModule + "@v0.0.0-00010101000000-000000000000",
			"-replace=" + cfgxModule + "=" + filepath.ToSlash(local),
			"-tool=" + cfgxModule + "/cmd/cfgx"})
	} else {
		steps = append(steps, []string{"get", "-tool", cfgxModule + "/cmd/cfgx@" + cfgxVersion()})
	}
	steps = append(steps,
		[]string{"mod", "tidy"},
		[]string{"generate", "./..."},
		[]string{"mod", "tidy"},
		[]string{"build", "."},
	)
	for _, s := range steps {
		fmt.Fprintln(stderr, "go", strings.Join(s, " "))
		cmd := exec.CommandContext(ctx, "go", s...)
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go %s: %v", strings.Join(s, " "), err)
		}
	}
	done = true
	fmt.Fprintf(stdout, "Created %s. Next: ./%s serve\n", module, d.Bin)
	return nil
}

// tree lists every path under dir.
func tree(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if err == nil {
			out[p] = true
		}
		return err
	})
	return out, err
}

// undo removes what isn't in before, deepest first.
func undo(before map[string]bool) {
	after, _ := tree(".")
	var made []string
	for p := range after {
		if !before[p] {
			made = append(made, p)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(made)))
	for _, p := range made {
		os.Remove(p)
	}
}

// cfgxVersion is the version of the running cfgx, which the new module
// gets too; latest for a build without one.
func cfgxVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Path == cfgxModule && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "latest"
}
