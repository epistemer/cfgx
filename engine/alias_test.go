package engine

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

type aliasVals struct {
	SSLMode string
	Legacy  string
	Number  int
	Active  bool
	Opt     string
}

func aliasTable() []Key[aliasVals] {
	return []Key[aliasVals]{
		{
			Path: "database.sslmode", Env: "CFGXT_DATABASE_SSLMODE", Flag: "database-sslmode",
			Type: "string", Desc: "SSL mode", DefaultText: "prefer",
			Aliases: []Alias{{Path: "database.ssl_mode", Env: "CFGXT_DATABASE_SSL_MODE", Flag: "database-ssl-mode"}},
			Set:     func(v *aliasVals, r string) error { return ParseString(r, &v.SSLMode) },
			Default: func(v *aliasVals) { v.SSLMode = "prefer" },
			IsSet:   func(v *aliasVals) bool { return v.SSLMode != "" },
		},
		{
			Path: "server.legacy", Env: "CFGXT_SERVER_LEGACY", Flag: "server-legacy",
			Type: "string", Desc: "Legacy setting",
			Deprecated: "use modern setting instead",
			Set:        func(v *aliasVals, r string) error { return ParseString(r, &v.Legacy) },
			IsSet:      func(v *aliasVals) bool { return v.Legacy != "" },
		},
		{
			Path: "server.number", Env: "CFGXT_SERVER_NUMBER", Flag: "server-number",
			Type: "int", Desc: "Deprecated number",
			Deprecated: "count is obsolete",
			Set:        func(v *aliasVals, r string) error { return ParseInt(r, &v.Number) },
			IsSet:      func(v *aliasVals) bool { return v.Number != 0 },
		},
		{
			Path: "log.active", Env: "CFGXT_LOG_ACTIVE", Flag: "log-active",
			Type: "bool", Desc: "Active logging", Bool: true,
			Set:   func(v *aliasVals, r string) error { return ParseBool(r, &v.Active) },
			IsSet: func(v *aliasVals) bool { return v.Active },
		},
		{
			Path: "server.opt", Env: "CFGXT_SERVER_OPT", Flag: "server-opt",
			Type: "string", Desc: "Conditional setting",
			Deprecated: "opt is obsolete",
			RelevantIf: []int{3}, // only relevant if log.active is true
			Set:        func(v *aliasVals, r string) error { return ParseString(r, &v.Opt) },
			IsSet:      func(v *aliasVals) bool { return v.Opt != "" },
		},
	}
}

func aliasApp() *App[aliasVals] {
	return &App[aliasVals]{
		EnvPrefix:   "CFGXT",
		ConfigFile:  "config",
		SearchPaths: []string{"$CFG"},
		Keys:        aliasTable(),
	}
}

