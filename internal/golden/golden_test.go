package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssertMatch(t *testing.T) {
	goldenPath := filepath.Join("testdata", "sample.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read sample.golden: %v", err)
	}

	// Exact match.
	Assert(t, goldenPath, want)

	// CRLF in got should match LF in golden file.
	gotCRLF := []byte(strings.ReplaceAll(string(want), "\n", "\r\n"))
	Assert(t, goldenPath, gotCRLF)
}

func TestAssertCRLFInGoldenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.golden")
	if err := os.WriteFile(path, []byte("line1\r\nline2\r\n"), 0o644); err != nil {
		t.Fatalf("write crlf.golden: %v", err)
	}

	// LF in got should match CRLF in golden file.
	Assert(t, path, []byte("line1\nline2\n"))
}

func TestAssertMismatch(t *testing.T) {
	goldenPath := filepath.Join("testdata", "sample.golden")
	got := []byte("# Sample golden file for internal/golden tests.\napp: zp\nserver:\n  host: 0.0.0.0\n  port: 8080\n")

	spy := &spyTB{TB: t}
	runWithSpy(spy, func() {
		Assert(spy, goldenPath, got)
	})

	if !spy.failed {
		t.Fatal("expected Assert to fail on mismatched content")
	}
	if !strings.Contains(spy.msg, "first difference at line 4") {
		t.Errorf("expected failure message to report line 4, got:\n%s", spy.msg)
	}
	if !strings.Contains(spy.msg, "-  host: 127.0.0.1") || !strings.Contains(spy.msg, "+  host: 0.0.0.0") {
		t.Errorf("expected failure message to contain unified diff, got:\n%s", spy.msg)
	}
}

func TestAssertMissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.golden")

	spy := &spyTB{TB: t}
	runWithSpy(spy, func() {
		Assert(spy, missingPath, []byte("hello\n"))
	})

	if !spy.failed {
		t.Fatal("expected Assert to fail when golden file does not exist")
	}
	if !strings.Contains(spy.msg, "-update") {
		t.Errorf("expected failure message to mention -update flag, got:\n%s", spy.msg)
	}
}

func TestAssertUpdate(t *testing.T) {
	prev := *update
	*update = true
	defer func() { *update = prev }()

	if !Update() {
		t.Fatal("expected Update() to return true when -update is set")
	}

	path := filepath.Join(t.TempDir(), "nested", "updated.golden")
	Assert(t, path, []byte("alpha\r\nbeta\r\n"))

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated golden file: %v", err)
	}
	if string(written) != "alpha\nbeta\n" {
		t.Fatalf("expected normalized LF content %q, got %q", "alpha\nbeta\n", string(written))
	}

	// Verify that comparing with -update=false now succeeds.
	*update = false
	Assert(t, path, []byte("alpha\nbeta\n"))
}

type spyAbort struct{}

type spyTB struct {
	testing.TB
	failed bool
	msg    string
}

func (s *spyTB) Helper() {}

func (s *spyTB) Fatalf(format string, args ...any) {
	s.failed = true
	s.msg = fmt.Sprintf(format, args...)
	panic(spyAbort{})
}

func (s *spyTB) Errorf(format string, args ...any) {
	s.failed = true
	s.msg = fmt.Sprintf(format, args...)
}

func (s *spyTB) FailNow() {
	s.failed = true
	panic(spyAbort{})
}

func runWithSpy(s *spyTB, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(spyAbort); !ok {
				panic(r)
			}
		}
	}()
	fn()
}
