package binding

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var (
	logLevel = Key{Flag: "log-level", Env: "ZP_LOG_LEVEL", Type: "string", Default: "info", Usage: "Minimum log level"}
	port     = Key{Flag: "port", Short: "p", Env: "ZP_SERVER_PORT", Type: "int", Default: "7777", Usage: "Listen port"}
	color    = Key{Flag: "color", Env: "ZP_LOG_COLOR", Type: "bool", Default: "false", Usage: "Force colored output"}
	tags     = Key{Flag: "tag", Env: "ZP_SERVER_TAG", Type: "stringSlice", Default: "", Usage: "Tags", Slice: true}
	cert     = Key{Flag: "cert", Env: "ZP_SERVER_TLS_CERT", Type: "string", Default: "ssl/cert.pem", Usage: "TLS certificate"}
)

type result struct {
	values  map[string][]string
	sources map[string]string
}

// app builds root (with a persistent, Global-style flag) and serve, the way
// BindRoot and ServeCommand will.
func app(t *testing.T) (*cobra.Command, *result) {
	t.Helper()
	res := &result{values: map[string][]string{}, sources: map[string]string{}}
	root := &cobra.Command{Use: "zp", SilenceUsage: true}
	Bind(root.PersistentFlags(), logLevel)
	root.RegisterFlagCompletionFunc(logLevel.Flag, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp
	})

	serve := &cobra.Command{
		Use: "serve",
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, k := range []Key{logLevel, port, color, tags, cert} {
				res.values[k.Flag], res.sources[k.Flag] = Resolve(cmd.Flags(), k)
			}
			return nil
		},
	}
	Bind(serve.Flags(), port)
	Bind(serve.Flags(), color)
	Bind(serve.Flags(), tags)
	Bind(serve.Flags(), cert)
	if err := serve.MarkFlagFilename(cert.Flag, "pem"); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(serve)
	return root, res
}

func run(t *testing.T, args ...string) (*result, string) {
	t.Helper()
	root, res := app(t)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return res, out.String()
}

func TestPrecedence(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		args    []string
		key     string
		want    string
		wantSrc string
	}{
		{"default", nil, []string{"serve"}, "port", "7777", "default"},
		{"env beats the flag's default", map[string]string{"ZP_SERVER_PORT": "9090"}, []string{"serve"}, "port", "9090", "env ZP_SERVER_PORT"},
		{"flag beats env", map[string]string{"ZP_SERVER_PORT": "9090"}, []string{"serve", "--port", "8080"}, "port", "8080", "flag --port"},
		{"short flag", nil, []string{"serve", "-p", "1234"}, "port", "1234", "flag --port"},
		{"empty env var is a value", map[string]string{"ZP_SERVER_TLS_CERT": ""}, []string{"serve"}, "cert", "", "env ZP_SERVER_TLS_CERT"},
		{"invalid int reaches cfgx as raw text", nil, []string{"serve", "--port", "abc"}, "port", "abc", "flag --port"},
		{"persistent flag before the subcommand", nil, []string{"--log-level", "debug", "serve"}, "log-level", "debug", "flag --log-level"},
		{"persistent flag after the subcommand", nil, []string{"serve", "--log-level", "warn"}, "log-level", "warn", "flag --log-level"},
		{"persistent flag: env when not passed", map[string]string{"ZP_LOG_LEVEL": "error"}, []string{"serve"}, "log-level", "error", "env ZP_LOG_LEVEL"},
		{"bool flag alone", nil, []string{"serve", "--color"}, "color", "true", "flag --color"},
		{"bool flag =false is still passed", map[string]string{"ZP_LOG_COLOR": "true"}, []string{"serve", "--color=false"}, "color", "false", "flag --color"},
		{"bool default", nil, []string{"serve"}, "color", "false", "default"},
		{"repeated slice flag accumulates", nil, []string{"serve", "--tag", "a", "--tag", "b,c"}, "tag", "a|b|c", "flag --tag"},
		{"slice from env", map[string]string{"ZP_SERVER_TAG": "x,y"}, []string{"serve"}, "tag", "x|y", "env ZP_SERVER_TAG"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			res, _ := run(t, c.args...)
			if got := strings.Join(res.values[c.key], "|"); got != c.want || res.sources[c.key] != c.wantSrc {
				t.Errorf("%s = %q from %q; want %q from %q", c.key, got, res.sources[c.key], c.want, c.wantSrc)
			}
		})
	}
}

func TestHelpShowsDefaultsWithoutApplyingThem(t *testing.T) {
	_, out := run(t, "serve", "--help")
	t.Logf("serve --help:\n%s", out)
	for _, want := range []string{
		"-p, --port int",
		"(default 7777)",
		`(default "info")`,
		`(default "ssl/cert.pem")`,
		"--color",
		"--tag",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q", want)
		}
	}
	if strings.Contains(out, "(default false)") {
		t.Errorf("a false bool default shouldn't be printed")
	}
}

func TestCompletion(t *testing.T) {
	_, out := run(t, cobra.ShellCompRequestCmd, "serve", "--log-level", "")
	t.Logf("enum completion:\n%s", out)
	for _, want := range []string{"debug", "info", "warn", "error", ":4"} {
		if !strings.Contains(out, want) {
			t.Errorf("enum completion is missing %q", want)
		}
	}
	_, out = run(t, cobra.ShellCompRequestCmd, "serve", "--cert", "")
	t.Logf("file completion:\n%s", out)
	if !strings.Contains(out, "pem") || !strings.Contains(out, ":8") {
		t.Errorf("file completion should filter on .pem (directive 8), got:\n%s", out)
	}
}