func loadAlias(t *testing.T, a *App[aliasVals], idx []int, e env, args ...string) (*Result[aliasVals], error) {
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

func mustLoadAlias(t *testing.T, a *App[aliasVals], idx []int, e env, args ...string) *Result[aliasVals] {
	t.Helper()
	res, err := loadAlias(t, a, idx, e, args...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func warnMsgs[V any](r *Result[V]) []string {
	var out []string
	for _, w := range r.Warnings {
		out = append(out, w.Msg)
	}
	return out
}

func TestAliasSurfaces(t *testing.T) {
	app := aliasApp()
	idx := []int{0}

	tests := []struct {
		name           string
		files          map[string]string
		env            env
		args           []string
		wantVal        string
		wantWarn       string
		wantWarnOrigin string
		wantOrigin     string
	}{
		{
			name:           "file surface via alias",
			files:          map[string]string{"config.yaml": "database:\n  ssl_mode: require\n"},
			wantVal:        "require",
			wantWarn:       "database.ssl_mode is deprecated, use database.sslmode",
			wantWarnOrigin: ":2:3",
			wantOrigin:     ":2:13",
		},
		{
			name:           "env surface via alias",
			env:            env{"CFGXT_DATABASE_SSL_MODE": "verify-full"},
			wantVal:        "verify-full",
			wantWarn:       "CFGXT_DATABASE_SSL_MODE is deprecated, use CFGXT_DATABASE_SSLMODE",
			wantWarnOrigin: "",
			wantOrigin:     "env CFGXT_DATABASE_SSL_MODE",
		},
		{
			name:           "flag surface via alias",
			args:           []string{"--database-ssl-mode=disable"},
			wantVal:        "disable",
			wantWarn:       "--database-ssl-mode is deprecated, use --database-sslmode",
			wantWarnOrigin: "",
			wantOrigin:     "flag --database-ssl-mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.env
			if e == nil {
				e = env{}
			}
			if tt.files != nil {
				dir := write(t, tt.files)
				e["CFG"] = dir
			}
			res := mustLoadAlias(t, app, idx, e, tt.args...)
			if res.Values.SSLMode != tt.wantVal {
				t.Errorf("value = %q, want %q", res.Values.SSLMode, tt.wantVal)
			}
			warns := warnMsgs(res)
			if len(warns) != 1 || !strings.Contains(warns[0], tt.wantWarn) {
				t.Errorf("warnings = %v, want containing %q", warns, tt.wantWarn)
			}
			if tt.wantWarnOrigin != "" && !strings.Contains(warns[0], tt.wantWarnOrigin) {
				t.Errorf("warnings[0] = %q, want containing %q", warns[0], tt.wantWarnOrigin)
			}
			origins := res.Origins[0]
			lastOrigin := origins[len(origins)-1].String()
			if !strings.Contains(lastOrigin, tt.wantOrigin) {
				t.Errorf("origin = %q, want containing %q", lastOrigin, tt.wantOrigin)
			}
		})
	}
}

func TestNewBeatsAliasInOneLayer(t *testing.T) {
	app := aliasApp()
	idx := []int{0}

	tests := []struct {
		name     string
		files    map[string]string
		env      env
		args     []string
		wantVal  string
		wantWarn string
	}{
		{
			name: "file layer new beats alias",
			files: map[string]string{
				"config.yaml": "database:\n  sslmode: verify-ca\n  ssl_mode: disable\n",
			},
			wantVal:  "verify-ca",
			wantWarn: "database.ssl_mode is ignored: database.sslmode is set too",
		},
		{
			name: "env layer new beats alias",
			env: env{
				"CFGXT_DATABASE_SSLMODE":  "verify-ca",
				"CFGXT_DATABASE_SSL_MODE": "disable",
			},
			wantVal:  "verify-ca",
			wantWarn: "CFGXT_DATABASE_SSL_MODE is ignored: CFGXT_DATABASE_SSLMODE is set too",
		},
		{
			name:     "flag layer new beats alias",
			args:     []string{"--database-sslmode=verify-ca", "--database-ssl-mode=disable"},
			wantVal:  "verify-ca",
			wantWarn: "--database-ssl-mode is ignored: --database-sslmode is set too",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.env
			if e == nil {
				e = env{}
			}
			if tt.files != nil {
				dir := write(t, tt.files)
				e["CFG"] = dir
			}
			res := mustLoadAlias(t, app, idx, e, tt.args...)
			if res.Values.SSLMode != tt.wantVal {
				t.Errorf("value = %q, want %q", res.Values.SSLMode, tt.wantVal)
			}
			warns := warnMsgs(res)
			if len(warns) != 1 || !strings.Contains(warns[0], tt.wantWarn) {
				t.Errorf("warnings = %v, want containing %q", warns, tt.wantWarn)
			}
		})
	}
}

func TestAliasOverriddenByHigherLayerStillWarns(t *testing.T) {
	app := aliasApp()
	idx := []int{0}

	t.Run("alias in file overridden by env", func(t *testing.T) {
		dir := write(t, map[string]string{"config.yaml": "database:\n  ssl_mode: disable\n"})
		e := env{"CFG": dir, "CFGXT_DATABASE_SSLMODE": "verify-ca"}
		res := mustLoadAlias(t, app, idx, e)
		if res.Values.SSLMode != "verify-ca" {
			t.Errorf("SSLMode = %q, want verify-ca", res.Values.SSLMode)
		}
		warns := warnMsgs(res)
		if len(warns) != 1 || !strings.Contains(warns[0], "database.ssl_mode is deprecated, use database.sslmode") {
			t.Errorf("warnings = %v, want deprecation warning for alias", warns)
		}
	})

	t.Run("alias in env overridden by flag", func(t *testing.T) {
		e := env{"CFGXT_DATABASE_SSL_MODE": "disable"}
		res := mustLoadAlias(t, app, idx, e, "--database-sslmode=verify-full")
		if res.Values.SSLMode != "verify-full" {
			t.Errorf("SSLMode = %q, want verify-full", res.Values.SSLMode)
		}
		warns := warnMsgs(res)
		if len(warns) != 1 || !strings.Contains(warns[0], "CFGXT_DATABASE_SSL_MODE is deprecated, use CFGXT_DATABASE_SSLMODE") {
			t.Errorf("warnings = %v, want deprecation warning for alias", warns)
		}
	})
}

func TestAliasKnownToUnknownScans(t *testing.T) {
	app := aliasApp()
	idx := []int{0}

	dir := write(t, map[string]string{"config.yaml": "database:\n  ssl_mode: require\n"})
	e := env{"CFG": dir, "CFGXT_DATABASE_SSL_MODE": "require"}
	res := mustLoadAlias(t, app, idx, e)

	for _, w := range warnMsgs(res) {
		if strings.Contains(w, "unknown key") || strings.Contains(w, "unknown environment variable") {
			t.Errorf("unexpected unknown warning: %s", w)
		}
	}
}

func TestHiddenAliasFlagNotInFlagUsages(t *testing.T) {
	app := aliasApp()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, app.Keys, []int{0})

	usages := fs.FlagUsages()
	if !strings.Contains(usages, "--database-sslmode") {
		t.Errorf("FlagUsages missing primary flag --database-sslmode:\n%s", usages)
	}
	if strings.Contains(usages, "--database-ssl-mode") {
		t.Errorf("FlagUsages should not list hidden alias flag --database-ssl-mode:\n%s", usages)
	}
}

