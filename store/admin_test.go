package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	bolt "go.etcd.io/bbolt"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/engine"
)

type mode int

type adVals struct {
	Bolt  string
	Wait  time.Duration
	Tags  []string
	Mode  mode
	Port  int
	Token cfgx.Secret
}

var modeTexts, modeValues = []string{"serial", "parallel"}, []mode{1, 2}

// adminApp has a store, three mutable keys of different kinds, and two
// static ones, one of them a secret.
func adminApp(dir string) *engine.App[adVals] {
	return &engine.App[adVals]{EnvPrefix: "AD", OpenStore: Open, StoreKey: 0, Keys: []engine.Key[adVals]{
		{Path: "bolt", Kind: "path", DefaultText: filepath.Join(dir, "app.db"),
			Set:   func(v *adVals, r string) error { return engine.ParsePath(r, &v.Bolt) },
			IsSet: func(v *adVals) bool { return v.Bolt != "" }, Show: func(v *adVals) any { return v.Bolt }},
		{Path: "wait", Env: "AD_WAIT", Kind: "duration", Mutable: true, Desc: "Wait", Tags: []string{"ops"},
			Schema: `{"type":"string"}`, DefaultText: "5s", Default: func(v *adVals) { v.Wait = 5 * time.Second },
			Set:   func(v *adVals, r string) error { return engine.ParseDuration(r, &v.Wait) },
			IsSet: func(v *adVals) bool { return v.Wait != 0 }, Show: func(v *adVals) any { return v.Wait.String() },
			Format: func(v *adVals) (string, error) { return engine.FormatDuration(v.Wait), nil },
			Check:  func(v *adVals) error { return engine.AtMost(v.Wait, time.Hour, "1h") }},
		{Path: "tags", Env: "AD_TAGS", Kind: "strings", Mutable: true,
			SetItems: func(v *adVals, items []string) error { return engine.ParseStrings(items, &v.Tags) },
			IsSet:    func(v *adVals) bool { return len(v.Tags) != 0 }, Show: func(v *adVals) any { return v.Tags },
			Format: func(v *adVals) (string, error) { return engine.FormatStrings(v.Tags), nil }},
		{Path: "mode", Env: "AD_MODE", Kind: "enum", Mutable: true, Enum: modeTexts,
			Set:   func(v *adVals, r string) error { return engine.ParseEnum(r, modeTexts, modeValues, &v.Mode) },
			IsSet: func(v *adVals) bool { return v.Mode != 0 }, Show: func(v *adVals) any { return engine.ShowEnum(v.Mode, modeTexts, modeValues) },
			Format: func(v *adVals) (string, error) { return engine.FormatEnum(v.Mode, modeTexts, modeValues) }},
		{Path: "port", Env: "AD_PORT", Kind: "int", DefaultText: "80", Default: func(v *adVals) { v.Port = 80 },
			Set:   func(v *adVals, r string) error { return engine.ParseInt(r, &v.Port) },
			IsSet: func(v *adVals) bool { return v.Port != 0 }, Show: func(v *adVals) any { return v.Port }},
		{Path: "token", Env: "AD_TOKEN", Kind: "secret",
			Set:   func(v *adVals, r string) error { return engine.ParseSecret(r, &v.Token) },
			IsSet: func(v *adVals) bool { return v.Token.IsSet() }, Show: func(v *adVals) any { return engine.ShowSecret(v.Token) }},
	}}
}

