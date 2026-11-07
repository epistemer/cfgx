package dsl

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/ir"
)

func reset() { apps, commands = nil, nil }

func TestExport(t *testing.T) {
	reset()
	db := Group("database", "Database connection",
		String("host", "Database host").Default("localhost"),
		Int("port", "Database port").Default(5432),
	)
	log := Group("log", "Logging", Bool("debug", "Debug logging"))
	App("demo", "Demo app", db, log).EnvPrefix("DEMO")
	Command("Serve").Uses(db, log)
	Command("Migrate").Uses(db)

	var buf bytes.Buffer
	if err := Export(&buf); err != nil {
		t.Fatal(err)
	}
	a, err := ir.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "demo" || a.EnvPrefix != "DEMO" || len(a.Groups) != 2 || len(a.Commands) != 2 {
		t.Fatalf("unexpected IR: %+v", a)
	}
	port := a.Groups[0].Keys[1]
	if port.Kind != ir.KindInt || port.Default == nil || *port.Default != "5432" {
		t.Errorf("port = %+v", port)
	}
	if d := a.Groups[1].Keys[0].Default; d != nil {
		t.Errorf("debug has no default, got %q", *d)
	}
	if got := strings.Join(a.Commands[0].Uses, ","); got != "database,log" {
		t.Errorf("Serve uses %q", got)
	}
	if filepath.Base(port.Pos.File) != "dsl_test.go" || port.Pos.Line == 0 {
		t.Errorf("position should point at this test's call: %v", port.Pos)
	}
}

func TestExportNeedsExactlyOneApp(t *testing.T) {
	reset()
	if err := Export(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "no dsl.App") {
		t.Errorf("no app: %v", err)
	}
	App("a", "A")
	App("b", "B")
	if err := Export(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("two apps: %v", err)
	}
}
