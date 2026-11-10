package engine

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/epistemer/cfgx/internal/storage"
	"github.com/epistemer/cfgx/store"
)

type stVals struct {
	Bolt        string
	Rate, Fixed int
}

func storeApp(dir string) *App[stVals] {
	return &App[stVals]{EnvPrefix: "CFGXT", OpenStore: store.Open, StoreKey: 0, Keys: []Key[stVals]{
		{Path: "bolt", Env: "CFGXT_BOLT", DefaultText: filepath.Join(dir, "app.db"),
			Set:   func(v *stVals, r string) error { return ParsePath(r, &v.Bolt) },
			IsSet: func(v *stVals) bool { return v.Bolt != "" }, Show: func(v *stVals) any { return v.Bolt }},
		{Path: "rate", Env: "CFGXT_RATE", Flag: "rate", Mutable: true,
			Set:      func(v *stVals, r string) error { return ParseInt(r, &v.Rate) },
			IsSet:    func(v *stVals) bool { return v.Rate != 0 },
			Check:    func(v *stVals) error { return AtMost(v.Rate, 1000, "1000") },
			Required: true,
			Default:  func(v *stVals) { v.Rate = 100 }, DefaultText: "100"},
		{Path: "fixed", Env: "CFGXT_FIXED",
			Set:   func(v *stVals, r string) error { return ParseInt(r, &v.Fixed) },
			IsSet: func(v *stVals) bool { return v.Fixed != 0 }},
	}}
}

func put(t *testing.T, path string, recs map[string]string) {
	t.Helper()
	b, err := store.Open(path, storage.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var cs []storage.Change
	for k, v := range recs {
		r := storage.Record{Raw: v, UpdatedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), UpdatedBy: "admin@zp"}
		cs = append(cs, storage.Change{Key: k, Record: &r, History: r})
	}
	if err := b.Apply(cs); err != nil {
		t.Fatal(err)
	}
}

func loadStore(t *testing.T, a *App[stVals], env map[string]string, args ...string) (*Result[stVals], error) {
	t.Helper()
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs, a.Keys, []int{0, 1, 2})
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	res, err := Load(a, fs, []int{0, 1, 2}, Options{Store: StoreReadWrite,
		LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Environ:   func() []string { return nil }})
	if res != nil && res.Store != nil {
		st := res.Store
		t.Cleanup(func() { st.Close() })
	}
	return res, err
}

func stWarnings(res *Result[stVals]) string {
	var ws []string
	for _, w := range res.Warnings {
		ws = append(ws, w.Msg)
	}
	return strings.Join(ws, "\n")
}

// A stored value wins over every other layer of a mutable key, and says
// what it hides; other keys ignore the store.
func TestStoreLayer(t *testing.T) {
	dir := t.TempDir()
	a := storeApp(dir)
	put(t, filepath.Join(dir, "app.db"), map[string]string{"rate": "250", "fixed": "7"})

	res, err := loadStore(t, a, map[string]string{"CFGXT_RATE": "50"}, "--rate", "60")
	if err != nil {
		t.Fatal(err)
	}
	if res.Values.Rate != 250 || res.Values.Fixed != 0 {
		t.Errorf("rate %d fixed %d; want the stored 250, and fixed unaffected", res.Values.Rate, res.Values.Fixed)
	}
	if o := res.Origins[1]; o[len(o)-1].Layer != LayerStore || o[len(o)-1].String() != "store" {
		t.Errorf("origins %v", o)
	}
	if w := stWarnings(res); w != "rate: stored value 250 (admin@zp, 2026-11-02) overrides flag --rate=60" {
		t.Errorf("warnings %q", w)
	}

	// With nothing else set, there's no shadow warning. (One opener at a
	// time: close the first.)
	res.Store.Close()
	res, err = loadStore(t, a, nil)
	if err != nil || res.Values.Rate != 250 || len(res.Warnings) != 0 {
		t.Errorf("rate %d, %v, warnings %q", res.Values.Rate, err, stWarnings(res))
	}
}

// A stored value that no longer passes the schema is ignored, with a
// warning, and the next layer applies; the record stays.
func TestInvalidStoredValue(t *testing.T) {
	for stored, why := range map[string]string{"x": `"x" is not an integer`, "5000": "must be at most 1000", "0": "is required, but the stored value is empty"} {
		dir := t.TempDir()
		a := storeApp(dir)
		put(t, filepath.Join(dir, "app.db"), map[string]string{"rate": stored})
		res, err := loadStore(t, a, map[string]string{"CFGXT_RATE": "50"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Values.Rate != 50 || res.Invalid[1] != why {
			t.Errorf("stored %q: rate %d, invalid %q", stored, res.Values.Rate, res.Invalid[1])
		}
		want := "rate: the stored value is ignored (" + why + "); using env CFGXT_RATE"
		if w := stWarnings(res); w != want {
			t.Errorf("warning %q, want %q", w, want)
		}
		if recs, _, _ := res.Store.Records(); recs["rate"].Raw != stored {
			t.Error("the invalid record was removed")
		}
	}
}

func TestStoreFailures(t *testing.T) {
	dir := t.TempDir()
	a := storeApp(dir)
	// The store key's directory doesn't exist.
	a.Keys[0].DefaultText = filepath.Join(dir, "missing", "app.db")
	if _, err := loadStore(t, a, nil); err == nil || !strings.Contains(err.Error(), "app.db") {
		t.Errorf("missing directory: %v", err)
	}
	// Held by another opener.
	a = storeApp(dir)
	held, err := store.Open(filepath.Join(dir, "app.db"), storage.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if _, err := loadStore(t, a, nil); !errors.Is(err, storage.ErrLocked) {
		t.Errorf("held store: %v", err)
	}
}
