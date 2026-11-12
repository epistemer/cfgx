package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/golden"
	"github.com/epistemer/cfgx/internal/ir"
	"github.com/epistemer/cfgx/internal/node"
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
				{Name: "host", Desc: "Host", Kind: ir.KindString, Aliases: []string{"database.hostname"}, Deprecated: "use host instead"},
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
			{Name: "TestEmail", Uses: []string{}, Tags: []string{"Mailer"}, Args: []ir.Key{
				{Name: "to.address", Desc: "Recipient", Kind: ir.KindString, Required: true},
			}},
			{Name: "Notify", Uses: []string{"smtp"}, Tags: []string{"Mailer"}},
			{Name: "Migrate", Uses: []string{}, Args: []ir.Key{
				{Name: "down", Desc: "Apply down migrations", Kind: ir.KindBool},
				{Name: "steps", Desc: "Number of steps", Kind: ir.KindInt},
			}},
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
		"to.address":          {"ToAddress", "--to-address", ""},
		"down":                {"Down", "--down", ""},
		"steps":               {"Steps", "--steps", ""},
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
			"Notify":  {"smtp", "AuthEmailfrom"},
			"Migrate": {"", ""},
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
}

// allKinds has one key of every kind, with and without defaults.
func allKinds() *ir.App {
	return &ir.App{
		Version: ir.Version, Name: "kinds", Desc: "Kinds", EnvPrefix: "K",
		Groups: []ir.Group{{Name: "k", Desc: "Every kind", Keys: []ir.Key{
			{Name: "s", Desc: "string", Kind: ir.KindString, Default: str("x")},
			{Name: "b", Desc: "bool", Kind: ir.KindBool, Default: str("true")},
			{Name: "i", Desc: "int", Kind: ir.KindInt, Default: str("-3")},
			{Name: "i64", Desc: "int64", Kind: ir.KindInt64, Default: str("9223372036854775807")},
			{Name: "u", Desc: "uint", Kind: ir.KindUint, Default: str("7")},
			{Name: "f", Desc: "float", Kind: ir.KindFloat, Default: str("0.25")},
			{Name: "d", Desc: "duration", Kind: ir.KindDuration, Default: str("300000000000ns")},
			{Name: "d2", Desc: "odd duration", Kind: ir.KindDuration, Default: str("1500ms")},
			{Name: "size", Desc: "bytesize", Kind: ir.KindByteSize, Default: str("1048576")},
			{Name: "none", Desc: "no default", Kind: ir.KindDuration},
			{Name: "path", Desc: "path", Kind: ir.KindPath, Default: str("data//app.db")},
			{Name: "file", Desc: "file", Kind: ir.KindFile, Default: str("ca.pem")},
			{Name: "url", Desc: "url", Kind: ir.KindURL, Default: str("https://example.com/x")},
			{Name: "loc", Desc: "location", Kind: ir.KindLocation, Default: str("UTC")},
			{Name: "url2", Desc: "url without default", Kind: ir.KindURL},
			{Name: "tags", Desc: "strings", Kind: ir.KindStrings, Default: str(`a,"b,c",""`)},
			{Name: "ports", Desc: "ints", Kind: ir.KindInts, Default: str("80, 443")},
			{Name: "backoff", Desc: "durations", Kind: ir.KindDurations, Default: str("1000ms,1m")},
			{Name: "labels", Desc: "stringmap", Kind: ir.KindStringMap, Default: str("b=2,a=x=y")},
			{Name: "none2", Desc: "empty list", Kind: ir.KindStrings, Default: str("")},
			{Name: "cidr", Desc: "text", Kind: ir.KindText, Default: str("10.0.0.0/8"),
				Type: &ir.Type{Path: "net/netip", Pkg: "netip", Name: "Prefix", Kind: "struct"}},
			{Name: "port", Desc: "checked int", Kind: ir.KindInt, Default: str("80"),
				Checks: []ir.Check{{Kind: ir.CheckMin, Args: []string{"1"}}, {Kind: ir.CheckMax, Args: []string{"65535"}}}},
			// The golden file is compiled, so Validate names a real function
			// of the right type; what it does doesn't matter here.
			{Name: "name", Desc: "checked string", Kind: ir.KindString,
				Checks:   []ir.Check{{Kind: ir.CheckLen, Args: []string{"1", "8"}}, {Kind: ir.CheckPattern, Args: []string{"[a-z]+"}}},
				Validate: &ir.FuncRef{Path: "errors", Name: "New"}},
			{Name: "cpus", Desc: "computed default", Kind: ir.KindInt, DefaultFunc: &ir.FuncRef{Path: "runtime", Name: "NumCPU"}},
			{Name: "wait", Desc: "checked duration", Kind: ir.KindDuration, Checks: []ir.Check{{Kind: ir.CheckMin, Args: []string{"1500ms"}}}},
			{Name: "nm", Desc: "renamed", Kind: ir.KindString, Short: "n", Flag: "name-flag", Env: "NAME_ENV", GoName: "Renamed", Hidden: true,
				Complete: &ir.Completion{Exts: []string{"pem", "crt"}}},
			{Name: "dir", Desc: "directory", Kind: ir.KindPath, Complete: &ir.Completion{Dirs: true}},
			{Name: "fk", Desc: "env only, renamed in the file", Kind: ir.KindString, FileKey: "file_key", Sources: []string{"env"}},
		}}},
		Commands: []ir.Command{{Name: "Run", Uses: []string{"k"}}},
	}
}

