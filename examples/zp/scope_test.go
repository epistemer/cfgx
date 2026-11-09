package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The compiler enforces each command's scope (interface.md §5.2): code that
// hands a command's configuration to a library that needs other keys, or
// reads a key the command doesn't load, doesn't build.
func TestScopeIsEnforcedByTheCompiler(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go vet")
	}
	for _, c := range []struct {
		name, body, want string
	}{
		{"migrate has no mailer", `func f(m *settings.Migrate) { mailer.New(m) }`,
			"*settings.Migrate does not implement settings.Mailer"},
		{"test-email has only the Mailer keys of auth", `func f(t *settings.TestEmail) { _ = t.AuthJWTPrivate() }`,
			"t.AuthJWTPrivate undefined"},
		{"test-email has the mailer", `func f(t *settings.TestEmail) { mailer.New(t); _ = t.AuthEmailFrom() }`, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Inside the module, so the package may import zp/internal/...
			dir, err := os.MkdirTemp(".", "zzscope")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			src := "package scope\n\nimport (\n\t\"zp/internal/mailer\"\n\t\"zp/internal/settings\"\n)\n\nvar _, _ = mailer.New, settings.BindRoot\n\n" + c.body + "\n"
			if err := os.WriteFile(filepath.Join(dir, "scope.go"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("go", "vet", "./"+filepath.ToSlash(dir)).CombinedOutput()
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("should build:\n%s", out)
			case c.want != "" && err == nil:
				t.Fatalf("built, but shouldn't: %s", c.body)
			case c.want != "" && !strings.Contains(string(out), c.want):
				t.Fatalf("failed without %q:\n%s", c.want, out)
			}
		})
	}
}
