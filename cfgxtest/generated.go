package cfgxtest

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AssertGenerated fails t if the generated files of the package in the
// current directory are stale: the code and, under the module root, the
// example config, JSON Schema, docs and lock file. It finds the package's
// "//go:generate ... cfgx gen ..." line and runs it with -check, which
// writes nothing. Call it from a test in the generated package:
//
//	func TestGenerated(t *testing.T) { cfgxtest.AssertGenerated(t) }
func AssertGenerated(t testing.TB) {
	t.Helper()
	args, err := generateLine(".")
	if err != nil {
		t.Fatalf("cfgxtest: %v", err)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cfgxtest: %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// generateLine returns the cfgx go:generate command of the package in dir,
// with -check added after gen.
func generateLine(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		fh, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			line, ok := strings.CutPrefix(sc.Text(), "//go:generate ")
			if !ok {
				continue
			}
			fields := strings.Fields(line)
			for i, w := range fields {
				if w == "gen" && i > 0 && strings.Contains(fields[i-1], "cfgx") {
					fh.Close()
					out := append(append(append([]string(nil), fields[:i+1]...), "-check"), fields[i+1:]...)
					return out, nil
				}
			}
		}
		fh.Close()
	}
	return nil, &noLineError{dir}
}

type noLineError struct{ dir string }

func (e *noLineError) Error() string {
	abs, _ := filepath.Abs(e.dir)
	return "no //go:generate line running cfgx gen in " + abs
}