// mutableKinds is allKinds with a store, an enum, and every key that can
// be Mutable mutable.
func mutableKinds() *ir.App {
	a := allKinds()
	a.Store = "k.path"
	g := &a.Groups[0]
	g.Keys = append(g.Keys, ir.Key{Name: "day", Desc: "enum", Kind: ir.KindEnum,
		Type: &ir.Type{Path: "time", Pkg: "time", Name: "Weekday", Kind: "int"},
		Enum: []ir.EnumValue{{Text: "sunday", Lit: "0", Zero: true}, {Text: "monday", Lit: "1"}}})
	for i := range g.Keys {
		if k := &g.Keys[i]; k.Name != "path" && k.Kind != ir.KindFile {
			k.Mutable = true
		}
	}
	g.Keys[2].Epoch = 3
	return a
}

// storeCommands is a store app with three commands: Run has a mutable key,
// Admin asks for the store, and Plain has neither.
func storeCommands() *ir.App {
	return &ir.App{Version: ir.Version, Name: "app", Desc: "App", EnvPrefix: "APP", Store: "s.bolt",
		Groups: []ir.Group{
			{Name: "s", Desc: "Storage", Keys: []ir.Key{{Name: "bolt", Desc: "file", Kind: ir.KindPath, Default: str("app.db")}}},
			{Name: "rate", Desc: "Rate", Keys: []ir.Key{{Name: "limit", Desc: "limit", Kind: ir.KindInt, Mutable: true}}},
			{Name: "db", Desc: "DB", Keys: []ir.Key{{Name: "host", Desc: "host", Kind: ir.KindString}}},
		},
		Commands: []ir.Command{
			{Name: "Run", Uses: []string{"rate"}},
			{Name: "Admin", Uses: []string{"db"}, WithStore: true},
			{Name: "Plain", Uses: []string{"db"}},
		}}
}

