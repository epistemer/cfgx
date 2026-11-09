package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/epistemer/cfgx"
)

type secretVals struct{ Pw, Key cfgx.Secret }

func secretApp() *App[secretVals] {
	return &App[secretVals]{EnvPrefix: "CFGXT", Keys: []Key[secretVals]{
		{
			Path: "db.password", Env: "CFGXT_DB_PASSWORD", Type: "", Desc: "Password",
			FileEnv: true, MaxFileSize: 8,
			Aliases: []Alias{{Path: "db.pass", Env: "CFGXT_DB_PASS"}},
			Set:     func(v *secretVals, r string) error { return ParseSecret(r, &v.Pw) },
			IsSet:   func(v *secretVals) bool { return v.Pw.IsSet() },
		},
		{
			Path: "tls.key", Env: "CFGXT_TLS_KEY", Flag: "tls-key", Type: "file", Desc: "Key",
			File: true, MaxFileSize: 8, DefaultText: "nowhere.pem",
			Set:   func(v *secretVals, r string) error { return ParseSecret(r, &v.Key) },
			IsSet: func(v *secretVals) bool { return v.Key.IsSet() },
		},
	}}
}

func loadSecrets(t *testing.T, env map[string]string, idx ...int) (*Result[secretVals], error) {
	t.Helper()
	var environ []string
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	return Load(secretApp(), nil, idx, Options{
		LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Environ:   func() []string { return environ },
	})
}

func TestSecretFileEnv(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw")
	if err := os.WriteFile(pw, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The alias's _FILE works, with a deprecation warning naming it, and
	// _FILE names are known env vars.
	res, err := loadSecrets(t, map[string]string{"CFGXT_DB_PASS_FILE": pw}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Values.Pw.Reveal(); got != "s3cret" {
		t.Errorf("pw = %q", got)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Msg != "CFGXT_DB_PASS_FILE is deprecated, use CFGXT_DB_PASSWORD_FILE" {
		t.Errorf("warnings = %+v", res.Warnings)
	}

	// An empty _FILE doesn't count, so the plain variable is used alone.
	res, err = loadSecrets(t, map[string]string{"CFGXT_DB_PASSWORD": "x", "CFGXT_DB_PASSWORD_FILE": ""}, 0)
	if err != nil || res.Values.Pw.Reveal() != "x" || len(res.Warnings) != 0 {
		t.Errorf("err = %v, warnings = %+v", err, res.Warnings)
	}

	// A directory isn't a regular file; the limit is MaxFileSize.
	for _, c := range []struct{ path, want string }{
		{dir, dir + " isn't a regular file"},
		{writeTemp(t, dir, "big", "123456789"), filepath.Join(dir, "big") + " is larger than 8 "},
	} {
		_, err := loadSecrets(t, map[string]string{"CFGXT_DB_PASSWORD_FILE": c.path}, 0)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.path, err, c.want)
		}
	}
}

func TestSecretFileDefaultNotReadUnlessLoaded(t *testing.T) {
	// Key 1's default path doesn't exist; a command that doesn't load it
	// never reads it.
	if _, err := loadSecrets(t, map[string]string{"CFGXT_DB_PASSWORD": "x"}, 0); err != nil {
		t.Fatal(err)
	}
	_, err := loadSecrets(t, nil, 1)
	if err == nil || !strings.Contains(err.Error(), "nowhere.pem doesn't exist (default)") {
		t.Errorf("err = %v", err)
	}
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRelative(t *testing.T) {
	cases := map[string]bool{"keys.csv": true, "a/b": true, "": true}
	if runtime.GOOS == "windows" {
		cases[`\secrets\k`] = false
		cases[`/secrets/k`] = false
		cases[`D:k`] = false
		cases[`C:\k`] = false
	} else {
		cases["/secrets/k"] = false
	}
	for p, want := range cases {
		if got := relative(p); got != want {
			t.Errorf("relative(%q) = %t, want %t", p, got, want)
		}
	}
}

func TestBothSetOnIrrelevantKeyWarns(t *testing.T) {
	a := secretApp()
	a.Keys = append(a.Keys, Key[secretVals]{
		Path: "db.on", Env: "CFGXT_DB_ON", Type: "bool", Bool: true,
		Set:   func(*secretVals, string) error { return nil },
		IsSet: func(*secretVals) bool { return false },
	})
	a.Keys[0].RelevantIf = []int{2}
	env := map[string]string{"CFGXT_DB_PASSWORD": "x", "CFGXT_DB_PASSWORD_FILE": "pw"}
	res, err := Load(a, nil, []int{0}, Options{
		LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Environ:   func() []string { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Msg, "is set (env CFGXT_DB_PASSWORD) but ignored") {
		t.Errorf("warnings = %+v", res.Warnings)
	}
}
