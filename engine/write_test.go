package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/pflag"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/storage"
	"github.com/epistemer/cfgx/store"
)

type wrVals struct {
	Bolt               string
	Rate, Burst, Fixed int
	Token              cfgx.Secret
}

// writeApp has a store, two mutable keys and two static ones; burst and
// fixed are mutually exclusive.
func writeApp(dir string) *App[wrVals] {
	ints := func(get func(*wrVals) *int) (func(*wrVals, string) error, func(*wrVals) bool, func(*wrVals) (string, error)) {
		return func(v *wrVals, r string) error { return ParseInt(r, get(v)) },
			func(v *wrVals) bool { return *get(v) != 0 },
			func(v *wrVals) (string, error) { return FormatInt(*get(v)), nil }
	}
	rSet, rIs, rFmt := ints(func(v *wrVals) *int { return &v.Rate })
	bSet, bIs, bFmt := ints(func(v *wrVals) *int { return &v.Burst })
	fSet, fIs, _ := ints(func(v *wrVals) *int { return &v.Fixed })
	return &App[wrVals]{EnvPrefix: "WT", OpenStore: store.Open, StoreKey: 0, Keys: []Key[wrVals]{
		{Path: "bolt", Env: "WT_BOLT", DefaultText: filepath.Join(dir, "app.db"),
			Set:   func(v *wrVals, r string) error { return ParsePath(r, &v.Bolt) },
			IsSet: func(v *wrVals) bool { return v.Bolt != "" }, Show: func(v *wrVals) any { return v.Bolt }},
		{Path: "rate", Env: "WT_RATE", Mutable: true, Required: true, Epoch: 2,
			Set: rSet, IsSet: rIs, Format: rFmt,
			Check:   func(v *wrVals) error { return AtMost(v.Rate, 1000, "1000") },
			Default: func(v *wrVals) { v.Rate = 100 }, DefaultText: "100"},
		{Path: "burst", Env: "WT_BURST", Mutable: true, Set: bSet, IsSet: bIs, Format: bFmt},
		{Path: "fixed", Env: "WT_FIXED", Set: fSet, IsSet: fIs},
		{Path: "token", Env: "WT_TOKEN", FileEnv: true, MaxFileSize: 1 << 20,
			Set:   func(v *wrVals, r string) error { return ParseSecret(r, &v.Token) },
			IsSet: func(v *wrVals) bool { return v.Token.IsSet() }},
		{Path: "other", Env: "WT_OTHER", Mutable: true, Set: bSet, IsSet: bIs, Format: bFmt},
	}, Rules: []Rule{{Kind: AtMostOne, Group: "", Keys: []int{2, 3}}}}
}