// A command without the store has none of its methods, and one that loads
// no mutable key has no Update or OnChange: using them doesn't compile.
func TestNoStoreMethods(t *testing.T) {
	m, err := Build(storeCommands(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, use := range []string{"(*Plain)(nil).Store", "(*Plain)(nil).Close", "(*Admin)(nil).Update", "(*Admin)(nil).OnChange"} {
		dir, err := os.MkdirTemp(".", "zzcompile")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		for f, b := range map[string][]byte{"zz_settings.go": src, "use.go": []byte("package settings\n\nvar _ = " + use + "\n")} {
			if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		out, err := exec.Command("go", "vet", "./"+filepath.Base(dir)).CombinedOutput()
		if err == nil || !strings.Contains(string(out), use+" undefined") {
			t.Errorf("%s compiled, or failed otherwise: %v\n%s", use, err, out)
		}
	}
}

// A store's app gets an Update type with a setter and a reset per mutable
// key, and each mutable key a Format.
func TestMutableGolden(t *testing.T) {
	m, err := Build(mutableKinds(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Artifacts(m); err != nil {
		t.Fatal(err)
	}
	src, err := Render(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	golden.Assert(t, filepath.Join("testdata", "mutable.golden"), src)
}

func TestKindsGolden(t *testing.T) {
	m, err := Build(allKinds(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Render(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	golden.Assert(t, filepath.Join("testdata", "kinds.golden"), src)
	got := map[string][2]string{}
	for _, k := range m.Keys {
		got[k.Path] = [2]string{k.DefaultText, k.DefaultLit}
	}
	for path, want := range map[string][2]string{
		"k.d":    {"5m0s", "5 * time.Minute"},
		"k.d2":   {"1.5s", "1500 * time.Millisecond"},
		"k.size": {"1MiB", "1048576"},
		"k.f":    {"0.25", "0.25"},
		// Parsed when loading: no Go literal.
		"k.path":    {"data//app.db", ""},
		"k.file":    {"ca.pem", ""},
		"k.url":     {"https://example.com/x", ""},
		"k.loc":     {"UTC", ""},
		"k.tags":    {`[a,"b,c",""]`, `[]string{"a", "b,c", ""}`},
		"k.ports":   {"[80,443]", "[]int{80, 443}"},
		"k.backoff": {"[1s,1m0s]", "[]time.Duration{1 * time.Second, 1 * time.Minute}"},
		"k.labels":  {"[a=x=y,b=2]", `map[string]string{"a": "x=y", "b": "2"}`},
		"k.none2":   {"", "nil"},
		"k.cidr":    {"10.0.0.0/8", ""},
	} {
		if got[path] != want {
			t.Errorf("%s: got %q, want %q", path, got[path], want)
		}
	}
}

func TestKindDefaults(t *testing.T) {
	for kind, bad := range map[ir.Kind]string{
		ir.KindDuration: "30", ir.KindByteSize: "1.5MiB", ir.KindFloat: "NaN", ir.KindUint: "-1", ir.KindInt64: "x",
		ir.KindURL: "localhost:5432", ir.KindLocation: "Mars/Olympus_Mons",
		ir.KindInts: "1,,2", ir.KindDurations: "5", ir.KindStringMap: "a", ir.KindStrings: `"x`,
	} {
		a := allKinds()
		a.Groups[0].Keys = []ir.Key{{Name: "x", Desc: "x", Kind: kind, Default: str(bad)}}
		if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "default of k.x") {
			t.Errorf("%s default %q: got %v", kind, bad, err)
		}
	}
	// A zero duration default is allowed in exclusive rules.
	a := allKinds()
	a.Groups[0].Keys = []ir.Key{
		{Name: "a", Desc: "a", Kind: ir.KindDuration, Default: str("0s")},
		{Name: "b", Desc: "b", Kind: ir.KindDuration},
	}
	a.Rules = []ir.Rule{{Kind: ir.RuleAtMostOne, Group: "k", Keys: []string{"k.a", "k.b"}}}
	if _, err := Build(a, "settings"); err != nil {
		t.Error(err)
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
		{"bad int default", func(a *ir.App) { a.Groups[0].Keys[1].Default = str("x") }, `default of database.port: "x" is not an integer`},
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
				// database.host only leads into the cycle; the error names a
				// key on it.
				{Kind: ir.RuleRelevantIf, Group: "database", Keys: []string{"database.host", "smtp.url"}},
				{Kind: ir.RuleRelevantIf, Group: "smtp", Keys: []string{"smtp.url", "log.debug"}},
				{Kind: ir.RuleRelevantIf, Group: "log", Keys: []string{"log.debug", "smtp.url"}},
			}
		}, "RelevantIf rules on smtp.url form a cycle"},
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
		{"arg reserved getter", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "args", Desc: "x", Kind: ir.KindString}}
		}, "getter Args of args is reserved"},
		{"arg getter collides with group getter", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "database_host", Desc: "x", Kind: ir.KindString}}
		}, "getter DatabaseHost of database_host collides with another getter on Serve"},
		{"arg getter collides with forward getter", func(a *ir.App) {
			a.Tags = []ir.Tag{{Name: "T", Desc: "x"}}
			a.Groups[0].Keys[0].Tags = []string{"T"}
			a.Commands = []ir.Command{{Name: "C", Tags: []string{"T"}, Args: []ir.Key{{Name: "database_host", Desc: "x", Kind: ir.KindString}}}}
		}, "getter DatabaseHost of database_host collides with another getter on C"},
		{"arg getter collides with another arg", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{
				{Name: "step_count", Desc: "x", Kind: ir.KindInt},
				{Name: "step.count", Desc: "x", Kind: ir.KindInt},
			}
		}, "getter StepCount of step.count collides with another getter on Serve"},
		{"arg flag help", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "help", Desc: "x", Kind: ir.KindBool}}
		}, "flag --help of help collides"},
		{"arg flag config with config file", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "config", Desc: "x", Kind: ir.KindString}}
		}, "flag --config of config collides"},
		{"arg flag collides with loaded key flag", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "database.host", Desc: "x", Kind: ir.KindString}}
		}, "flag --database-host of database.host collides with another flag on Serve"},
		{"arg flag collides with global key flag", func(a *ir.App) {
			a.Commands[2].Args = []ir.Key{{Name: "log_debug", Desc: "x", Kind: ir.KindBool}}
		}, "flag --log-debug of log_debug collides with another flag on Version"},
		{"arg with tags", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "force", Desc: "x", Kind: ir.KindBool, Tags: []string{"Mailer"}}}
		}, "argument force of Serve has tags"},
		{"arg flag collides with a RelevantIf condition", func(a *ir.App) {
			a.Groups = append(a.Groups, ir.Group{Name: "x", Desc: "x", Keys: []ir.Key{{Name: "on", Desc: "x", Kind: ir.KindBool}}})
			a.Rules = []ir.Rule{{Kind: ir.RuleRelevantIf, Group: "database", Keys: []string{"database.host", "x.on"}}}
			a.Commands[1].Args = []ir.Key{{Name: "x.on", Desc: "x", Kind: ir.KindBool}}
		}, "flag --x-on of x.on collides with another flag on Migrate"},
		{"arg flag collides with another arg flag", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{
				{Name: "dry_run", Desc: "x", Kind: ir.KindBool},
				{Name: "dry.run", Desc: "x", Kind: ir.KindBool},
			}
		}, "flag --dry-run of dry.run collides with another flag on Serve"},
		{"arg field collides with group key field", func(a *ir.App) {
			a.Groups = append(a.Groups, ir.Group{Name: "serve", Desc: "x", Keys: []ir.Key{{Name: "host", Desc: "x", Kind: ir.KindString}}})
			a.Commands[0].Args = []ir.Key{{Name: "host", Desc: "x", Kind: ir.KindString}}
		}, "Go name ServeHost of host collides with another key or group"},
		{"bad alias path", func(a *ir.App) {
			a.Groups[0].Keys[0].Aliases = []string{"bad_alias"}
		}, `alias "bad_alias" must be a full dotted path`},
		{"alias path collision", func(a *ir.App) {
			a.Groups[1].Keys[0].Aliases = []string{"database.host"}
		}, "file key database.host of smtp.url collides with another key or group"},
		{"alias env collision", func(a *ir.App) {
			a.Groups[1].Keys[0].Aliases = []string{"database.host"}
		}, "env var DEMO_DATABASE_HOST of smtp.url collides with another key or group"},
		{"alias flag collision", func(a *ir.App) {
			a.Groups[1].Keys[0].Aliases = []string{"database.host"}
		}, "flag --database-host of smtp.url collides with another key or group"},
		{"alias on an argument", func(a *ir.App) {
			a.Commands[0].Args = []ir.Key{{Name: "verbose", Desc: "x", Kind: ir.KindBool, Aliases: []string{"server.verbose"}}}
		}, "aliases aren't supported on arguments"},
		{"blank Deprecated", func(a *ir.App) {
			a.Groups[0].Keys[0].Deprecated = "   "
		}, "deprecated message of database.host must not be blank"},
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

