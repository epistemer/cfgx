package cfgxtest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGenerateLine(t *testing.T) {
	for line, want := range map[string][]string{
		"//go:generate go tool cfgx gen ./schema":                                      {"go", "tool", "cfgx", "gen", "-check", "./schema"},
		"//go:generate\tgo run example.com/cfgx/cmd/cfgx gen -artifacts \"a dir\" ./s": {"go", "run", "example.com/cfgx/cmd/cfgx", "gen", "-check", "-artifacts", "a dir", "./s"},
	} {
		dir := t.TempDir()
		// A directive in a test file counts, as it does for go generate.
		if err := os.WriteFile(filepath.Join(dir, "gen_test.go"), []byte("package p\n\n"+line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := generateLine(dir)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s: got %q, %v; want %q", line, got, err, want)
		}
	}
	if _, err := generateLine(t.TempDir()); err == nil {
		t.Error("no line: want an error")
	}
}
