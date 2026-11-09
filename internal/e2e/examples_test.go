package e2e

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExamples builds, vets and tests every example module, and checks that
// its committed generated files, code and artifacts, are what cfgx
// generates now.
func TestExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the examples")
	}
	mods, err := filepath.Glob(filepath.Join("..", "..", "examples", "*", "go.mod"))
	if err != nil || len(mods) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, mod := range mods {
		dir := filepath.Dir(mod)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			before, err := snapshot(dir)
			if err != nil {
				t.Fatal(err)
			}
			// Whatever happens, leave the example as it was; a failure says
			// how to update it.
			t.Cleanup(func() { restore(dir, before) })
			goCmd(t, dir, "generate", "./...")
			after, err := snapshot(dir)
			if err != nil {
				t.Fatal(err)
			}
			stale := false
			for f, data := range after {
				if old, ok := before[f]; !ok || !bytes.Equal(old, data) {
					stale = true
					t.Errorf("%s is stale or missing; run go generate ./... in %s", f, dir)
				}
			}
			if stale {
				return // vet and test would only repeat it
			}
			goCmd(t, dir, "build", "./...")
			goCmd(t, dir, "vet", "./...")
			goCmd(t, dir, "test", "./...")
		})
	}
}

func goCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v in %s: %v\n%s", args, dir, err, out)
	}
}

// snapshot reads every file under dir, by path.
func snapshot(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = data
		return err
	})
	return files, err
}

// restore puts dir back as snapshot found it: changed files get their old
// contents, and new files are removed.
func restore(dir string, files map[string][]byte) {
	now, _ := snapshot(dir)
	for f, data := range now {
		old, ok := files[f]
		switch {
		case !ok:
			_ = os.Remove(f)
		case !bytes.Equal(old, data):
			_ = os.WriteFile(f, old, 0o644)
		}
	}
}