func TestGlobalInUsesWarns(t *testing.T) {
	m, err := Build(sample(), "settings") // Serve lists the global log group
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Warnings) != 1 || !strings.Contains(m.Warnings[0].Msg, `command Serve uses group "log", which is Global`) {
		t.Errorf("warnings: %v", m.Warnings)
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
		"kinds":          func(a *ir.App) { *a = *allKinds() },
		"mutable":        func(a *ir.App) { *a = *mutableKinds() },
		"store commands": func(a *ir.App) { *a = *storeCommands() },
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
			if name == "store commands" {
				// Run has the store and its methods; Plain has neither.
				assert := `package settings

import (
	"context"

	"github.com/epistemer/cfgx/store"
)

var (
	_ func() *store.Store                                                         = (*Run)(nil).Store
	_ func(context.Context, func(*Update) error, ...cfgx.WriteOption) error       = (*Run)(nil).Update
	_ func(func(Change))                                                          = (*Run)(nil).OnChange
	_ func() *Run                                                                 = (*Run)(nil).Snapshot
	_ func() error                                                                = (*Run)(nil).Close
	_ func() *Admin                                                               = (*Admin)(nil).Snapshot
)

`
				assert = strings.Replace(assert, `"context"`, `"context"`+"\n\n\t\"github.com/epistemer/cfgx\"", 1)
				if err := os.WriteFile(filepath.Join(dir, "assert.go"), []byte(assert), 0o644); err != nil {
					t.Fatal(err)
				}
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

func TestFuncRefRules(t *testing.T) {
	for _, c := range []struct {
		from, to string
		ok       bool
	}{
		{"m/a", "m/internal/x", true},
		{"m/internal/settings", "m/internal/x", true},
		{"m/internal/settings", "m/internal/settings/schema/internal/x", false},
		{"other/a", "m/internal/x", false},
		{"m/a", "internal/abi", false},
		{"m/a", "m/x", true},
	} {
		if got := internalOK(c.from, c.to); got != c.ok {
			t.Errorf("internalOK(%s, %s) = %v", c.from, c.to, got)
		}
	}
	for p, want := range map[string]string{"gopkg.in/yaml.v3": "yaml", "x/go-cmp": "go_cmp", "math/rand/v2": "v2", "x/2fa": "pkg", "runtime": "runtime"} {
		if got := guessAlias(p); got != want {
			t.Errorf("guessAlias(%s) = %s, want %s", p, got, want)
		}
	}
	a := allKinds()
	a.Groups[0].Keys = []ir.Key{{Name: "x", Desc: "x", Kind: ir.KindInt, Validate: &ir.FuncRef{Path: "main", Name: "Check"}}}
	if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "k.x: Validate: main.Check is in package main") {
		t.Errorf("a function in main: %v", err)
	}
	a.Groups[0].Keys = []ir.Key{{Name: "x", Desc: "x", Kind: ir.KindString, Checks: []ir.Check{{Kind: ir.CheckMin, Args: []string{"1"}}}}}
	if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "Min, Max and Range need a number or duration key") {
		t.Errorf("Min on a string: %v", err)
	}
}

