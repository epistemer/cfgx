package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/epistemer/cfgx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type vals struct {
	Host  string
	Port  int
	Debug bool
	Name  string
}

func table() []Key[vals] {
	return []Key[vals]{
		{Path: "server.host", Env: "CFGXT_SERVER_HOST", Flag: "server-host", Type: "string", Desc: "Host", DefaultText: "localhost", Quote: true,
			Set: func(v *vals, r string) error { return ParseString(r, &v.Host) }, Default: func(v *vals) { v.Host = "localhost" },
			IsSet: func(v *vals) bool { return v.Host != "" }},
		{Path: "server.port", Env: "CFGXT_SERVER_PORT", Flag: "server-port", Short: "p", Type: "int", Desc: "Port", DefaultText: "7777",
			Set: func(v *vals, r string) error { return ParseInt(r, &v.Port) }, Default: func(v *vals) { v.Port = 7777 },
			IsSet: func(v *vals) bool { return v.Port != 0 }},
		{Path: "log.debug", Env: "CFGXT_LOG_DEBUG", Flag: "log-debug", Type: "bool", Desc: "Debug logging", DefaultText: "false", Bool: true,
			Set: func(v *vals, r string) error { return ParseBool(r, &v.Debug) }, Default: func(v *vals) { v.Debug = false },
			IsSet: func(v *vals) bool { return v.Debug }},
		{Path: "database.name", Env: "CFGXT_DATABASE_NAME", Flag: "database-name", Type: "string", Desc: "Database name", Quote: true, Required: true,
			Set:   func(v *vals, r string) error { return ParseString(r, &v.Name) },
			IsSet: func(v *vals) bool { return v.Name != "" }},
	}
}

// server is the keys of a command that doesn't load database.name.
var server = []int{0, 1, 2}

// testApp returns an app that searches $CFG for config.*.
func testApp() *App[vals] {
	return &App[vals]{EnvPrefix: "CFGXT", ConfigFile: "config", SearchPaths: []string{"$CFG"}, Keys: table()}
}

type env map[string]string

func (e env) lookup(k string) (string, bool) { v, ok := e[k]; return v, ok }

func (e env) environ() []string {
	var out []string
	for k, v := range e {
		out = append(out, k+"="+v)
	}
	return out
}

// write writes files into a new directory and returns it.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func load(t *testing.T, a *App[vals], idx []int, e env, args ...string) (*Result[vals], error) {
	t.Helper()
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	if a.ConfigFile != "" {
		fs.Var(&rawValue{typ: "path"}, "config", "")
	}
	BindFlags(fs, a.Keys, idx)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return Load(a, fs, idx, Options{LookupEnv: e.lookup, Environ: e.environ})
}

