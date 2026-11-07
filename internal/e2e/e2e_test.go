// Package e2e runs end-to-end testscript suites against the cfgx module.
package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/rogpeppe/go-internal/testscript"
)

const cfgxModulePath = "github.com/epistemer/cfgx"

// goEnvKeys lists the Go environment variables queried via "go env -json"
// and propagated into each testscript environment so the go command uses
// the host's warm build and module caches and works offline.
var goEnvKeys = []string{
	"GOROOT",
	"GOCACHE",
	"GOMODCACHE",
	"GOPATH",
	"GOPROXY",
	"GOFLAGS",
	"GONOSUMDB",
	"GONOSUMCHECK",
	"GOPRIVATE",
	"GOINSECURE",
	"GONOPROXY",
	"GOSUMDB",
	"GOTOOLCHAIN",
}

// hostPassthroughEnvKeys lists host and OS-specific environment variables
// passed through from os.Getenv when non-empty.
var hostPassthroughEnvKeys = []string{
	"PATH",
	// Windows variables required by the Go toolchain, linker, and crypto APIs.
	"USERPROFILE",
	"LOCALAPPDATA",
	"APPDATA",
	"SYSTEMROOT",
	"TEMP",
	"TMP",
	"PATHEXT",
	"COMSPEC",
	"WINDIR",
}

func TestScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e tests in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("skipping e2e tests: go binary not found on PATH: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot, err := findRepoRoot(wd)
	if err != nil {
		t.Fatalf("find cfgx repository root: %v", err)
	}

	goEnv, err := queryGoEnv(goEnvKeys)
	if err != nil {
		t.Fatalf("query go env: %v", err)
	}

	gosum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read repo go.sum: %v", err)
	}
	reqs, err := moduleRequirements(repoRoot)
	if err != nil {
		t.Fatalf("list the repository's modules: %v", err)
	}

	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "script"),
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"go": cmdGo,
		},
		Setup: func(e *testscript.Env) error {
			passEnv(e, goEnv)
			e.Setenv("CFGX_ROOT", repoRoot)
			gomod := formatGoMod(repoRoot, reqs)
			if err := os.WriteFile(filepath.Join(e.WorkDir, "go.mod"), []byte(gomod), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(e.WorkDir, "go.sum"), gosum, 0o644)
		},
	})
}

// cmdGo allows scripts to invoke "go ..." directly in addition to "exec go ...".
func cmdGo(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: go subcommand ...")
	}
	err := ts.Exec("go", args...)
	if err != nil {
		ts.Logf("[%v]\n", err)
		if !neg {
			ts.Fatalf("unexpected go command failure")
		}
	} else if neg {
		ts.Fatalf("unexpected go command success")
	}
}

// queryGoEnv runs "go env -json" for the given variable names and returns the
// decoded key/value map.
func queryGoEnv(keys []string) (map[string]string, error) {
	args := append([]string{"env", "-json"}, keys...)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, err
	}
	var env map[string]string
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, err
	}
	return env, nil
}

// passEnv populates e with the Go and OS environment variables needed for
// "go build" and "go run" to use the real caches and work across platforms.
func passEnv(e *testscript.Env, goEnv map[string]string) {
	for _, k := range hostPassthroughEnvKeys {
		if v := os.Getenv(k); v != "" {
			e.Setenv(k, v)
		}
	}

	// Always set core Go cache/module variables from "go env" (falling back to os.Getenv).
	alwaysKeys := map[string]bool{
		"GOCACHE":    true,
		"GOMODCACHE": true,
		"GOPATH":     true,
		"GOPROXY":    true,
		"GOFLAGS":    true,
	}
	for _, k := range goEnvKeys {
		v := goEnv[k]
		if v == "" {
			v = os.Getenv(k)
		}
		if v != "" || alwaysKeys[k] {
			e.Setenv(k, v)
		}
	}
}

// findRepoRoot walks up from startDir until it finds a go.mod file declaring
// module github.com/epistemer/cfgx.
func findRepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		gomodPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gomodPath); err == nil {
			if declaresModule(string(data), cfgxModulePath) {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod for module " + cfgxModulePath + " not found")
		}
		dir = parent
	}
}

// declaresModule reports whether a go.mod file's contents declare the given module path.
func declaresModule(gomodContent, wantModule string) bool {
	for _, line := range strings.Split(gomodContent, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			mod := strings.Trim(fields[1], "\"`")
			return mod == wantModule
		}
	}
	return false
}

// moduleRequirements lists every module in the repository's build list
// except cfgx itself, as "path version" lines. Scripts require all of them,
// so their go.mod never drifts from the repository's dependencies; the
// repository's go.sum, copied next to it, covers every entry.
func moduleRequirements(repoRoot string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{if not .Main}}{{.Path}} {{.Version}}{{end}}", "all")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var reqs []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			reqs = append(reqs, line)
		}
	}
	return reqs, nil
}

// formatGoMod returns the go.mod contents for an e2e script's working directory.
func formatGoMod(repoRoot string, reqs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module example.com/e2e\n\ngo 1.26.0\n\nrequire %s v0.0.0\n", cfgxModulePath)
	if len(reqs) > 0 {
		b.WriteString("\nrequire (\n")
		for _, r := range reqs {
			fmt.Fprintf(&b, "\t%s // indirect\n", r)
		}
		b.WriteString(")\n")
	}
	fmt.Fprintf(&b, "\nreplace %s => %s\n", cfgxModulePath, quoteGoModToken(repoRoot))
	return b.String()
}

// quoteGoModToken quotes s for go.mod if it contains whitespace or special
// characters, while leaving standard Unix and Windows paths unquoted when safe.
func quoteGoModToken(s string) string {
	if mustQuoteGoModToken(s) {
		return strconv.Quote(s)
	}
	return s
}

func mustQuoteGoModToken(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '"', '\'', '`', '(', ')', '[', ']', '{', '}', ',':
			return true
		default:
			if !unicode.IsPrint(r) {
				return true
			}
		}
	}
	return s == "" || strings.Contains(s, "//") || strings.Contains(s, "/*")
}