// Rules name keys by the path the schema declared, which Key changes only
// in the config file.
func TestRuleOnRenamedKey(t *testing.T) {
	a := allKinds()
	a.Groups[0].Keys = []ir.Key{
		{Name: "a", Desc: "a", Kind: ir.KindString, FileKey: "alpha"},
		{Name: "b", Desc: "b", Kind: ir.KindString},
	}
	a.Rules = []ir.Rule{{Kind: ir.RuleAtMostOne, Group: "k", Keys: []string{"k.a", "k.b"}}}
	m, err := Build(a, "settings")
	if err != nil {
		t.Fatal(err)
	}
	if m.Keys[0].Path != "k.alpha" || m.Keys[0].Env != "K_K_A" || m.Keys[0].Flag != "k-a" || m.Keys[0].Getter != "KA" {
		t.Errorf("renamed key: %+v", m.Keys[0])
	}
	if len(m.Rules) != 1 || fmt.Sprint(m.Rules[0].Keys) != "[0 1]" {
		t.Errorf("rules: %+v", m.Rules)
	}
}

// TestArtifactsGolden checks every generated file (M6.1) for two schemas,
// and that the example config and the schema parse.
func TestArtifactsGolden(t *testing.T) {
	for name, app := range map[string]*ir.App{"nested": nested(), "kinds": allKinds()} {
		t.Run(name, func(t *testing.T) {
			app.ConfigFile, app.SearchPaths = "config", []string{".", "$HOME/.zp"}
			m, err := Build(app, "settings")
			if err != nil {
				t.Fatal(err)
			}
			files, err := Artifacts(m)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range []string{"config.example.yaml", "config.schema.json", ".env.example", "docs/CONFIG.md", "cfgx.lock.json"} {
				data, ok := files[f]
				if !ok {
					t.Errorf("no %s", f)
					continue
				}
				golden.Assert(t, filepath.Join("testdata", "artifacts", name, strings.ReplaceAll(f, "/", "_")+".golden"), data)
			}
			if _, err := node.Parse("config.example.yaml", files["config.example.yaml"], node.YAML); err != nil {
				t.Errorf("the example config doesn't parse: %v", err)
			}
			var schema map[string]any
			if err := json.Unmarshal(files["config.schema.json"], &schema); err != nil {
				t.Errorf("the schema isn't JSON: %v", err)
			}
			if m.ExampleConfig != string(files["config.example.yaml"]) || m.JSONSchema != string(files["config.schema.json"]) {
				t.Error("the embedded files differ from the written ones")
			}
		})
	}
}