func mustLoad(t *testing.T, a *App[vals], idx []int, e env, args ...string) *Result[vals] {
	t.Helper()
	res, err := load(t, a, idx, e, args...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestPrecedenceMatrix sets server.port in every combination of file, env
// and flag: the highest layer present wins, and every layer present is in
// the provenance, lowest first.
func TestPrecedenceMatrix(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": "server:\n  port: 1001\n"})
	empty := t.TempDir()
	for mask := 0; mask < 8; mask++ {
		file, inEnv, flag := mask&1 != 0, mask&2 != 0, mask&4 != 0
		t.Run(fmt.Sprintf("file=%v,env=%v,flag=%v", file, inEnv, flag), func(t *testing.T) {
			e := env{"CFG": empty}
			want, origins := 7777, []string{"default"}
			if file {
				e["CFG"] = dir
				want = 1001
				origins = append(origins, filepath.Join(dir, "config.yaml")+":2:9")
			}
			if inEnv {
				e["CFGXT_SERVER_PORT"] = "1002"
				want = 1002
				origins = append(origins, "env CFGXT_SERVER_PORT")
			}
			var args []string
			if flag {
				args = []string{"--server-port", "1003"}
				want = 1003
				origins = append(origins, "flag --server-port")
			}
			res := mustLoad(t, testApp(), server, e, args...)
			if res.Values.Port != want {
				t.Errorf("port = %d, want %d", res.Values.Port, want)
			}
			var got []string
			for _, o := range res.Origins[1] {
				got = append(got, o.String())
			}
			if !reflect.DeepEqual(got, origins) {
				t.Errorf("origins = %q, want %q", got, origins)
			}
		})
	}
}

func TestPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  env
		args []string
		want vals
	}{
		{"defaults", nil, nil, vals{"localhost", 7777, false, ""}},
		{"short flag", nil, []string{"-p", "1234"}, vals{"localhost", 1234, false, ""}},
		{"bool flag alone", nil, []string{"--log-debug"}, vals{"localhost", 7777, true, ""}},
		{"bool flag false beats env true", env{"CFGXT_LOG_DEBUG": "true"}, []string{"--log-debug=false"}, vals{"localhost", 7777, false, ""}},
		{"empty env clears", env{"CFGXT_SERVER_HOST": "", "CFGXT_SERVER_PORT": ""}, nil, vals{"", 0, false, ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := testApp()
			a.ConfigFile = ""
			res := mustLoad(t, a, server, c.env, c.args...)
			if *res.Values != c.want {
				t.Errorf("got %+v, want %+v", *res.Values, c.want)
			}
		})
	}
}

