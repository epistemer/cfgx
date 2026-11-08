package gen

import (
	"bytes"
	"os"
	"os/exec"
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
		ConfigFile: "config", SearchPaths: []string{".", "$HOME/.demo"},
		Global: []string{"log"},
		Groups: []ir.Group{
			{Name: "database", Desc: "Database connection", Keys: []ir.Key{
				{Name: "host", Desc: "Database host", Kind: ir.KindString, Default: str("localhost")},
				{Name: "port", Desc: "Database port", Kind: ir.KindInt, Default: str("5432")},
			}},
			{Name: "smtp", Desc: "Outgoing mail", Keys: []ir.Key{
				{Name: "url", Desc: "Server URL", Kind: ir.KindString, Required: true},
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
	for in, want := range map[string]string{"database": "database", "db": "db", "smtp_tls": "smtpTLS", "db_1": "db1", "http_api": "httpAPI", "scheduler_retry": "schedulerRetry", "a_db": "aDB"} {
		if got := lowerCamel(in); got != want {
			t.Errorf("lowerCamel(%q) = %q, want %q", in, got, want)
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
		{"reserved getter", func(a *ir.App) {
			a.Groups = append(a.Groups, ir.Group{Name: "on", Desc: "x", Keys: []ir.Key{{Name: "change", Desc: "x", Kind: ir.KindBool}}})
		}, "getter OnChange of on.change is reserved"},
		{"orphan group", func(a *ir.App) {
			a.Commands[0].Orphans = []ir.Ref{{Name: "extra", Pos: ir.Pos{File: "schema.go", Line: 9}}}
		}, `uses group "extra" (declared at schema.go:9), which isn't passed to dsl.App`},
		{"bad derived env prefix", func(a *ir.App) { a.EnvPrefix = ""; a.Name = "123app" }, "derived from the app name"},
		{"trailing underscore", func(a *ir.App) { a.Groups[0].Keys[0].Name = "host_" }, `key name "host_"`},
		{"double underscore", func(a *ir.App) { a.Groups[0].Keys[0].Name = "a__b" }, `key name "a__b"`},
		{"required with default", func(a *ir.App) { a.Groups[0].Keys[0].Required = true }, "Required has no effect"},
		{"global orphan", func(a *ir.App) {
			a.GlobalOrphans = []ir.Ref{{Name: "extra", Pos: ir.Pos{File: "schema.go", Line: 9}}}
		}, `Global lists group "extra" (declared at schema.go:9), which isn't passed to dsl.App`},
		{"global unknown", func(a *ir.App) { a.Global = append(a.Global, "nope") }, `Global lists group "nope"`},
		{"global twice", func(a *ir.App) { a.Global = append(a.Global, "log") }, `Global lists group "log" twice`},
		{"config file with directory", func(a *ir.App) { a.ConfigFile = "etc/config" }, "without a directory"},
		{"config file with extension", func(a *ir.App) { a.ConfigFile = "config.yaml" }, "must not have an extension"},
		{"search paths without basename", func(a *ir.App) { a.ConfigFile = "" }, "ConfigFile needs a basename"},
		{"empty search path", func(a *ir.App) { a.SearchPaths = []string{""} }, "empty search path"},
		{"unknown format", func(a *ir.App) { a.Formats = []string{"toml"} }, `unknown config file format "toml"`},
		{"format twice", func(a *ir.App) { a.Formats = []string{"yaml", "yaml"} }, "Formats lists yaml twice"},
		{"formats without config file", func(a *ir.App) { a.ConfigFile = ""; a.SearchPaths = nil; a.Formats = []string{"json"} }, "Formats has no effect"},
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

func TestBadPackageName(t *testing.T) {
	for _, pkg := range []string{"", "type", "_", "../x"} {
		if _, err := Build(sample(), pkg); err == nil || !strings.Contains(err.Error(), "package name") {
			t.Errorf("package %q: got %v", pkg, err)
		}
	}
}

func TestGlobalGroups(t *testing.T) {
	m, err := Build(sample(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.GlobalIdx) != 1 || m.GlobalIdx[0] != 3 || !m.Keys[3].Global {
		t.Fatalf("GlobalIdx = %v", m.GlobalIdx)
	}
	for _, c := range m.Commands {
		if len(c.Groups) == 0 || c.Groups[0].Name != "log" {
			t.Errorf("%s doesn't get the global group first: %v", c.Name, c.Groups)
		}
		n := 0
		for _, g := range c.Groups {
			if g.Name == "log" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s has the global group %d times", c.Name, n)
		}
	}
}

func TestDefaultsAreCanonical(t *testing.T) {
	a := sample()
	a.Groups[0].Keys[1].Default = str("+007")
	a.Groups[2].Keys[0].Default = str("TRUE")
	m, err := Build(a, "settings")
	if err != nil {
		t.Fatal(err)
	}
	if m.Keys[1].DefaultLit != "7" || m.Keys[3].DefaultLit != "true" {
		t.Errorf("defaults not canonical: %q, %q", m.Keys[1].DefaultLit, m.Keys[3].DefaultLit)
	}
}

// TestGeneratedCodeCompiles vets the generated code for schemas at the edges:
// no commands, no groups, an empty group, a command without groups, and
// names made of initialisms and digits.
func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	cases := map[string]func(a *ir.App){
		"sample":         func(a *ir.App) {},
		"no config file": func(a *ir.App) { a.ConfigFile = ""; a.SearchPaths = nil; a.Global = nil },
		"formats":        func(a *ir.App) { a.Formats = []string{"json"}; a.SearchPaths = nil },
		"no commands":    func(a *ir.App) { a.Commands = nil },
		"no groups": func(a *ir.App) {
			a.Groups = nil
			a.Global = nil
			a.Commands = []ir.Command{{Name: "Version", Uses: []string{}}}
		},
		"empty group": func(a *ir.App) { a.Groups = append(a.Groups, ir.Group{Name: "empty", Desc: "Nothing yet"}) },
		"initialisms and digits": func(a *ir.App) {
			a.Groups = append(a.Groups,
				ir.Group{Name: "smtp_tls", Desc: "x", Keys: []ir.Key{{Name: "ca_url", Desc: "x", Kind: ir.KindString}}},
				ir.Group{Name: "db_1", Desc: "x", Keys: []ir.Key{{Name: "id", Desc: "x", Kind: ir.KindInt}}})
			a.Commands = append(a.Commands, ir.Command{Name: "HTTP2", Uses: []string{"smtp_tls", "db_1"}})
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			a := sample()
			edit(a)
			m, err := Build(a, "settings")
			if err != nil {
				t.Fatal(err)
			}
			src, err := Render(m)
			if err != nil {
				t.Fatalf("%v\n%s", err, src)
			}
			// Inside this module, so the generated code can import cfgx.
			dir, err := os.MkdirTemp(".", "zzcompile")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			if err := os.WriteFile(filepath.Join(dir, "zz_settings.go"), src, 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("go", "vet", "./"+filepath.Base(dir)).CombinedOutput(); err != nil {
				t.Fatalf("generated code doesn't vet: %v\n%s\n%s", err, out, src)
			}
		})
	}
}
