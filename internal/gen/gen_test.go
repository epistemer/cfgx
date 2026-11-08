package gen

import (
	"bytes"
	"fmt"
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

// nested is a schema with the structure M5.1 adds: a nested group, GoName,
// FlagPrefix, dotted key names and App.Initialisms.
func nested() *ir.App {
	empty := ""
	return &ir.App{
		Version: ir.Version, Name: "zp", Desc: "Nested", EnvPrefix: "ZP",
		Initialisms: []string{"OTP"},
		Groups: []ir.Group{
			{Name: "database", Desc: "Database connection", GoName: "DB", Keys: []ir.Key{
				{Name: "host", Desc: "Host", Kind: ir.KindString},
			}},
			{Name: "server", Desc: "HTTP server", FlagPrefix: &empty, Keys: []ir.Key{
				{Name: "port", Desc: "Port", Kind: ir.KindInt, Default: str("7777")},
				{Name: "tls.enabled", Desc: "Serve HTTPS", Kind: ir.KindBool},
				{Name: "tls.cert", Desc: "TLS certificate", Kind: ir.KindString, Default: str("cert.pem")},
			}},
			{Name: "scheduler", Desc: "Task scheduler", Keys: []ir.Key{
				{Name: "workers", Desc: "Workers", Kind: ir.KindInt},
			}, Groups: []ir.Group{
				{Name: "retry", Desc: "Retry policy", Keys: []ir.Key{
					{Name: "max", Desc: "Maximum retries", Kind: ir.KindInt, Default: str("3")},
				}},
			}},
			{Name: "auth", Desc: "Authentication", Keys: []ir.Key{
				{Name: "otp.length", Desc: "OTP length", Kind: ir.KindInt},
				{Name: "emailfrom", Desc: "Sender address for auth emails", Kind: ir.KindString, Tags: []string{"Mailer"}},
				{Name: "token", Desc: "Static API token", Kind: ir.KindString},
				{Name: "keys", Desc: "API keys file", Kind: ir.KindString},
			}},
			{Name: "smtp", Desc: "Outgoing mail", Tags: []string{"Mailer"}, Keys: []ir.Key{
				{Name: "host", Desc: "SMTP server host", Kind: ir.KindString},
				{Name: "port", Desc: "SMTP server port", Kind: ir.KindInt, Default: str("587")},
				{Name: "tls", Desc: "Use implicit TLS", Kind: ir.KindBool},
			}},
		},
		Tags: []ir.Tag{{Name: "Mailer", Desc: "Everything the mailer needs"}},
		Rules: []ir.Rule{
			{Kind: ir.RuleAtMostOne, Group: "auth", Keys: []string{"auth.token", "auth.keys"}},
			{Kind: ir.RuleRelevantIf, Group: "server", Keys: []string{"server.tls.cert", "server.tls.enabled"}},
		},
		Commands: []ir.Command{
			{Name: "Serve", Uses: []string{"database", "server", "scheduler", "scheduler.retry", "auth", "smtp"}, Tags: []string{"Mailer"}},
			{Name: "Retry", Uses: []string{"scheduler.retry"}},
			{Name: "TestEmail", Uses: []string{}, Tags: []string{"Mailer"}},
			{Name: "Notify", Uses: []string{"smtp"}, Tags: []string{"Mailer"}},
		},
	}
}

func TestNestedGolden(t *testing.T) {
	m, err := Build(nested(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Render(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	golden.Assert(t, filepath.Join("testdata", "nested.golden"), src)
	byPath := map[string]*KeyModel{}
	for _, k := range m.Keys {
		byPath[k.Path] = k
	}
	for path, want := range map[string][3]string{
		"database.host":       {"DBHost", "--database-host", "ZP_DATABASE_HOST"},
		"server.port":         {"ServerPort", "--port", "ZP_SERVER_PORT"},
		"server.tls.enabled":  {"ServerTLSEnabled", "--tls-enabled", "ZP_SERVER_TLS_ENABLED"},
		"scheduler.retry.max": {"SchedulerRetryMax", "--scheduler-retry-max", "ZP_SCHEDULER_RETRY_MAX"},
		"auth.otp.length":     {"AuthOTPLength", "--auth-otp-length", "ZP_AUTH_OTP_LENGTH"},
		"scheduler.workers":   {"SchedulerWorkers", "--scheduler-workers", "ZP_SCHEDULER_WORKERS"},
	} {
		k := byPath[path]
		if k == nil {
			t.Errorf("no key %s", path)
			continue
		}
		if got := [3]string{k.Getter, "--" + k.Flag, k.Env}; got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
	types := map[string]string{}
	for _, g := range m.Groups {
		types[g.Path] = g.Iface + " " + g.Type
	}
	for path, want := range map[string]string{
		"database": "DB dbGroup", "server": "Server serverGroup",
		"scheduler": "Scheduler schedulerGroup", "scheduler.retry": "SchedulerRetry schedulerRetryGroup",
	} {
		if types[path] != want {
			t.Errorf("%s: got %q, want %q", path, types[path], want)
		}
	}
	// Serve uses scheduler and scheduler.retry: retry is reached through
	// scheduler, not embedded twice.
	for _, c := range m.Commands {
		var names []string
		for _, g := range c.Groups {
			names = append(names, g.Path)
		}
		var fwd []string
		for _, k := range c.Forward {
			fwd = append(fwd, k.Getter)
		}
		want := map[string][2]string{
			"Serve":     {"database,server,scheduler,auth,smtp", ""},
			"Retry":     {"scheduler.retry", ""},
			"TestEmail": {"", "AuthEmailfrom,SMTPHost,SMTPPort,SMTPTLS"},
			// Notify embeds smtp whole, so only the auth key is forwarded.
			"Notify": {"smtp", "AuthEmailfrom"},
		}[c.Name]
		if got := [2]string{strings.Join(names, ","), strings.Join(fwd, ",")}; got != want {
			t.Errorf("%s: embeds and forwards %q, want %q", c.Name, got, want)
		}
	}
	if len(m.Tags) != 1 || len(m.Tags[0].Keys) != 4 {
		t.Errorf("Mailer tag: %+v", m.Tags)
	}
}

func TestRules(t *testing.T) {
	a := sample()
	empty := ""
	a.Groups[0].Keys[0].Default = &empty // a zero default is fine in exclusive rules
	a.Rules = []ir.Rule{
		{Kind: ir.RuleAtMostOne, Group: "database", Keys: []string{"database.host", "smtp.url"}},
		{Kind: ir.RuleRelevantIf, Group: "database", Keys: []string{"database.port", "log.debug"}},
	}
	m, err := Build(a, "settings")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Rules) != 1 || m.Rules[0].Kind != "AtMostOne" || fmt.Sprint(m.Rules[0].Keys) != "[0 2]" {
		t.Errorf("rules: %+v", m.Rules)
	}
	if fmt.Sprint(m.Keys[1].RelevantIf) != "[3]" {
		t.Errorf("RelevantIf = %v", m.Keys[1].RelevantIf)
	}
	// Migrate uses only database, but loads log.debug as well, the condition
	// of database.port.
	for _, c := range m.Commands {
		if c.Name == "Migrate" && fmt.Sprint(c.KeyIdx) != "[0 1 3]" {
			t.Errorf("Migrate loads %v", c.KeyIdx)
		}
	}
}

func TestLowerGo(t *testing.T) {
	for in, want := range map[string]string{"DB": "db", "HTTPServer": "httpServer", "Mail": "mail", "MyDB": "myDB", "X": "x",
		"DB1": "db1", "HTTP2": "http2", "HTTP2Server": "http2Server", "S3Bucket": "s3Bucket"} {
		if got := lowerGo(in); got != want {
			t.Errorf("lowerGo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"database": "Database", "smtp_url": "SMTPURL", "db": "DB", "max_conns": "MaxConns", "otp_ttl": "OTPTTL"} {
		if got := newNamer().pascal(in); got != want {
			t.Errorf("pascal(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"database": "database", "db": "db", "smtp_tls": "smtpTLS", "db_1": "db1", "http_api": "httpAPI", "scheduler_retry": "schedulerRetry", "a_db": "aDB"} {
		if got := newNamer().lowerCamel(in); got != want {
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
		{"command named Config", func(a *ir.App) { a.Commands[0].Name = "Config" }, "command binder ConfigCommand collides"},
		{"command named All", func(a *ir.App) { a.Commands[0].Name = "All" }, "command type All collides"},
		{"group named values", func(a *ir.App) { a.Groups[1].Name = "values" }, "group interface Values collides"},
		{"global orphan", func(a *ir.App) {
			a.GlobalOrphans = []ir.Ref{{Name: "extra", Pos: ir.Pos{File: "schema.go", Line: 9}}}
		}, `Global lists group "extra" (declared at schema.go:9), which isn't passed to dsl.App`},
		{"bad GoName", func(a *ir.App) { a.Groups[0].GoName = "db" }, `GoName "db" of group "database"`},
		{"bad FlagPrefix", func(a *ir.App) { p := "Srv_"; a.Groups[0].FlagPrefix = &p }, `FlagPrefix "Srv_"`},
		{"bad initialism", func(a *ir.App) { a.Initialisms = []string{"otp"} }, `initialism "otp"`},
		{"bad dotted key", func(a *ir.App) { a.Groups[0].Keys[0].Name = "tls..x" }, `key name "tls..x"`},
		{"key under a key", func(a *ir.App) {
			a.Groups[0].Keys = append(a.Groups[0].Keys, ir.Key{Name: "host.x", Desc: "x", Kind: ir.KindString})
		}, "file key database.host.x needs database.host to be a map"},
		{"key and nested group share a path", func(a *ir.App) {
			a.Groups[0].Keys = append(a.Groups[0].Keys, ir.Key{Name: "pool.size", Desc: "x", Kind: ir.KindInt})
			a.Groups[0].Groups = []ir.Group{{Name: "pool", Desc: "x", Keys: []ir.Key{{Name: "size", Desc: "x", Kind: ir.KindInt}}}}
		}, "file key database.pool.size"},
		{"key and nested group share a name", func(a *ir.App) {
			a.Groups[0].Groups = []ir.Group{{Name: "host", Desc: "x"}}
		}, `group "database" has two members named "host"`},
		{"flag collision from FlagPrefix", func(a *ir.App) {
			e := ""
			a.Groups[0].FlagPrefix, a.Groups[1].FlagPrefix = &e, &e
			a.Groups[1].Keys = append(a.Groups[1].Keys, ir.Key{Name: "host", Desc: "x", Kind: ir.KindString})
		}, "flag --host"},
		{"rule with one key", func(a *ir.App) {
			a.Rules = []ir.Rule{{Kind: ir.RuleAtMostOne, Group: "smtp", Keys: []string{"smtp.url"}}}
		}, "AtMostOne needs at least two keys"},
		{"requires with three keys", func(a *ir.App) {
			a.Rules = []ir.Rule{{Kind: ir.RuleRequires, Group: "database", Keys: []string{"database.host", "database.port", "smtp.url"}}}
		}, "Requires takes two keys"},
		{"rule on an unknown key", func(a *ir.App) {
			a.Rules = []ir.Rule{{Kind: ir.RuleRequires, Group: "smtp", Keys: []string{"smtp.url", "nope.x"}}}
		}, "Requires refers to key nope.x"},
		{"rule lists a key twice", func(a *ir.App) {
			a.Rules = []ir.Rule{{Kind: ir.RuleAllOrNone, Group: "smtp", Keys: []string{"smtp.url", "smtp.url"}}}
		}, "AllOrNone lists smtp.url twice"},
		{"exclusive key with a default", func(a *ir.App) {
			a.Rules = []ir.Rule{{Kind: ir.RuleExactlyOne, Group: "database", Keys: []string{"database.port", "smtp.url"}}}
		}, "ExactlyOne can't include database.port, whose default 5432 counts as set"},
		{"RelevantIf cycle", func(a *ir.App) {
			a.Rules = []ir.Rule{
				{Kind: ir.RuleRelevantIf, Group: "smtp", Keys: []string{"smtp.url", "log.debug"}},
				{Kind: ir.RuleRelevantIf, Group: "log", Keys: []string{"log.debug", "smtp.url"}},
			}
		}, "form a cycle"},
		{"tag on no keys", func(a *ir.App) { a.Tags = []ir.Tag{{Name: "Mailer", Desc: "x"}} }, "tag Mailer isn't on any key"},
		{"bad tag name", func(a *ir.App) {
			a.Tags = []ir.Tag{{Name: "mailer", Desc: "x"}}
			a.Groups[1].Tags = []string{"mailer"}
		}, `tag name "mailer" must be an exported Go identifier`},
		{"tag collides with a group interface", func(a *ir.App) {
			a.Tags = []ir.Tag{{Name: "Database", Desc: "x"}}
			a.Groups[1].Tags = []string{"Database"}
		}, "collides"},
		{"tag declared twice", func(a *ir.App) {
			a.Tags = []ir.Tag{{Name: "Mailer", Desc: "x"}, {Name: "Mailer", Desc: "y"}}
			a.Groups[1].Tags = []string{"Mailer"}
		}, "tag Mailer is declared twice"},
		{"key with an undeclared tag", func(a *ir.App) { a.Groups[0].Keys[0].Tags = []string{"Mailr"} }, "key database.host has tag Mailr, which isn't declared"},
		{"group with an undeclared tag", func(a *ir.App) { a.Groups[0].Tags = []string{"Mailr"} }, "group database has tag Mailr, which isn't declared"},
		{"command uses an unknown tag", func(a *ir.App) { a.Commands[0].Tags = []string{"Nope"} }, "uses tag Nope, which isn't declared"},
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
		"nested":         func(a *ir.App) { *a = *nested() },
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
			if name == "nested" {
				// The interfaces each command type must implement (A5).
				assert := `package settings

var (
	_ Mailer         = (*TestEmail)(nil)
	_ Mailer         = (*Notify)(nil)
	_ Mailer         = (*Serve)(nil)
	_ SMTP           = (*Notify)(nil)
	_ Scheduler      = (*Serve)(nil)
	_ SchedulerRetry = (*Serve)(nil)
	_ SchedulerRetry = (*Retry)(nil)
	_ DB             = (*Serve)(nil)
)
`
				if err := os.WriteFile(filepath.Join(dir, "assert.go"), []byte(assert), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("go", "vet", "./"+filepath.Base(dir)).CombinedOutput(); err != nil {
				t.Fatalf("generated code doesn't vet: %v\n%s\n%s", err, out, src)
			}
		})
	}
}