func adminStore(t *testing.T, env map[string]string) *Store {
	t.Helper()
	a := adminApp(t.TempDir())
	res, err := engine.Load(a, pflag.NewFlagSet("t", pflag.ContinueOnError), []int{0, 1, 2, 3, 4, 5},
		engine.Options{Store: engine.StoreReadWrite,
			LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
			Environ:   func() []string { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Writer.Close() })
	return New(res.Writer)
}

func get(t *testing.T, s *Store, key string) Setting {
	t.Helper()
	st, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// Keys and List cover every key of the command, static ones included,
// with secrets masked.
func TestKeysAndList(t *testing.T) {
	s := adminStore(t, map[string]string{"AD_TOKEN": "hunter2", "AD_PORT": "8080"})
	ks := s.Keys()
	if len(ks) != 6 {
		t.Fatalf("%d keys", len(ks))
	}
	w := ks[1]
	if w.Key != "wait" || w.Type != "duration" || !w.Mutable || w.Default != "5s" || w.Description != "Wait" ||
		!slices.Equal(w.Tags, []string{"ops"}) || !json.Valid(w.Schema) {
		t.Errorf("wait: %+v", w)
	}
	if ks[3].Type != "enum" || !slices.Equal(ks[3].Enum, modeTexts) || ks[4].Mutable {
		t.Errorf("mode %+v, port %+v", ks[3], ks[4])
	}
	if st := get(t, s, "token"); st.Value != "******" || st.Source != "env AD_TOKEN" {
		t.Errorf("token: %+v", st)
	}
	if st := get(t, s, "port"); st.Value != "8080" || st.Source != "env AD_PORT" || st.Change != nil {
		t.Errorf("port: %+v", st)
	}
	if st := get(t, s, "wait"); st.Value != "5s" || st.Source != "default" {
		t.Errorf("wait: %+v", st)
	}
	if st := get(t, s, "mode"); st.Value != "" || st.Source != "unset" {
		t.Errorf("mode: %+v", st)
	}
	if _, err := s.Get("nope"); err == nil || err.Error() != "nope isn't a key of this command" {
		t.Errorf("unknown key: %v", err)
	}
}

// Text written through Update reads back as canonical text, which Update
// takes back unchanged: writing it again changes nothing.
func TestUpdateRoundTrips(t *testing.T) {
	s := adminStore(t, nil)
	ctx := context.Background()
	for key, c := range map[string]struct{ in, canon string }{
		"wait": {"90s", "1m30s"},
		"tags": {`a, "b,c", " d"`, `a,"b,c"," d"`},
		"mode": {" parallel ", "parallel"},
	} {
		if err := s.Update(ctx, map[string]string{key: c.in}); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		st := get(t, s, key)
		if st.Value != c.canon || st.Source != "store" || st.Change == nil || st.Change.Value != c.in {
			t.Errorf("%s: %+v, change %+v", key, st, st.Change)
		}
		if err := s.Update(ctx, map[string]string{key: st.Value}); err != nil {
			t.Fatalf("%s: writing its value back: %v", key, err)
		}
		if again := get(t, s, key); again.Value != c.canon {
			t.Errorf("%s: %q after writing it back", key, again.Value)
		}
	}
}

// Update and Reset write history with who and why; a stored value shows
// what it hides; a reset brings the env value back.
func TestUpdateResetHistory(t *testing.T) {
	s := adminStore(t, map[string]string{"AD_WAIT": "10s"})
	ctx := context.Background()
	if err := s.Update(ctx, map[string]string{"wait": "1m", "mode": "serial"}, cfgx.By("admin"), cfgx.Reason("slow")); err != nil {
		t.Fatal(err)
	}
	st := get(t, s, "wait")
	if st.Value != "1m0s" || !slices.Equal(st.Shadowed, []string{"env AD_WAIT"}) || st.Change.UpdatedBy != "admin" || st.Change.Reason != "slow" {
		t.Errorf("wait: %+v, change %+v", st, st.Change)
	}
	if err := s.Reset(ctx, []string{"wait"}, cfgx.By("ops")); err != nil {
		t.Fatal(err)
	}
	if st := get(t, s, "wait"); st.Value != "10s" || st.Source != "env AD_WAIT" || st.Change != nil || st.Shadowed != nil {
		t.Errorf("after reset: %+v", st)
	}
	h, err := s.History("wait", 0)
	if err != nil || len(h) != 2 || !h[0].Reset || h[0].UpdatedBy != "ops" || h[1].Value != "1m" || h[1].Reset {
		t.Errorf("history %+v, %v", h, err)
	}
	if h, _ := s.History("wait", 1); len(h) != 1 {
		t.Errorf("limit 1: %d entries", len(h))
	}
	if h, err := s.History("port", 0); err != nil || len(h) != 0 {
		t.Errorf("static key's history: %v, %v", h, err)
	}
	if _, err := s.History("nope", 0); err == nil {
		t.Error("history of an unknown key")
	}
}

// A bad write, by value, key or mix, changes nothing.
func TestUpdateRejects(t *testing.T) {
	s := adminStore(t, nil)
	ctx := context.Background()
	for _, c := range []struct {
		values map[string]string
		reset  []string
		want   string
	}{
		{map[string]string{"wait": "2h"}, nil, "wait: must be at most 1h"},
		{map[string]string{"wait": "1m", "mode": "fast"}, nil, `"fast" isn't one of serial, parallel`},
		{map[string]string{"port": "1"}, nil, "port isn't mutable"},
		{map[string]string{"nope": "1"}, nil, "nope isn't a key of this command"},
		{nil, []string{"token"}, "token isn't mutable"},
	} {
		var err error
		if c.values != nil {
			err = s.Update(ctx, c.values)
		} else {
			err = s.Reset(ctx, c.reset)
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v %v: %v, want %q", c.values, c.reset, err, c.want)
		}
	}
	for _, st := range s.List() {
		if st.Source == "store" {
			t.Errorf("%s was written", st.Key)
		}
	}
	if err := s.Update(ctx, nil); err != nil {
		t.Errorf("an empty update: %v", err)
	}
}

// The app keeps its own buckets in the same file, through DB.
func TestDB(t *testing.T) {
	s := adminStore(t, nil)
	if string(s.Bucket()) != "cfgx" || s.DB() == nil {
		t.Fatalf("bucket %q, db %v", s.Bucket(), s.DB())
	}
	err := s.DB().Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("app"))
		if err != nil {
			return err
		}
		return b.Put([]byte("k"), []byte("v"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(context.Background(), map[string]string{"wait": "1s"}); err != nil {
		t.Fatal(err)
	}
	s.DB().View(func(tx *bolt.Tx) error {
		if v := tx.Bucket([]byte("app")).Get([]byte("k")); string(v) != "v" {
			t.Errorf("the app's bucket holds %q", v)
		}
		return nil
	})
}