// loadWriter loads every key but "other", as a command that has the store.
func loadWriter(t *testing.T, a *App[wrVals], env map[string]string) *Writer[wrVals] {
	t.Helper()
	res, err := Load(a, pflag.NewFlagSet("t", pflag.ContinueOnError), []int{0, 1, 2, 3, 4}, Options{Store: StoreReadWrite,
		LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Environ:   func() []string { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Writer.Close() })
	return res.Writer
}

func history(t *testing.T, w *Writer[wrVals], key string) []storage.Record {
	t.Helper()
	h, err := w.backend.History(key, 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func setRate(n int) func(*Edits[wrVals]) error {
	return func(e *Edits[wrVals]) error { e.Set(1, func(v *wrVals) { v.Rate = n }); return nil }
}

// A write stores the value with who, when and why, stamped with the key's
// epoch; a reset brings back the env value; resetting what isn't stored
// changes nothing.
func TestWriteAndReset(t *testing.T) {
	w := loadWriter(t, writeApp(t.TempDir()), map[string]string{"WT_RATE": "50"})
	ctx := context.Background()
	if err := w.Write(ctx, setRate(200), cfgx.By("admin@zp"), cfgx.Reason("Black Friday")); err != nil {
		t.Fatal(err)
	}
	if r := w.Snapshot().Load().Rate; r != 200 {
		t.Errorf("rate %d after the write", r)
	}
	recs, _, _ := w.backend.Records()
	if r := recs["rate"]; r.Raw != "200" || r.Epoch != 2 || r.UpdatedBy != "admin@zp" || r.Reason != "Black Friday" || r.UpdatedAt.IsZero() {
		t.Errorf("record %+v", r)
	}
	if o := w.res.Origins[1]; o[len(o)-1].Layer != LayerStore {
		t.Errorf("origins %v", o)
	}

	reset := func(e *Edits[wrVals]) error { e.Reset(1); return nil }
	if err := w.Write(ctx, reset, cfgx.By("ops")); err != nil {
		t.Fatal(err)
	}
	if r := w.Snapshot().Load().Rate; r != 50 {
		t.Errorf("rate %d after the reset; want env's 50", r)
	}
	recs, _, _ = w.backend.Records()
	h := history(t, w, "rate")
	if _, still := recs["rate"]; still || len(h) != 2 || !h[0].Reset || h[0].UpdatedBy != "ops" || h[1].Raw != "200" {
		t.Errorf("records %v, history %+v", recs, h)
	}
	if err := w.Write(ctx, reset); err != nil || len(history(t, w, "rate")) != 2 {
		t.Errorf("a second reset: %v, %d history entries", err, len(history(t, w, "rate")))
	}
	// Setting the stored value again writes nothing.
	_ = w.Write(ctx, setRate(300))
	_ = w.Write(ctx, setRate(300))
	if n := len(history(t, w, "rate")); n != 3 {
		t.Errorf("%d history entries; want 3", n)
	}
}

// A write that fails validation, by itself or with the rest of the
// configuration, changes nothing: not the store, not the snapshot.
func TestWriteInvalid(t *testing.T) {
	w := loadWriter(t, writeApp(t.TempDir()), map[string]string{"WT_FIXED": "7"})
	ctx := context.Background()
	before := w.Snapshot().Load()
	for _, tc := range []struct {
		name string
		fill func(*Edits[wrVals]) error
		want string
	}{
		{"constraint", setRate(5000), "rate: must be at most 1000"},
		{"required", setRate(0), "rate: is required, but the stored value is empty"},
		{"not parsed", func(e *Edits[wrVals]) error { e.SetText(1, "fast"); return nil }, `"fast" is not an integer`},
		{"rule", func(e *Edits[wrVals]) error { e.Set(2, func(v *wrVals) { v.Burst = 3 }); return nil }, "burst and fixed are mutually exclusive"},
		{"one bad of two", func(e *Edits[wrVals]) error {
			e.Set(1, func(v *wrVals) { v.Rate = 10 })
			e.Set(2, func(v *wrVals) { v.Burst = 3 })
			return nil
		}, "mutually exclusive"},
		{"static", func(e *Edits[wrVals]) error { e.SetText(3, "1"); return nil }, "fixed isn't mutable"},
		{"not the command's", func(e *Edits[wrVals]) error { e.SetText(5, "1"); return nil }, "other isn't a key of this command"},
		{"too large", func(e *Edits[wrVals]) error { e.SetText(1, strings.Repeat("1", storage.MaxValue+1)); return nil }, "larger than 64KiB"},
		{"no such key", func(e *Edits[wrVals]) error { e.SetText(99, "1"); return nil }, "no key 99"},
		{"no such key to set", func(e *Edits[wrVals]) error { e.Set(-1, func(*wrVals) {}); return nil }, "no key -1"},
		{"fill fails", func(e *Edits[wrVals]) error { e.Set(1, func(v *wrVals) { v.Rate = 10 }); return errors.New("no") }, "no"},
	} {
		err := w.Write(ctx, tc.fill)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	if w.Snapshot().Load() != before {
		t.Error("the snapshot changed")
	}
	if recs, _, _ := w.backend.Records(); len(recs) != 0 {
		t.Errorf("records %v", recs)
	}
	// A canceled context stops the write before it commits.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := w.Write(cctx, setRate(10)); !errors.Is(err, context.Canceled) || w.Snapshot().Load().Rate != 100 {
		t.Errorf("canceled: %v, rate %d", err, w.Snapshot().Load().Rate)
	}
	// After the store closes, writes fail.
	w.Close()
	if err := w.Write(ctx, setRate(10)); err == nil {
		t.Error("a write after Close succeeded")
	}
}

// A write loads the other keys from what they loaded from at startup: a
// secret file that changed since isn't read again.
func TestWriteKeepsBootstrap(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := loadWriter(t, writeApp(dir), map[string]string{"WT_TOKEN_FILE": tok})
	if err := os.WriteFile(tok, []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), setRate(10)); err != nil {
		t.Fatal(err)
	}
	if v := w.Snapshot().Load(); v.Rate != 10 || v.Token.Reveal() != "old" {
		t.Errorf("rate %d, token %q", v.Rate, v.Token.Reveal())
	}
}

// Concurrent writers lose nothing: every write is in the history, and the
// records and the snapshot agree at the end.
func TestConcurrentWrites(t *testing.T) {
	w := loadWriter(t, writeApp(t.TempDir()), nil)
	var wg sync.WaitGroup
	const n = 20
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fill := setRate(i + 1)
			if i%2 == 1 {
				fill = func(e *Edits[wrVals]) error { e.Set(2, func(v *wrVals) { v.Burst = i + 1 }); return nil }
			}
			if err := w.Write(context.Background(), fill, cfgx.By(fmt.Sprint(i))); err != nil {
				t.Error(err)
			}
			_ = w.Snapshot().Load().Rate // getters read while others write
		}()
	}
	wg.Wait()
	if a, b := len(history(t, w, "rate")), len(history(t, w, "burst")); a+b != n {
		t.Errorf("%d + %d history entries; want %d", a, b, n)
	}
	recs, _, _ := w.backend.Records()
	v := w.Snapshot().Load()
	if recs["rate"].Raw != fmt.Sprint(v.Rate) || recs["burst"].Raw != fmt.Sprint(v.Burst) || v.Rate == 0 || v.Burst == 0 {
		t.Errorf("records %v, values %+v", recs, v)
	}
}

