package engine

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePathURLLocation(t *testing.T) {
	var p string
	for raw, want := range map[string]string{"": "", "a//b/../c": filepath.Join("a", "c"), "./x/": "x", " x ": " x "} {
		if err := ParsePath(raw, &p); err != nil || p != want {
			t.Errorf("ParsePath(%q) = %q, %v; want %q", raw, p, err, want)
		}
	}

	var b []byte
	if ParseBytes("", &b); b != nil {
		t.Errorf("ParseBytes(\"\") = %q", b)
	}

	var u *url.URL
	if err := ParseURL(" https://h:1/p?q=1 ", &u); err != nil || u.Host != "h:1" || u.RawQuery != "q=1" {
		t.Errorf("ParseURL = %v, %v", u, err)
	}
	if err := ParseURL("file:///etc/x", &u); err != nil || u.Path != "/etc/x" {
		t.Errorf("ParseURL(file) = %v, %v", u, err)
	}
	if err := ParseURL("", &u); err != nil || u != nil {
		t.Errorf("ParseURL(\"\") = %v, %v", u, err)
	}
	// No message shows a password.
	for raw, want := range map[string]string{
		"//u:hunter2@host/db":                 "isn't an absolute URL; it needs a scheme",
		"db/app?password=hunter2":             "isn't an absolute URL; it needs a scheme",
		"user:hunter2@host/db":                "isn't an absolute URL: user: isn't followed by // or /",
		"https://u:hunter2@h:port":            "isn't a valid URL: invalid port",
		"https://user:hunter2 host.internal/": "isn't a valid URL: invalid port",
		"https://h/\x01hunter2":               "isn't a valid URL: invalid control character in URL",
		"localhost:5432":                      "isn't an absolute URL: localhost: isn't followed",
	} {
		err := ParseURL(raw, &u)
		if err == nil || !strings.HasPrefix(err.Error(), want) || strings.Contains(err.Error(), "hunter2") {
			t.Errorf("ParseURL(%q) = %v; want %q, without the password", raw, err, want)
		}
	}

	var loc *time.Location
	if err := ParseLocation("UTC", &loc); err != nil || loc != time.UTC {
		t.Errorf("ParseLocation(UTC) = %v, %v", loc, err)
	}
	if err := ParseLocation("Mars/Base", &loc); err == nil || !strings.Contains(err.Error(), `"Mars/Base" isn't a time zone`) {
		t.Errorf("ParseLocation(Mars/Base) = %v", err)
	}
}

type locVals struct {
	On  bool
	Loc *time.Location
}

func locApp(def string) *App[locVals] {
	return &App[locVals]{EnvPrefix: "CFGXT", Keys: []Key[locVals]{
		{
			Path: "on", Env: "CFGXT_ON", Flag: "on", Type: "bool", Desc: "On",
			Set:   func(v *locVals, r string) error { return ParseBool(r, &v.On) },
			IsSet: func(v *locVals) bool { return v.On },
		},
		{
			Path: "tz", Env: "CFGXT_TZ", Flag: "tz", Type: "location", Desc: "Zone", DefaultText: def,
			RelevantIf: []int{0},
			Set:        func(v *locVals, r string) error { return ParseLocation(r, &v.Loc) },
			IsSet:      func(v *locVals) bool { return v.Loc != nil },
		},
	}}
}

// A default parsed at load time is parsed only if it wins, and an
// irrelevant key still gets it.
func TestParsedDefault(t *testing.T) {
	load := func(def string, env map[string]string) (*Result[locVals], error) {
		return Load(locApp(def), nil, []int{0, 1}, Options{
			LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
			Environ:   func() []string { return nil },
		})
	}
	res, err := load("UTC", nil)
	if err != nil || res.Values.Loc != time.UTC {
		t.Fatalf("irrelevant key: %v, %v; want its default", res, err)
	}
	if _, err := load("Mars/Base", map[string]string{"CFGXT_ON": "true"}); err == nil || !strings.Contains(err.Error(), "(default)") {
		t.Errorf("bad default that wins: %v", err)
	}
	res, err = load("Mars/Base", map[string]string{"CFGXT_ON": "true", "CFGXT_TZ": "UTC"})
	if err != nil || res.Values.Loc != time.UTC {
		t.Errorf("bad default overridden: %v, %v", res, err)
	}
	if _, err := load("Mars/Base", nil); err != nil {
		t.Errorf("bad default on an irrelevant key: %v", err)
	}
}
