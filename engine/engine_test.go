package engine

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type vals struct {
	Host  string
	Port  int
	Debug bool
}

var table = []Key[vals]{
	{Path: "server.host", Env: "T_SERVER_HOST", Flag: "server-host", Type: "string", Desc: "Host", DefaultText: "localhost", Quote: true,
		Set: func(v *vals, r string) error { return ParseString(r, &v.Host) }, Default: func(v *vals) { v.Host = "localhost" }},
	{Path: "server.port", Env: "T_SERVER_PORT", Flag: "server-port", Short: "p", Type: "int", Desc: "Port", DefaultText: "7777",
		Set: func(v *vals, r string) error { return ParseInt(r, &v.Port) }, Default: func(v *vals) { v.Port = 7777 }},
	{Path: "log.debug", Env: "T_LOG_DEBUG", Flag: "log-debug", Type: "bool", Desc: "Debug logging", DefaultText: "false", Bool: true,
		Set: func(v *vals, r string) error { return ParseBool(r, &v.Debug) }, Default: func(v *vals) { v.Debug = false }},
}

var all = []int{0, 1, 2}

func load(t *testing.T, env map[string]string, args ...string) (*vals, error) {
	t.Helper()
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, table, all)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return Load(fs, table, all, Options{LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok }})
}

func TestPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		args []string
		want vals
	}{
		{"defaults", nil, nil, vals{"localhost", 7777, false}},
		{"env beats default", map[string]string{"T_SERVER_PORT": "9090"}, nil, vals{"localhost", 9090, false}},
		{"flag beats env", map[string]string{"T_SERVER_PORT": "9090"}, []string{"--server-port", "8080"}, vals{"localhost", 8080, false}},
		{"short flag", nil, []string{"-p", "1234"}, vals{"localhost", 1234, false}},
		{"bool flag alone", nil, []string{"--log-debug"}, vals{"localhost", 7777, true}},
		{"bool flag false beats env true", map[string]string{"T_LOG_DEBUG": "true"}, []string{"--log-debug=false"}, vals{"localhost", 7777, false}},
		{"empty env clears", map[string]string{"T_SERVER_HOST": "", "T_SERVER_PORT": ""}, nil, vals{"", 0, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := load(t, c.env, c.args...)
			if err != nil {
				t.Fatal(err)
			}
			if *got != c.want {
				t.Errorf("got %+v, want %+v", *got, c.want)
			}
		})
	}
}

func TestAllProblemsAtOnce(t *testing.T) {
	_, err := load(t, map[string]string{"T_SERVER_PORT": "abc", "T_LOG_DEBUG": "maybe"}, "--server-port", "99999999999999999999")
	var errs Errors
	if !errors.As(err, &errs) {
		t.Fatalf("want Errors, got %v", err)
	}
	want := `invalid configuration (3 problems)
  log.debug: "maybe" is not a boolean; use true or false (env T_LOG_DEBUG)
  server.port: "abc" is not an integer (env T_SERVER_PORT)
  server.port: "99999999999999999999" is out of range for an integer (flag --server-port)`
	if err.Error() != want {
		t.Errorf("got:\n%s\nwant:\n%s", err, want)
	}
}

func TestHelpShowsDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, table, all)
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

func TestCommand(t *testing.T) {
	type cfg struct{ v *vals }
	var got *cfg
	cmd := Command(&cobra.Command{Use: "serve"}, table, all,
		func(v *vals) *cfg { return &cfg{v} },
		func(_ context.Context, c *cfg) error { got = c; return nil })
	cmd.SetArgs([]string{"--server-port", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.v.Port != 1 {
		t.Fatalf("run got %+v", got)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--server-port", "x"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("want a configuration error")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Errorf("a configuration error shouldn't print usage:\n%s", out.String())
	}
}
