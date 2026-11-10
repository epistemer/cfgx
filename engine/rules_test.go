package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type rvals struct {
	Token, Keys, Cert string
	TLS, A, B, C      bool
}

func strKey(path, env string, f func(*rvals) *string) Key[rvals] {
	return Key[rvals]{Path: path, Env: env, Flag: strings.ReplaceAll(path, ".", "-"), Type: "string", Desc: path,
		Set: func(v *rvals, r string) error { return ParseString(r, f(v)) }, IsSet: func(v *rvals) bool { return *f(v) != "" }}
}

func boolKey(path, env string, f func(*rvals) *bool) Key[rvals] {
	return Key[rvals]{Path: path, Env: env, Flag: strings.ReplaceAll(path, ".", "-"), Type: "bool", Desc: path, Bool: true,
		Set: func(v *rvals, r string) error { return ParseBool(r, f(v)) }, IsSet: func(v *rvals) bool { return *f(v) }}
}

// rulesApp: auth.token and auth.keys are exclusive; server.tls.cert only
// matters when server.tls.enabled is set (default "cert.pem"); x.a, x.b and
// x.c exercise the other rules.
func rulesApp(rules ...Rule) *App[rvals] {
	cert := strKey("server.tls.cert", "R_SERVER_TLS_CERT", func(v *rvals) *string { return &v.Cert })
	cert.RelevantIf = []int{3}
	cert.Default = func(v *rvals) { v.Cert = "cert.pem" }
	cert.Required = true
	return &App[rvals]{EnvPrefix: "R", ConfigFile: "config", SearchPaths: []string{"$CFG"}, Rules: rules, Keys: []Key[rvals]{
		strKey("auth.token", "R_AUTH_TOKEN", func(v *rvals) *string { return &v.Token }),
		strKey("auth.keys", "R_AUTH_KEYS", func(v *rvals) *string { return &v.Keys }),
		cert,
		boolKey("server.tls.enabled", "R_SERVER_TLS_ENABLED", func(v *rvals) *bool { return &v.TLS }),
		boolKey("x.a", "R_X_A", func(v *rvals) *bool { return &v.A }),
		boolKey("x.b", "R_X_B", func(v *rvals) *bool { return &v.B }),
		boolKey("x.c", "R_X_C", func(v *rvals) *bool { return &v.C }),
	}}
}

func rload(t *testing.T, a *App[rvals], idx []int, e env, args ...string) (*Result[rvals], error) {
	t.Helper()
	fs := newFlagSet(a, idx)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if e == nil {
		e = env{}
	}
	if _, ok := e["CFG"]; !ok {
		e["CFG"] = t.TempDir()
	}
	return Load(a, fs, idx, Options{LookupEnv: e.lookup, Environ: e.environ})
}

func TestRules(t *testing.T) {
	exclusive := Rule{Kind: AtMostOne, Group: "auth", Keys: []int{0, 1}}
	dir := write(t, map[string]string{"config.yaml": "auth:\n  keys: k.csv\n"})
	p := filepath.Join(dir, "config.yaml")
	all := []int{0, 1, 2, 3, 4, 5, 6}
	cases := []struct {
		name string
		rule Rule
		env  env
		args []string
		want string // "" means it loads
	}{
		{"at most one: one set", exclusive, env{"R_AUTH_TOKEN": "t"}, nil, ""},
		{"at most one: both set", exclusive, env{"R_AUTH_TOKEN": "t", "CFG": dir}, nil,
			fmt.Sprintf("auth: token and keys are mutually exclusive (token: env R_AUTH_TOKEN, keys: %s:2:9)", p)},
		{"an empty env var clears the file value", exclusive, env{"R_AUTH_TOKEN": "t", "R_AUTH_KEYS": "", "CFG": dir}, nil, ""},
		{"at most one of three", Rule{Kind: AtMostOne, Group: "x", Keys: []int{4, 5, 6}}, nil, []string{"--x-a", "--x-c"},
			"x: at most one of a, b and c may be set (a: flag --x-a, c: flag --x-c)"},
		{"false doesn't count as set", Rule{Kind: AtMostOne, Group: "x", Keys: []int{4, 5}}, nil, []string{"--x-a", "--x-b=false"}, ""},
		{"exactly one: none", Rule{Kind: ExactlyOne, Group: "x", Keys: []int{4, 5}}, nil, nil, "x: one of a and b must be set"},
		{"exactly one: two", Rule{Kind: ExactlyOne, Group: "x", Keys: []int{4, 5}}, env{"R_X_A": "true", "R_X_B": "1"}, nil,
			"x: only one of a and b may be set (a: env R_X_A, b: env R_X_B)"},
		{"exactly one: one", Rule{Kind: ExactlyOne, Group: "x", Keys: []int{4, 5}}, nil, []string{"--x-b"}, ""},
		{"all or none: some", Rule{Kind: AllOrNone, Group: "x", Keys: []int{4, 5, 6}}, nil, []string{"--x-b"},
			"x: a, b and c must be set together or not at all; a and c aren't set (b: flag --x-b)"},
		{"all or none: none", Rule{Kind: AllOrNone, Group: "x", Keys: []int{4, 5, 6}}, nil, nil, ""},
		{"all or none: all", Rule{Kind: AllOrNone, Group: "x", Keys: []int{4, 5, 6}}, nil, []string{"--x-a", "--x-b", "--x-c"}, ""},
		{"requires: missing", Rule{Kind: Requires, Group: "x", Keys: []int{4, 5}}, nil, []string{"--x-a"}, "x: a requires b (a: flag --x-a)"},
		{"requires: met", Rule{Kind: Requires, Group: "x", Keys: []int{4, 5}}, nil, []string{"--x-a", "--x-b"}, ""},
		{"requires: only b", Rule{Kind: Requires, Group: "x", Keys: []int{4, 5}}, nil, []string{"--x-b"}, ""},
		{"keys outside the group keep their path", Rule{Kind: Requires, Group: "auth", Keys: []int{0, 3}}, env{"R_AUTH_TOKEN": "t"}, nil,
			"auth: token requires server.tls.enabled (token: env R_AUTH_TOKEN)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := rload(t, rulesApp(c.rule), all, c.env, c.args...)
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "\n  "+c.want) {
				t.Errorf("got:\n%v\nwant a line:\n  %s", err, c.want)
			}
		})
	}
}