// Generated files stay well-formed whatever the schema says: multi-line
// descriptions and defaults, secrets.
func TestArtifactsEdgeCases(t *testing.T) {
	a := allKinds()
	a.ConfigFile = "config"
	a.Groups[0].Desc = "Every kind\nsecond line"
	a.Groups[0].Keys = []ir.Key{
		{Name: "multi", Desc: "One line.\nAnother line.", Kind: ir.KindString, Default: str("a\nb")},
		{Name: "pw", Desc: "Password", Kind: ir.KindSecret, Default: str("hunter2")},
		{Name: "waits", Desc: "Waits", Kind: ir.KindDurations, Default: str("0,5s")},
		{Name: "size", Desc: "Size", Kind: ir.KindByteSize, Default: str("1024")},
	}
	m, err := Build(a, "settings")
	if err != nil {
		t.Fatal(err)
	}
	files, err := Artifacts(m)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if strings.Contains(string(data), "hunter2") {
			t.Errorf("%s shows the secret", name)
		}
	}
	if _, err := node.Parse("config.example.yaml", files["config.example.yaml"], node.YAML); err != nil {
		t.Errorf("example config: %v\n%s", err, files["config.example.yaml"])
	}
	for _, l := range strings.Split(string(files[".env.example"]), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			t.Errorf(".env.example has an uncommented line %q", l)
		}
	}
	if !strings.Contains(string(files[".env.example"]), `# K_K_MULTI="a\nb"`) {
		t.Errorf(".env.example:\n%s", files[".env.example"])
	}
}

// A store's generated names are claimed: a command can't take them.
func TestMutableNamesClaimed(t *testing.T) {
	for _, name := range []string{"Update", "Change", "Key"} {
		a := mutableKinds()
		a.Commands = append(a.Commands, ir.Command{Name: name, Uses: []string{"k"}})
		if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "command type "+name+" collides") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// i18nApp is nested with App.I18n and a string enum whose description
// needs quoting.
func i18nApp() *ir.App {
	a := nested()
	a.I18n = true
	g := &a.Groups[0]
	g.Keys = append(g.Keys, ir.Key{Name: "sslmode", Desc: `SSL mode, as in "sslmode=require"`, Kind: ir.KindEnum,
		Type: &ir.Type{Name: "string", Kind: "string"},
		Enum: []ir.EnumValue{{Text: "disable", Lit: `"disable"`}, {Text: "verify-full", Lit: `"verify-full"`}}})
	return a
}

func TestI18nGolden(t *testing.T) {
	m, err := Build(i18nApp(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	src, err := RenderI18n(m)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	golden.Assert(t, filepath.Join("testdata", "i18n.golden"), src)
	if strings.Contains(string(src), "to.address") || strings.Contains(string(src), "migrate") {
		t.Error("arguments are flags of one run and get no labels")
	}

	m, err = Build(nested(), "settings")
	if err != nil {
		t.Fatal(err)
	}
	if m.I18n || len(m.L10n) > 0 {
		t.Error("labels without App.I18n")
	}
}

func TestI18nNamesClaimed(t *testing.T) {
	a := i18nApp()
	a.Tags = append(a.Tags, ir.Tag{Name: "L10nDB", Desc: "x"})
	if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "L10nDB collides") {
		t.Errorf("tag L10nDB: %v", err)
	}
	a = i18nApp()
	a.Tags = append(a.Tags, ir.Tag{Name: "L10n", Desc: "x"})
	if _, err := Build(a, "settings"); err == nil || !strings.Contains(err.Error(), "L10n collides") {
		t.Errorf("tag L10n: %v", err)
	}
}
