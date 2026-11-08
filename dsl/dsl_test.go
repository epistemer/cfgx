package dsl

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/ir"
)

func reset() { apps, commands, tags = nil, nil, nil }

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

// A command must use the very group passed to dsl.App, not another group
// that happens to have the same name.
func TestUsesNeedsTheAppsGroup(t *testing.T) {
	reset()
	inApp := Group("database", "Database", String("host", "Host"))
	lookalike := Group("database", "Database", String("host", "Host"))
	App("demo", "Demo", inApp)
	Command("Serve").Uses(lookalike)
	var buf bytes.Buffer
	if err := Export(&buf); err != nil {
		t.Fatal(err)
	}
	a, err := ir.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	c := a.Commands[0]
	if len(c.Uses) != 0 || len(c.Orphans) != 1 || c.Orphans[0].Name != "database" {
		t.Fatalf("the lookalike must be an orphan, got %+v", c)
	}
}

func TestNilKeyIsAnError(t *testing.T) {
	reset()
	var missing *StringKey
	App("a", "A", Group("g", "G", missing))
	if err := Export(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "nil key or group") {
		t.Errorf("got %v", err)
	}
}

func TestGroupInTwoPlaces(t *testing.T) {
	reset()
	shared := Group("shared", "Shared", Int("x", "X"))
	App("a", "A", Group("one", "One", shared), Group("two", "Two", shared))
	if err := Export(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "used twice, as one.shared and as two.shared") {
		t.Errorf("got %v", err)
	}
}

func TestNestedPathsAndTags(t *testing.T) {
	reset()
	mailer := Tag("Mailer", "Mail")
	retry := Group("retry", "Retry", Int("max", "Max"))
	sched := Group("scheduler", "Scheduler", retry).Tags(mailer)
	App("a", "A", sched).Global(retry)
	Command("Retry").Uses(retry, mailer)
	var buf bytes.Buffer
	if err := Export(&buf); err != nil {
		t.Fatal(err)
	}
	a, err := ir.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	c := a.Commands[0]
	if strings.Join(c.Uses, ",") != "scheduler.retry" || strings.Join(c.Tags, ",") != "Mailer" {
		t.Errorf("command: %+v", c)
	}
	if strings.Join(a.Global, ",") != "scheduler.retry" || len(a.Tags) != 1 || a.Groups[0].Tags[0] != "Mailer" {
		t.Errorf("app: global %v, tags %+v, group tags %v", a.Global, a.Tags, a.Groups[0].Tags)
	}
}