// A rule whose keys the command doesn't all load isn't checked.
func TestRuleNeedsAllItsKeys(t *testing.T) {
	a := rulesApp(Rule{Kind: ExactlyOne, Group: "x", Keys: []int{4, 5}})
	if _, err := rload(t, a, []int{4}, nil); err != nil {
		t.Error(err)
	}
}

// A rule over a key that failed to parse isn't reported again.
func TestRuleSkipsFailedKeys(t *testing.T) {
	a := rulesApp(Rule{Kind: ExactlyOne, Group: "x", Keys: []int{4, 5}})
	_, err := rload(t, a, []int{4, 5}, env{"R_X_A": "maybe"})
	if err == nil || strings.Contains(err.Error(), "must be set") || !strings.Contains(err.Error(), "1 problem") {
		t.Errorf("got %v", err)
	}
}

func TestRelevantIf(t *testing.T) {
	idx := []int{2, 3} // the cert is listed before its condition
	// TLS off: the cert keeps its default and isn't checked, though it is
	// Required and an empty env var would clear it. An empty value doesn't
	// count as setting it (A4), so there is no warning.
	res, err := rload(t, rulesApp(), idx, env{"R_SERVER_TLS_CERT": ""})
	if err != nil {
		t.Fatal(err)
	}
	if res.Values.Cert != "cert.pem" || len(res.Warnings) != 0 {
		t.Errorf("cert = %q, warnings %v; want the default and none", res.Values.Cert, res.Warnings)
	}
	// Setting it to a value warns.
	res, err = rload(t, rulesApp(), idx, env{"R_SERVER_TLS_CERT": "x.pem"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"server.tls.cert is set (env R_SERVER_TLS_CERT) but ignored: it only matters when server.tls.enabled is set"}
	var got []string
	for _, w := range res.Warnings {
		got = append(got, w.Msg)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("warnings %q, want %q", got, want)
	}

	// --tls-enabled=false is not set either.
	if _, err := rload(t, rulesApp(), idx, env{"R_SERVER_TLS_CERT": ""}, "--server-tls-enabled=false"); err != nil {
		t.Error(err)
	}

	// TLS on: the cert is loaded and checked.
	_, err = rload(t, rulesApp(), idx, env{"R_SERVER_TLS_CERT": ""}, "--server-tls-enabled")
	if err == nil || !strings.Contains(err.Error(), "server.tls.cert: is required, but is empty (env R_SERVER_TLS_CERT)") {
		t.Errorf("got %v", err)
	}

	// A condition that fails to parse is reported once; no warning about the
	// cert.
	res, err = rload(t, rulesApp(), idx, env{"R_SERVER_TLS_ENABLED": "maybe", "R_SERVER_TLS_CERT": "x"})
	if err == nil || len(res.Warnings) != 0 {
		t.Errorf("err %v, warnings %v", err, res.Warnings)
	}
}

// A command that loads the cert but not its condition still loads the
// condition, without holding it to Required and without rules.
func TestConditionOnly(t *testing.T) {
	a := rulesApp(Rule{Kind: ExactlyOne, Group: "server", Keys: []int{3, 4}})
	a.Keys[3].Required = true
	if _, err := rload(t, a, []int{2}, nil); err != nil {
		t.Errorf("the condition isn't this command's key: %v", err)
	}
	_, err := rload(t, a, []int{2}, env{"R_SERVER_TLS_CERT": ""}, "--server-tls-enabled")
	if err == nil || !strings.Contains(err.Error(), "server.tls.cert: is required, but is empty") {
		t.Errorf("with the condition set, the cert is checked: %v", err)
	}
}

func TestCommandBindsConditionFlags(t *testing.T) {
	a := rulesApp()
	a.ConfigFile = ""
	cmd := Command(&cobra.Command{Use: "x"}, a, "X", []int{2}, func(s *Snapshot[rvals], _ *Writer[rvals], _ []string) *rvals { return s.Load() },
		func(context.Context, *rvals) error { return nil })
	if cmd.Flags().Lookup("server-tls-enabled") == nil {
		t.Error("the condition's flag must be bound, or the cert can never be relevant through flags")
	}
}

// An irrelevant key counts as unset in rules.
func TestIrrelevantKeyIsUnsetInRules(t *testing.T) {
	a := rulesApp(Rule{Kind: AtMostOne, Group: "auth", Keys: []int{0, 2}})
	if _, err := rload(t, a, []int{0, 2, 3}, env{"R_AUTH_TOKEN": "t"}); err != nil {
		t.Error(err)
	}
}

// newFlagSet binds --config and the flags of the keys at idx and their
// RelevantIf conditions, as Command does.
func newFlagSet[V any](a *App[V], idx []int) *pflag.FlagSet {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	if a.ConfigFile != "" {
		fs.Var(&rawValue{typ: "path"}, "config", "")
	}
	BindFlags(fs, a.Keys, append(append([]int(nil), idx...), conditions(a, idx)...))
	return fs
}
