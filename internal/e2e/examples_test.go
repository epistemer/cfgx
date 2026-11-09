package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExamples builds, vets and tests every example module, and checks that
// its committed generated code is what cfgx generates now.
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
			generated, err := filepath.Glob(filepath.Join(dir, "internal", "*", "zz_*.go"))
			if err != nil || len(generated) == 0 {
				t.Fatalf("no generated files in %s", dir)
			}
			before := map[string][]byte{}
			for _, f := range generated {
				if before[f], err = os.ReadFile(f); err != nil {
					t.Fatal(err)
				}
			}
			// Whatever happens, leave the committed files as they were; a
			// failure says how to update them.
			t.Cleanup(func() {
				for f, old := range before {
					if now, err := os.ReadFile(f); err != nil || !bytes.Equal(old, now) {
						_ = os.WriteFile(f, old, 0o644)
					}
				}
			})
			goCmd(t, dir, "generate", "./...")
			stale := false
			for f, old := range before {
				now, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(old, now) {
					stale = true
					t.Errorf("%s is stale; run go generate ./... in %s", f, dir)
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
