// Package golden provides helpers for golden-file tests.
//
// Pass -update to "go test" to rewrite golden files with the actual output.
package golden

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/diff"
)

var update = flag.Bool("update", false, "update golden files with actual output")

// Update reports whether the -update test flag is enabled.
func Update() bool {
	return *update
}

// Assert compares got against the golden file at path, normalizing CRLF line
// endings to LF on both sides before comparing. When the -update flag is set,
// Assert writes the normalized got bytes to path instead.
func Assert(t testing.TB, path string, got []byte) {
	t.Helper()

	gotNorm := normalize(got)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("golden: mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, gotNorm, 0o644); err != nil {
			t.Fatalf("golden: write %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: read %s: %v (run with -update to create)", path, err)
	}
	wantNorm := normalize(want)

	if bytes.Equal(gotNorm, wantNorm) {
		return
	}

	lineNum, wantLine, gotLine := firstDiffLine(wantNorm, gotNorm)
	unified := diff.Diff(path, wantNorm, "got", gotNorm)
	t.Fatalf(
		"golden mismatch for %s (first difference at line %d: want %q, got %q; re-run with -update to update):\n%s",
		path, lineNum, wantLine, gotLine, unified,
	)
}

// normalize replaces CRLF (\r\n) with LF (\n).
func normalize(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// firstDiffLine returns the 1-based line number of the first differing line
// along with the expected and actual content of that line.
func firstDiffLine(want, got []byte) (line int, wantLine, gotLine string) {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	n := min(len(wantLines), len(gotLines))
	for i := range n {
		if wantLines[i] != gotLines[i] {
			return i + 1, wantLines[i], gotLines[i]
		}
	}
	if len(wantLines) > n {
		return n + 1, wantLines[n], "<EOF>"
	}
	if len(gotLines) > n {
		return n + 1, "<EOF>", gotLines[n]
	}
	return 1, "", ""
}
