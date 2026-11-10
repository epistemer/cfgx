package engine

import (
	"errors"
	"os"
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

// A record from an older epoch moves to history; one from a newer epoch is
// ignored and kept.
func TestEpochs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	a := storeApp(dir)
	a.Keys[1].Epoch = 2
	put(t, path, map[string]string{"rate": "250"}) // epoch 0
	res, err := loadStore(t, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values.Rate != 100 {
		t.Errorf("rate %d; want the default, 100", res.Values.Rate)
	}
	want := "rate: the stored value 250 is from epoch 0, and the key is at epoch 2 now; it moves to history, using the default"
	if w := stWarnings(res); w != want {
		t.Errorf("warning %q, want %q", w, want)
	}
	recs, _, _ := res.Store.Records()
	h, _ := res.Store.History("rate", 0)
	if _, still := recs["rate"]; still || len(h) != 2 || !h[0].Reset || h[0].Epoch != 2 || h[1].Raw != "250" {
		t.Errorf("records %v, history %+v", recs, h)
	}
	res.Store.Close()

	// The next start has nothing to say.
	res, err = loadStore(t, a, nil)
	if err != nil || len(res.Warnings) != 0 {
		t.Errorf("%v, warnings %q", err, stWarnings(res))
	}
	res.Store.Close()

	// A newer epoch's value is left alone.
	a.Keys[1].Epoch = 1
	b, err := store.Open(path, storage.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r := storage.Record{Raw: "300", Epoch: 3, UpdatedAt: time.Now()}
	if err := b.Apply([]storage.Change{{Key: "rate", Record: &r, History: r}}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	res, err = loadStore(t, a, nil)
	if err != nil || res.Values.Rate != 100 || !strings.Contains(stWarnings(res), "from epoch 3, newer than this program's 1") {
		t.Errorf("rate %d, %v, warnings %q", res.Values.Rate, err, stWarnings(res))
	}
	if recs, _, _ := res.Store.Records(); recs["rate"].Raw != "300" {
		t.Error("a newer epoch's record was removed")
	}
}

func loadReadOnly(t *testing.T, a *App[stVals]) *Result[stVals] {
	t.Helper()
	res, err := Load(a, nil, []int{0, 1, 2}, Options{Store: StoreReadOnly,
		LookupEnv: func(string) (string, bool) { return "", false }, Environ: func() []string { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Store != nil {
		res.Store.Close()
	}
	return res
}

// The config subcommands open the store read-only: a missing file stays
// missing, a held store is skipped with a warning, records from older
// epochs stay put, and there is no writer.
func TestStoreReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	a := storeApp(dir)
	if res := loadReadOnly(t, a); res.Values.Rate != 100 || len(res.Warnings) != 0 || res.Writer != nil {
		t.Errorf("missing file: rate %d, warnings %q", res.Values.Rate, stWarnings(res))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the file was created: %v", err)
	}

	put(t, path, map[string]string{"rate": "250"})
	if res := loadReadOnly(t, a); res.Values.Rate != 250 || res.Writer != nil {
		t.Errorf("stored: rate %d", res.Values.Rate)
	}

	a.Keys[1].Epoch = 2
	res := loadReadOnly(t, a)
	if res.Values.Rate != 100 || !strings.Contains(stWarnings(res), "the program moves it to history when it next opens the store") {
		t.Errorf("older epoch: rate %d, warnings %q", res.Values.Rate, stWarnings(res))
	}
	b, err := store.Open(path, storage.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if recs, _, _ := b.Records(); recs["rate"].Raw != "250" {
		t.Error("a read-only load retired the record")
	}
	// An empty store key names no file: nothing to say.
	a.Keys[0].DefaultText = ""
	if res := loadReadOnly(t, a); len(res.Warnings) != 0 || res.Values.Rate != 100 {
		t.Errorf("empty key: rate %d, warnings %q", res.Values.Rate, stWarnings(res))
	}
	a.Keys[0].DefaultText = path
	// Held by another opener: values without the store, and a warning.
	res = loadReadOnly(t, a)
	b.Close()
	if !strings.Contains(stWarnings(res), "showing values without the store: ") || !strings.Contains(stWarnings(res), storage.ErrLocked.Error()) {
		t.Errorf("held: warnings %q", stWarnings(res))
	}
}

func TestStoreSource(t *testing.T) {
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	env := []Origin{{Layer: LayerDefault}, {LayerEnv, "ZP_X"}, {LayerFlag, "--x"}}
	for _, c := range []struct {
		r    storage.Record
		want string
	}{
		{storage.Record{UpdatedAt: day, UpdatedBy: "admin@zp", Reason: "PCI review"}, `store (2026-11-02, admin@zp: "PCI review"), hides env ZP_X, flag --x`},
		{storage.Record{UpdatedAt: day, Reason: "PCI review"}, `store (2026-11-02, "PCI review"), hides env ZP_X, flag --x`},
		{storage.Record{UpdatedBy: "ops"}, `store (ops), hides env ZP_X, flag --x`},
		{storage.Record{}, `store, hides env ZP_X, flag --x`},
	} {
		if got := storeSource(c.r, env); got != c.want {
			t.Errorf("%+v: %q, want %q", c.r, got, c.want)
		}
	}
	if got := storeSource(storage.Record{UpdatedAt: day}, env[:1]); got != "store (2026-11-02)" {
		t.Errorf("nothing hidden: %q", got)
	}
}