// OnChange callbacks run after the swap, one at a time, in commit order,
// with the keys whose values changed; a callback may write.
func TestOnChange(t *testing.T) {
	w := loadWriter(t, writeApp(t.TempDir()), map[string]string{"WT_RATE": "50"})
	ctx := context.Background()
	var mu sync.Mutex
	var got []string
	var running, overlap int32
	w.OnChange(func(paths []string) {
		mu.Lock()
		running++
		if running > 1 {
			overlap++
		}
		got = append(got, fmt.Sprintf("%v rate=%d", paths, w.Snapshot().Load().Rate))
		mu.Unlock()
		mu.Lock()
		running--
		mu.Unlock()
	})
	burst := func(n int) func(*Edits[wrVals]) error {
		return func(e *Edits[wrVals]) error { e.Set(2, func(v *wrVals) { v.Burst = n }); return nil }
	}
	_ = w.Write(ctx, setRate(200))
	_ = w.Write(ctx, burst(3))
	_ = w.Write(ctx, func(e *Edits[wrVals]) error {
		e.Set(1, func(v *wrVals) { v.Rate = 300 })
		e.Set(2, func(v *wrVals) { v.Burst = 4 })
		return nil
	})
	_ = w.Write(ctx, setRate(300))  // no change: no callback
	_ = w.Write(ctx, setRate(5000)) // invalid: no callback
	if want := "[[rate] rate=200 [burst] rate=200 [rate burst] rate=300]"; fmt.Sprint(got) != want {
		t.Errorf("got %v\nwant %s", got, want)
	}

	// Storing the value the env var gives changes the value from 300; the
	// reset then changes no value: no callback.
	got = nil
	_ = w.Write(ctx, setRate(50))
	_ = w.Write(ctx, func(e *Edits[wrVals]) error { e.Reset(1); return nil })
	if fmt.Sprint(got) != "[[rate] rate=50]" {
		t.Errorf("got %v", got)
	}

	// A callback that writes doesn't deadlock; its write's callback runs
	// after it returns.
	got = nil
	once := sync.Once{}
	w.OnChange(func(paths []string) {
		once.Do(func() {
			if err := w.Write(ctx, burst(9)); err != nil {
				t.Error(err)
			}
		})
	})
	_ = w.Write(ctx, setRate(70))
	if len(got) != 2 || got[1] != "[burst] rate=70" {
		t.Errorf("got %v", got)
	}

	// Concurrent writers: every change is reported, never two callbacks at
	// once.
	got = nil
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Write(ctx, setRate(100+i))
		}()
	}
	wg.Wait()
	if len(got) != 20 || overlap != 0 {
		t.Errorf("%d callbacks, %d overlapping", len(got), overlap)
	}
}

// A callback that panics doesn't stop the others, or later changes; the
// writer running it panics once they have run.
func TestOnChangePanic(t *testing.T) {
	w := loadWriter(t, writeApp(t.TempDir()), nil)
	var calls []string
	w.OnChange(func(p []string) {
		calls = append(calls, "a")
		if len(calls) == 1 {
			panic("boom")
		}
	})
	w.OnChange(func([]string) { calls = append(calls, "b") })
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v", r)
			}
		}()
		_ = w.Write(context.Background(), setRate(10))
	}()
	if err := w.Write(context.Background(), setRate(20)); err != nil || fmt.Sprint(calls) != "[a b a b]" {
		t.Errorf("%v, calls %v", err, calls)
	}
}
