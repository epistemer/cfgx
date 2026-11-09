package gen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/ir"
)

func TestComputedRelativeOutFile(t *testing.T) {
	m, dir, err := resolve(t, computedApp([]string{"DSN"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range m.Groups {
		for _, c := range g.Computed {
			c.Sig = ""
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, filepath.Join(dir, "zz_settings.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveComputed(context.Background(), m, dir, rel); err != nil {
		t.Fatal(err)
	}
	if m.Groups[0].Computed[0].Sig != "DSN() string" {
		t.Errorf("sig = %q", m.Groups[0].Computed[0].Sig)
	}
}

// computedApp has a DB group with DSN, a scheduler with a nested retry group,
// and a command using both.
func computedApp(dbComputed, schedComputed []string) *ir.App {
	return &ir.App{
		Version: ir.Version, Name: "zp", Desc: "Computed", EnvPrefix: "ZP",
		Groups: []ir.Group{
			{Name: "database", Desc: "Database", GoName: "DB", Computed: dbComputed, Keys: []ir.Key{
				{Name: "host", Desc: "Host", Kind: ir.KindString},
			}},
			{Name: "scheduler", Desc: "Scheduler", Computed: schedComputed, Keys: []ir.Key{
				{Name: "workers", Desc: "Workers", Kind: ir.KindInt},
			}, Groups: []ir.Group{{Name: "retry", Desc: "Retry", Keys: []ir.Key{
				{Name: "max", Desc: "Max", Kind: ir.KindInt},
			}}}},
		},
		Commands: []ir.Command{{Name: "Serve", Uses: []string{"database", "scheduler"}}},
	}
}

// hand is the hand-written file of the settings package. It imports
// math/rand/v2 under another name and declares rand, so the generated file
// must import the package as rand2.
const hand = `package settings

import (
	"time"

	mrand "math/rand/v2"
)

var rand = 1

func (g *dbGroup) DSN() string { return "postgres://" + g.DBHost() }

func (g *dbGroup) Rand() *mrand.Rand { return nil }

func (g *schedulerGroup) RetryPolicy() (time.Duration, error) { return time.Second, nil }

func (g *schedulerGroup) Named() (n int, err error) { return }

func (g *schedulerGroup) Takes(x int) int { return x }

func (g *schedulerRetryGroup) Backoff() time.Duration { return time.Second }
`

// resolve builds app, type-checks it against hand in a package inside this
// module, and returns the model and the directory.
func resolve(t *testing.T, app *ir.App) (*Model, string, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the go command")
	}
	m, err := Build(app, "settings")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(".", "zzcomputed")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "settings.go"), []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = ResolveComputed(context.Background(), m, abs, filepath.Join(abs, "zz_settings.go"))
	return m, abs, err
}

func TestComputed(t *testing.T) {
	m, dir, err := resolve(t, computedApp([]string{"DSN", "Rand"}, []string{"RetryPolicy", "Named"}))
	if err != nil {
		t.Fatal(err)
	}
	sigs := map[string]string{}
	for _, g := range m.Groups {
		for _, c := range g.Computed {
			sigs[c.Name] = c.Sig
		}
	}
	for name, want := range map[string]string{
		"DSN":         "DSN() string",
		"Rand":        "Rand() *rand2.Rand",
		"RetryPolicy": "RetryPolicy() (time.Duration, error)",
		"Named":       "Named() (n int, err error)",
	} {
		if sigs[name] != want {
			t.Errorf("%s: got %q, want %q", name, sigs[name], want)
		}
	}
	src, err := Render(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	for _, want := range []string{`\trand2 "math/rand/v2"`, `\t"time"`, `\tDSN\(\) string\s+// computed`, `\tRetryPolicy\(\) \(time.Duration, error\)\s+// computed`} {
		if !regexp.MustCompile(want).Match(src) {
			t.Errorf("generated file lacks %s", want)
		}
	}
	// The command embeds both groups whole, so it implements both
	// interfaces, computed methods included.
	assert := "package settings\n\nvar (\n\t_ DB = (*Serve)(nil)\n\t_ Scheduler = (*Serve)(nil)\n)\n"
	for name, content := range map[string]string{"zz_settings.go": string(src), "assert.go": assert} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("go", "vet", "./"+filepath.Base(dir)).CombinedOutput(); err != nil {
		t.Fatalf("generated code doesn't vet: %v\n%s\n%s", err, out, src)
	}
}

func TestComputedProblems(t *testing.T) {
	cases := []struct {
		name          string
		db, scheduler []string
		want          string
	}{
		{"missing", []string{"Nope"}, nil, `group "database" declares computed method Nope, but *dbGroup has no method Nope`},
		{"parameters", nil, []string{"Takes"}, `computed method Takes of group "scheduler" takes parameters`},
		{"promoted", nil, []string{"Backoff"}, `computed method Backoff of group "scheduler" is promoted from a nested group`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := resolve(t, computedApp(c.db, c.scheduler))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestComputedChecks(t *testing.T) {
	cases := []struct {
		name string
		edit func(a *ir.App)
		want string
	}{
		{"unexported", func(a *ir.App) { a.Groups[0].Computed = []string{"dsn"} }, `computed method "dsn" of group "database" must be an exported`},
		{"reserved", func(a *ir.App) { a.Groups[0].Computed = []string{"Close"} }, "computed method Close of group \"database\" is reserved"},
		{"twice", func(a *ir.App) { a.Groups[0].Computed = []string{"DSN", "DSN"} }, "lists computed method DSN twice"},
		{"same as a getter", func(a *ir.App) { a.Groups[0].Computed = []string{"DBHost"} }, `group "database"'s interface has two methods named DBHost`},
		{"same as a nested getter", func(a *ir.App) { a.Groups[1].Computed = []string{"SchedulerRetryMax"} }, "two methods named SchedulerRetryMax"},
		{"nested computed same as a parent getter", func(a *ir.App) { a.Groups[1].Groups[0].Computed = []string{"SchedulerWorkers"} }, `group "scheduler"'s interface has two methods named SchedulerWorkers`},
		{"two groups on one command", func(a *ir.App) {
			a.Groups[0].Computed = []string{"Ping"}
			a.Groups[1].Computed = []string{"Ping"}
		}, `command Serve gets method Ping from both database and group "scheduler"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := computedApp(nil, nil)
			c.edit(a)
			_, err := Build(a, "settings")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want %q", err, c.want)
			}
		})
	}
}