func TestDeprecatedWarnsOnlyWhenSetNonZero(t *testing.T) {
	app := aliasApp()
	idx := []int{1, 2, 3, 4}

	t.Run("default only emits no warning", func(t *testing.T) {
		res := mustLoadAlias(t, app, idx, env{})
		if len(warnMsgs(res)) != 0 {
			t.Errorf("expected no warnings, got %v", warnMsgs(res))
		}
	})

	t.Run("zero values emit no warning", func(t *testing.T) {
		e := env{"CFGXT_SERVER_LEGACY": "", "CFGXT_SERVER_NUMBER": "0"}
		res := mustLoadAlias(t, app, idx, e, "--server-legacy=", "--server-number=0")
		if len(warnMsgs(res)) != 0 {
			t.Errorf("expected no warnings for zero values, got %v", warnMsgs(res))
		}
	})

	t.Run("non-zero string in file warns", func(t *testing.T) {
		dir := write(t, map[string]string{"config.yaml": "server:\n  legacy: oldval\n"})
		res := mustLoadAlias(t, app, idx, env{"CFG": dir})
		warns := warnMsgs(res)
		want := fmt.Sprintf("server.legacy is deprecated: use modern setting instead (%s:2:11)", filepath.Join(dir, "config.yaml"))
		if len(warns) != 1 || warns[0] != want {
			t.Errorf("got %v, want %q", warns, want)
		}
	})

	t.Run("non-zero int in env warns", func(t *testing.T) {
		e := env{"CFGXT_SERVER_NUMBER": "42"}
		res := mustLoadAlias(t, app, idx, e)
		warns := warnMsgs(res)
		want := "server.number is deprecated: count is obsolete (env CFGXT_SERVER_NUMBER)"
		if len(warns) != 1 || warns[0] != want {
			t.Errorf("got %v, want %q", warns, want)
		}
	})

	t.Run("non-zero in flag warns", func(t *testing.T) {
		res := mustLoadAlias(t, app, idx, env{}, "--server-legacy=oldval")
		warns := warnMsgs(res)
		want := "server.legacy is deprecated: use modern setting instead (flag --server-legacy)"
		if len(warns) != 1 || warns[0] != want {
			t.Errorf("got %v, want %q", warns, want)
		}
	})

	t.Run("irrelevant key does not warn", func(t *testing.T) {
		// server.opt has RelevantIf: []int{3} (log.active).
		// When log.active is false (default), server.opt is irrelevant and setting it should NOT warn with deprecation.
		res := mustLoadAlias(t, app, idx, env{"CFGXT_SERVER_OPT": "val"})
		for _, w := range warnMsgs(res) {
			if strings.Contains(w, "is deprecated") {
				t.Errorf("irrelevant key should not warn with deprecation, got: %s", w)
			}
		}
	})
}