func TestFileValues(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": `server:
  host: "db.internal"
  port: ~
log:
  debug: true
`})
	res := mustLoad(t, testApp(), server, env{"CFG": dir})
	if got := *res.Values; got != (vals{"db.internal", 7777, true, ""}) {
		t.Errorf("got %+v (a null value counts as absent)", got)
	}
	if res.File != filepath.Join(dir, "config.yaml") {
		t.Errorf("File = %q", res.File)
	}
}

func TestEmptyEnvClearsFileValue(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": "server:\n  host: h\n"})
	res := mustLoad(t, testApp(), server, env{"CFG": dir, "CFGXT_SERVER_HOST": ""})
	if res.Values.Host != "" {
		t.Errorf("host = %q", res.Values.Host)
	}
}

func TestJSONFile(t *testing.T) {
	dir := write(t, map[string]string{"config.json": `{"server": {"host": "\uD83D\uDE00", "port": 9}}`})
	res := mustLoad(t, testApp(), server, env{"CFG": dir})
	if res.Values.Host != "😀" || res.Values.Port != 9 {
		t.Errorf("got %+v", *res.Values)
	}
}

func problems(t *testing.T, err error) string {
	t.Helper()
	var errs cfgx.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("want cfgx.Errors, got %v", err)
	}
	return err.Error()
}

func TestAllProblemsAtOnce(t *testing.T) {
	a := testApp()
	a.ConfigFile = ""
	_, err := load(t, a, server, env{"CFGXT_SERVER_HOST": "ok", "CFGXT_LOG_DEBUG": "maybe"}, "--server-port", "99999999999999999999")
	want := `invalid configuration (2 problems)
  log.debug: "maybe" is not a boolean; use true or false (env CFGXT_LOG_DEBUG)
  server.port: "99999999999999999999" is out of range for an integer (flag --server-port)`
	if got := problems(t, err); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFileProblems(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": `server:
  host: [a, b]
  port: {x: 1}
log: true
database:
  name: x
`})
	_, err := load(t, testApp(), server, env{"CFG": dir})
	p := filepath.Join(dir, "config.yaml")
	want := fmt.Sprintf(`invalid configuration (3 problems)
  log: expected a map of settings, found a single value (%[1]s:4:6)
  server.host: expected a single value, found a list (%[1]s:2:9)
  server.port: expected a single value, found a map (%[1]s:3:9)`, p)
	if got := problems(t, err); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// The same file with every bad value overridden loads.
	res := mustLoad(t, testApp(), server, env{"CFG": dir, "CFGXT_SERVER_HOST": "h", "CFGXT_SERVER_PORT": "1"}, "--log-debug")
	if *res.Values != (vals{"h", 1, true, ""}) {
		t.Errorf("got %+v", *res.Values)
	}
}

func TestBadFileValueReportsPosition(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": "server:\n  port: 8o80\n"})
	_, err := load(t, testApp(), server, env{"CFG": dir})
	want := fmt.Sprintf(`server.port: "8o80" is not an integer (%s:2:9)`, filepath.Join(dir, "config.yaml"))
	if got := problems(t, err); !strings.Contains(got, want) {
		t.Errorf("got:\n%s\nwant it to contain:\n%s", got, want)
	}
}

func warnings(res *Result[vals]) []string {
	var out []string
	for _, w := range res.Warnings {
		out = append(out, w.Msg)
	}
	return out
}

func TestUnknownFileKeys(t *testing.T) {
	dir := write(t, map[string]string{"config.yaml": `databse:
  name: x
server:
  hots: h
  port: 1
  extra: {a: 1}
database:
  name: y
zzz: 1
`})
	res := mustLoad(t, testApp(), server, env{"CFG": dir})
	p := filepath.Join(dir, "config.yaml")
	want := []string{
		fmt.Sprintf(`unknown key "databse" in %s:1:1; did you mean "database"?`, p),
		fmt.Sprintf(`unknown key "server.hots" in %s:4:3; did you mean "server.host"?`, p),
		fmt.Sprintf(`unknown key "server.extra" in %s:6:3`, p),
		fmt.Sprintf(`unknown key "zzz" in %s:9:1`, p),
	}
	// database.name isn't loaded by this command, but it is a key of the
	// app, so it isn't unknown.
	if got := warnings(res); !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestUnknownEnv(t *testing.T) {
	e := env{"CFGXT_SERVER_HOTS": "x", "CFGXT_CONFIG": "", "CFGXT_SERVER_HOST": "h", "CFGXT_ZZZ": "1", "OTHER_X": "1", "TX": "1"}
	res := mustLoad(t, testApp(), server, e)
	want := []string{
		"unknown environment variable CFGXT_SERVER_HOTS; did you mean CFGXT_SERVER_HOST?",
		"unknown environment variable CFGXT_ZZZ",
	}
	if got := warnings(res); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnknownEnvCaseInsensitive(t *testing.T) {
	old := envCaseInsensitive
	envCaseInsensitive = true
	defer func() { envCaseInsensitive = old }()
	res := mustLoad(t, testApp(), server, env{"cfgxt_server_host": "h", "cfgxt_Server_Hots": "x"})
	want := []string{"unknown environment variable cfgxt_Server_Hots; did you mean CFGXT_SERVER_HOST?"}
	if got := warnings(res); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRequired(t *testing.T) {
	all := []int{0, 1, 2, 3}
	_, err := load(t, testApp(), all, nil)
	want := `invalid configuration (1 problem)
  database.name: is required; set --database-name, CFGXT_DATABASE_NAME or database.name in the config file`
	if got := problems(t, err); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	_, err = load(t, testApp(), all, env{"CFGXT_DATABASE_NAME": ""})
	if got := problems(t, err); !strings.Contains(got, "database.name: is required, but is empty (env CFGXT_DATABASE_NAME)") {
		t.Errorf("got:\n%s", got)
	}

	// A non-zero default doesn't make Required pointless: an empty env var
	// clears it (A4).
	a := testApp()
	a.Keys[0].Required = true
	_, err = load(t, a, server, env{"CFGXT_SERVER_HOST": ""})
	if got := problems(t, err); !strings.Contains(got, "server.host: is required, but is empty (env CFGXT_SERVER_HOST)") {
		t.Errorf("got:\n%s", got)
	}

	// The message never quotes the value.
	a = testApp()
	a.Keys[1].Required = true
	_, err = load(t, a, server, env{"CFGXT_SERVER_PORT": "000"})
	if got := problems(t, err); !strings.Contains(got, "server.port: is required, but is set to the zero value (env CFGXT_SERVER_PORT)") {
		t.Errorf("got:\n%s", got)
	}

	if res := mustLoad(t, testApp(), all, env{"CFGXT_DATABASE_NAME": "db"}); res.Values.Name != "db" {
		t.Errorf("name = %q", res.Values.Name)
	}
	// A command that doesn't load the key doesn't check it.
	mustLoad(t, testApp(), server, nil)
}

func TestDiscovery(t *testing.T) {
	yaml := map[string]string{"config.yaml": "server:\n  port: 1\n"}
	cases := []struct {
		name    string
		app     func(*App[vals])
		files   map[string]string
		env     func(dir string) env
		args    func(dir string) []string
		port    int    // when it loads
		wantErr string // when it doesn't; %s is the directory
	}{
		{name: "search finds yaml", files: yaml, port: 1},
		{name: "search finds yml", files: map[string]string{"config.yml": "server: {port: 2}"}, port: 2},
		{name: "search finds json", files: map[string]string{"config.json": `{"server": {"port": 3}}`}, port: 3},
		{name: "nothing found is fine", files: map[string]string{"other.yaml": "x: 1"}, port: 7777},
		{name: "two matches", files: map[string]string{"config.yaml": "", "config.json": "{}"},
			wantErr: "found more than one config file: %[1]s/config.yaml and %[1]s/config.json; keep one"},
		{name: "a directory named like the file is skipped", files: map[string]string{"config.yaml/x": ""}, port: 7777},
		{name: "unset variable skips the path", files: yaml,
			app: func(a *App[vals]) { a.SearchPaths = []string{"$NOPE/x", "${CFG}"} }, port: 1},
		{name: "empty variable skips the path", files: yaml,
			env: func(dir string) env { return env{"CFG": "", "CFG2": dir} },
			app: func(a *App[vals]) { a.SearchPaths = []string{"$CFG", "$CFG2"} }, port: 1},
		{name: "first search path wins", files: map[string]string{"a/config.yaml": "server: {port: 4}", "b/config.yaml": "server: {port: 5}"},
			env: func(dir string) env { return env{"A": filepath.Join(dir, "a"), "B": filepath.Join(dir, "b")} },
			app: func(a *App[vals]) { a.SearchPaths = []string{"$B", "$A"} }, port: 5},
		{name: "formats restrict the search", files: map[string]string{"config.yaml": "server: {port: 6}", "config.json": `{"server": {"port": 7}}`},
			app: func(a *App[vals]) { a.Formats = []string{"json"} }, port: 7},
		{name: "--config", files: map[string]string{"x.yaml": "server: {port: 8}"},
			args: func(dir string) []string { return []string{"--config", filepath.Join(dir, "x.yaml")} }, port: 8},
		{name: "env names the file", files: map[string]string{"x.json": `{"server": {"port": 9}}`},
			env: func(dir string) env { return env{"CFGXT_CONFIG": filepath.Join(dir, "x.json")} }, port: 9},
		{name: "--config beats the env var", files: map[string]string{"x.yaml": "server: {port: 10}", "y.yaml": "server: {port: 11}"},
			env:  func(dir string) env { return env{"CFGXT_CONFIG": filepath.Join(dir, "y.yaml")} },
			args: func(dir string) []string { return []string{"--config", filepath.Join(dir, "x.yaml")} }, port: 10},
		{name: "explicit file beats the search", files: map[string]string{"config.yaml": "server: {port: 1}", "x.yaml": "server: {port: 12}"},
			env: func(dir string) env { return env{"CFG": dir, "CFGXT_CONFIG": filepath.Join(dir, "x.yaml")} }, port: 12},
		{name: "empty env var is no file", files: yaml, env: func(dir string) env { return env{"CFG": dir, "CFGXT_CONFIG": ""} }, port: 1},
		{name: "an empty --config searches, ignoring the env var", files: yaml,
			env:  func(dir string) env { return env{"CFG": dir, "CFGXT_CONFIG": filepath.Join(dir, "no.yaml")} },
			args: func(string) []string { return []string{"--config="} }, port: 1},
		{name: "extensions ignore case", files: map[string]string{"X.JSON": `{"server": {"port": 13}}`},
			args: func(dir string) []string { return []string{"--config", filepath.Join(dir, "X.JSON")} }, port: 13},
		{name: "--config must exist", args: func(dir string) []string { return []string{"--config", filepath.Join(dir, "no.yaml")} },
			wantErr: "the config file %s/no.yaml (from --config) doesn't exist"},
		{name: "env file must exist", env: func(dir string) env { return env{"CFGXT_CONFIG": filepath.Join(dir, "no.yaml")} },
			wantErr: "the config file %s/no.yaml (from CFGXT_CONFIG) doesn't exist"},
		{name: "explicit unknown extension", files: map[string]string{"x.conf": ""},
			args:    func(dir string) []string { return []string{"--config", filepath.Join(dir, "x.conf")} },
			wantErr: "the config file %s/x.conf (from --config) has an unknown format; use .yaml, .yml or .json"},
		{name: "explicit format not accepted", files: map[string]string{"x.yaml": ""},
			app:     func(a *App[vals]) { a.Formats = []string{"json"} },
			args:    func(dir string) []string { return []string{"--config", filepath.Join(dir, "x.yaml")} },
			wantErr: "the config file %s/x.yaml (from --config) is YAML, which this app doesn't accept; use .json"},
		{name: "explicit directory", files: map[string]string{"d.yaml/x": ""},
			args:    func(dir string) []string { return []string{"--config", filepath.Join(dir, "d.yaml")} },
			wantErr: "the config file %s/d.yaml (from --config) isn't a regular file"},
		{name: "invalid file", files: map[string]string{"config.yaml": "server:\n  port: 1\n  port: 2\n"},
			wantErr: `the config file is invalid: %s/config.yaml:3:3: duplicate key "port" (first at 2:3)`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := write(t, c.files)
			e := env{"CFG": dir}
			if c.env != nil {
				e = c.env(dir)
			}
			var args []string
			if c.args != nil {
				args = c.args(dir)
			}
			a := testApp()
			if c.app != nil {
				c.app(a)
			}
			res, err := load(t, a, server, e, args...)
			if c.wantErr != "" {
				want := fmt.Sprintf(c.wantErr, filepath.ToSlash(dir))
				if err == nil || filepath.ToSlash(err.Error()) != want {
					t.Fatalf("got %v, want %s", err, want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Values.Port != c.port {
				t.Errorf("port = %d, want %d", res.Values.Port, c.port)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	got, ok := expand("$HOME/.zp", func(string) (string, bool) { return "", false })
	if !ok || got != home+"/.zp" {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestKeyWithoutFlag(t *testing.T) {
	a := &App[vals]{EnvPrefix: "CFGXT", Keys: []Key[vals]{{Path: "x.host", Env: "CFGXT_X", Type: "string", Desc: "no flag",
		Set: func(v *vals, r string) error { return ParseString(r, &v.Host) }, IsSet: func(*vals) bool { return true }}}}
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, a.Keys, []int{0})
	if fs.HasFlags() {
		t.Error("a key without a flag name must not register a flag")
	}
	res, err := Load(a, fs, []int{0}, Options{LookupEnv: env{"CFGXT_X": "h"}.lookup, Environ: env{}.environ})
	if err != nil || res.Values.Host != "h" {
		t.Errorf("got %+v, %v", res, err)
	}
}

func TestHelpShowsDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, table(), server)
	help := fs.FlagUsages()
	for _, want := range []string{"--server-host string", `Host (default "localhost")`, "-p, --server-port int", "Port (default 7777)", "--log-debug"} {
		if !strings.Contains(help, want) {
			t.Errorf("help is missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "default false") {
		t.Errorf("a false bool default shouldn't show:\n%s", help)
	}
}

type cfg struct {
	v    *vals
	args []string
}

// tree builds a root with BindRoot (unless bind is false) and one command.
func tree(a *App[vals], bind bool, idx []int, opts []cfgx.Option, run func(context.Context, *cfg) error) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "app"}
	if bind {
		BindRoot(root, a, opts...)
	}
	root.AddCommand(Command(&cobra.Command{Use: "serve", Args: cobra.MaximumNArgs(2)}, a, idx,
		func(v *vals, args []string) *cfg { return &cfg{v, args} }, run))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	return root, &out
}

func TestCommand(t *testing.T) {
	t.Setenv("CFG", t.TempDir())
	type key struct{}
	var got *cfg
	var gotCtx context.Context
	root, _ := tree(testApp(), true, server, nil, func(ctx context.Context, c *cfg) error { got, gotCtx = c, ctx; return nil })
	root.SetArgs([]string{"serve", "--server-port", "1", "a", "b"})
	ctx := context.WithValue(context.Background(), key{}, "v")
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.v.Port != 1 || strings.Join(got.args, ",") != "a,b" {
		t.Fatalf("run got %+v", got)
	}
	if gotCtx.Value(key{}) != "v" {
		t.Error("run didn't get cmd.Context()")
	}
}

// A configuration error prints "<root>: <problems>" and then the warnings,
// with no usage text and no cobra "Error:" line, and Execute fails.
func TestCommandConfigError(t *testing.T) {
	t.Setenv("CFG", t.TempDir())
	t.Setenv("CFGXT_SERVER_HOTS", "x")
	root, out := tree(testApp(), true, server, nil, func(context.Context, *cfg) error { t.Error("run was called"); return nil })
	root.SetArgs([]string{"serve", "--server-port", "x"})
	if err := root.Execute(); err == nil {
		t.Fatal("want a configuration error")
	}
	want := `app: invalid configuration (1 problem)
  server.port: "x" is not an integer (flag --server-port)
warning: unknown environment variable CFGXT_SERVER_HOTS; did you mean CFGXT_SERVER_HOST?
`
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestCommandLogger(t *testing.T) {
	t.Setenv("CFG", t.TempDir())
	t.Setenv("CFGXT_SERVER_HOTS", "x")
	var logged bytes.Buffer
	l := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}))
	root, out := tree(testApp(), true, server, []cfgx.Option{cfgx.WithLogger(l)}, func(context.Context, *cfg) error { return nil })
	root.SetArgs([]string{"serve"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("with a logger, nothing goes to stderr; got %q", out)
	}
	want := `level=WARN msg="unknown environment variable CFGXT_SERVER_HOTS; did you mean CFGXT_SERVER_HOST?" env=CFGXT_SERVER_HOTS` + "\n"
	if logged.String() != want {
		t.Errorf("logged %q, want %q", logged.String(), want)
	}
}

func TestGlobalFlags(t *testing.T) {
	t.Setenv("CFG", t.TempDir())
	a := testApp()
	a.Keys[2].Global = true
	a.Global = []int{2}
	var got *cfg
	root, _ := tree(a, true, server, nil, func(_ context.Context, c *cfg) error { got = c; return nil })
	if root.PersistentFlags().Lookup("log-debug") == nil || root.PersistentFlags().Lookup("config") == nil {
		t.Fatal("BindRoot must add --config and global flags as persistent flags")
	}
	serve, _, _ := root.Find([]string{"serve"})
	if serve.LocalNonPersistentFlags().Lookup("log-debug") != nil {
		t.Error("a global flag must not also be a local flag")
	}
	root.SetArgs([]string{"serve", "--log-debug"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !got.v.Debug {
		t.Error("the persistent flag didn't reach the command")
	}
}

func TestBindRootMissing(t *testing.T) {
	root, _ := tree(testApp(), false, server, nil, func(context.Context, *cfg) error { return nil })
	root.SetArgs([]string{"serve"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "BindRoot wasn't called") {
		t.Errorf("got %v", err)
	}

	// An app with no config file and no global groups doesn't need it.
	a := testApp()
	a.ConfigFile = ""
	root, _ = tree(a, false, server, nil, func(context.Context, *cfg) error { return nil })
	root.SetArgs([]string{"serve"})
	if err := root.Execute(); err != nil {
		t.Error(err)
	}
}

func TestConfigUsage(t *testing.T) {
	a := testApp()
	a.SearchPaths = []string{".", "$HOME/.t", "/etc/t"}
	want := "config file; overrides CFGXT_CONFIG (default: config.yaml, .yml or .json in ., $HOME/.t or /etc/t)"
	if got := a.configUsage(); got != want {
		t.Errorf("got %q", got)
	}
}

func TestErrorsWithoutSource(t *testing.T) {
	err := cfgx.Errors{{Key: "a.b", Message: "is required"}}
	if got := err.Error(); got != "invalid configuration (1 problem)\n  a.b: is required" {
		t.Errorf("got %q", got)
	}
}

func TestFlagOnlyKey(t *testing.T) {
	type extraVals struct {
		vals
		Steps int
	}
	keys := []Key[extraVals]{
		{
			Path: "server.host", Env: "CFGXT_SERVER_HOST", Flag: "server-host", Type: "string", Desc: "Host",
			Set: func(v *extraVals, r string) error { return ParseString(r, &v.Host) },
		},
		{
			Path: "steps", Flag: "steps", Type: "int", Desc: "Steps", FlagOnly: true,
			Set: func(v *extraVals, r string) error { return ParseInt(r, &v.Steps) },
		},
	}
	dir := write(t, map[string]string{
		"config.yaml": "steps: 99\n",
	})
	a := &App[extraVals]{
		EnvPrefix:   "CFGXT",
		ConfigFile:  "config",
		SearchPaths: []string{dir},
		Keys:        keys,
	}
	e := env{
		"CFGXT_STEPS": "88",
	}

	// Without flag: steps remains 0 (not read from file or env)
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, a.Keys, []int{0, 1})
	res, err := Load(a, fs, []int{0, 1}, Options{LookupEnv: e.lookup, Environ: e.environ})
	if err != nil {
		t.Fatal(err)
	}
	if res.Values.Steps != 0 {
		t.Errorf("steps = %d, want 0 (should not read from file or env)", res.Values.Steps)
	}
	// Check warnings: both file and env should warn as unknown
	var warnKeys []string
	for _, w := range res.Warnings {
		warnKeys = append(warnKeys, w.Msg)
	}
	warnStr := strings.Join(warnKeys, "\n")
	if !strings.Contains(warnStr, `unknown key "steps"`) {
		t.Errorf("expected unknown file key warning for steps, got %v", warnKeys)
	}
	if !strings.Contains(warnStr, "unknown environment variable CFGXT_STEPS") {
		t.Errorf("expected unknown env warning for CFGXT_STEPS, got %v", warnKeys)
	}

	// With flag: sets the value
	fs = pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, a.Keys, []int{0, 1})
	if err := fs.Parse([]string{"--steps", "3"}); err != nil {
		t.Fatal(err)
	}
	res, err = Load(a, fs, []int{0, 1}, Options{LookupEnv: e.lookup, Environ: e.environ})
	if err != nil {
		t.Fatal(err)
	}
	if res.Values.Steps != 3 {
		t.Errorf("steps = %d, want 3", res.Values.Steps)
	}
}
