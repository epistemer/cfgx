package gen

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/golden"
	"github.com/epistemer/cfgx/internal/ir"
)

func str(s string) *string { return &s }

func sample() *ir.App {
	return &ir.App{
		Version: ir.Version, Name: "demo", Desc: "Demo app", EnvPrefix: "DEMO",
		Groups: []ir.Group{
			{Name: "database", Desc: "Database connection", Keys: []ir.Key{
				{Name: "host", Desc: "Database host", Kind: ir.KindString, Default: str("localhost")},
				{Name: "port", Desc: "Database port", Kind: ir.KindInt, Default: str("5432")},
			}},
			{Name: "smtp", Desc: "Outgoing mail", Keys: []ir.Key{
				{Name: "url", Desc: "Server URL", Kind: ir.KindString},
			}},
			{Name: "log", Desc: "Logging", Keys: []ir.Key{
				{Name: "debug", Desc: "Debug logging", Kind: ir.KindBool, Default: str("false")},
			}},
		},
		Commands: []ir.Command{
			{Name: "Serve", Uses: []string{"database", "smtp", "log"}},
			{Name: "Migrate", Uses: []string{"database"}},
			{Name: "Version", Uses: []string{}},
		},
	}
}

func TestGolden(t *testing.T) {
	m, err := Build(sample(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Render(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	golden.Assert(t, filepath.Join("testdata", "settings.golden"), src)

	again, _ := Render(m)
	if !bytes.Equal(src, again) {
		t.Error("rendering twice gave different output")
	}
	if strings.Contains(string(src), "cfgx/dsl") {
		t.Error("generated code must never import cfgx/dsl")
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"database": "Database", "smtp_url": "SMTPURL", "db": "DB", "max_conns": "MaxConns", "otp_ttl": "OTPTTL"} {
		if got := pascal(in); got != want {
			t.Errorf("pascal(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"DB": "db", "SMTPHost": "smtpHost", "Database": "database", "SchedulerRetry": "schedulerRetry", "X": "x"} {
		if got := lowerFirstWord(in); got != want {
			t.Errorf("lowerFirstWord(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChecks(t *testing.T) {
	cases := []struct {
		name string
		edit func(a *ir.App)
		want string
	}{
		{"empty description", func(a *ir.App) { a.Groups[0].Keys[0].Desc = " " }, "needs a description"},
		{"bad key name", func(a *ir.App) { a.Groups[0].Keys[0].Name = "Host" }, `key name "Host"`},
		{"duplicate key", func(a *ir.App) { a.Groups[0].Keys[1].Name = "host" }, "declared twice"},
		{"duplicate group", func(a *ir.App) { a.Groups[1].Name = "database" }, `group "database" is declared twice`},
		{"flag collision", func(a *ir.App) {
			a.Groups = append(a.Groups, ir.Group{Name: "database_host", Desc: "x", Keys: []ir.Key{{Name: "x", Desc: "x", Kind: ir.KindString}}})
			a.Groups[0].Keys = append(a.Groups[0].Keys, ir.Key{Name: "host_x", Desc: "x", Kind: ir.KindString})
		}, "flag --database-host-x"},
		{"unknown group", func(a *ir.App) { a.Commands[0].Uses = append(a.Commands[0].Uses, "nope") }, `"nope", which isn't in the dsl.App`},
		{"bad command name", func(a *ir.App) { a.Commands[0].Name = "serve" }, "exported Go identifier"},
		{"command collides with group", func(a *ir.App) { a.Commands[0].Name = "Database" }, "collides"},
		{"bad int default", func(a *ir.App) { a.Groups[0].Keys[1].Default = str("x") }, "isn't an int"},
		{"bad env prefix", func(a *ir.App) { a.EnvPrefix = "demo" }, "env prefix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := sample()
			c.edit(a)
			_, err := Build(a, "settings")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestDefaultEnvPrefix(t *testing.T) {
	a := sample()
	a.EnvPrefix = ""
	a.Name = "my-app"
	m, err := Build(a, "settings")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Keys[0].Env; got != "MY_APP_DATABASE_HOST" {
		t.Errorf("env = %q", got)
	}
}
