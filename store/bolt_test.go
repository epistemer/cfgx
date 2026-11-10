package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/epistemer/cfgx/internal/storage"
)

func open(t *testing.T, path string, o storage.Options) storage.Backend {
	t.Helper()
	b, err := Open(path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestOpenApplyHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zp.db")
	b := open(t, path, storage.Options{Timeout: time.Second})
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("mode %v, %v; want 0600", fi.Mode().Perm(), err)
		}
	}
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	set := func(key, v string) storage.Change {
		r := storage.Record{Raw: v, UpdatedAt: now, UpdatedBy: "admin"}
		return storage.Change{Key: key, Record: &r, History: r}
	}
	// Keys that are prefixes of each other keep separate histories.
	if err := b.Apply([]storage.Change{set("auth.otp", "a"), set("auth.otp.length", "6")}); err != nil {
		t.Fatal(err)
	}
	if err := b.Apply([]storage.Change{set("auth.otp.length", "8")}); err != nil {
		t.Fatal(err)
	}
	if err := b.Apply([]storage.Change{{Key: "auth.otp", History: storage.Record{UpdatedAt: now, Reset: true}}}); err != nil {
		t.Fatal(err)
	}
	recs, bad, err := b.Records()
	if err != nil || len(bad) != 0 || len(recs) != 1 || recs["auth.otp.length"].Raw != "8" {
		t.Fatalf("records = %v, %v, %v", recs, bad, err)
	}
	h, err := b.History("auth.otp.length", 0)
	if err != nil || len(h) != 2 || h[0].Raw != "8" || h[1].Raw != "6" {
		t.Errorf("history = %v, %v; want newest first", h, err)
	}
	if h, _ := b.History("auth.otp", 1); len(h) != 1 || !h[0].Reset {
		t.Errorf("reset history = %v", h)
	}
	// A value over 64 KiB is refused, and nothing in the batch is written.
	big := string(make([]byte, storage.MaxValue+1))
	if err := b.Apply([]storage.Change{set("a", "1"), set("b", big)}); err == nil {
		t.Error("a 64KiB+1 value was stored")
	}
	if recs, _, _ := b.Records(); len(recs) != 1 {
		t.Errorf("a failed batch wrote %v", recs)
	}
}

func TestHistoryIsCapped(t *testing.T) {
	b := open(t, filepath.Join(t.TempDir(), "zp.db"), storage.Options{Timeout: time.Second})
	var batch []storage.Change
	for i := 0; i < storage.MaxHistory+5; i++ {
		r := storage.Record{Raw: "v", UpdatedAt: time.Now()}
		batch = append(batch, storage.Change{Key: "k", Record: &r, History: r})
	}
	// One change per transaction, as writes come.
	for _, c := range batch {
		if err := b.Apply([]storage.Change{c}); err != nil {
			t.Fatal(err)
		}
	}
	if h, _ := b.History("k", 0); len(h) != storage.MaxHistory {
		t.Errorf("history has %d entries, want %d", len(h), storage.MaxHistory)
	}
}

// Trimming several entries at once removes exactly the oldest ones.
func TestTrimSeveral(t *testing.T) {
	b := open(t, filepath.Join(t.TempDir(), "zp.db"), storage.Options{Timeout: time.Second})
	db := b.(*boltBackend).DB()
	err := db.Update(func(tx *bolt.Tx) error {
		h, err := tx.Bucket(rootBucket).Bucket(historyBucket).CreateBucketIfNotExists([]byte("k"))
		if err != nil {
			return err
		}
		var last uint64
		for i := 0; i < 10; i++ {
			if last, err = h.NextSequence(); err != nil {
				return err
			}
			if err := h.Put(seqKey(last), []byte(`{"value":"`+strconv.Itoa(int(last))+`"}`)); err != nil {
				return err
			}
		}
		return trim(h, last, 3)
	})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := b.History("k", 0)
	var got []string
	for _, r := range h {
		got = append(got, r.Raw)
	}
	if strings.Join(got, ",") != "10,9,8" {
		t.Errorf("kept %v, want the newest three: 10,9,8", got)
	}
}

func TestLockedMissingAndBad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zp.db")
	open(t, path, storage.Options{Timeout: time.Second})
	// The file is held: a second open waits for the timeout, then says so.
	if _, err := Open(path, storage.Options{Timeout: 100 * time.Millisecond, ReadOnly: true}); !errors.Is(err, storage.ErrLocked) {
		t.Errorf("locked file: %v", err)
	}
	// A read-only open never creates the file.
	missing := filepath.Join(dir, "none.db")
	if _, err := Open(missing, storage.Options{ReadOnly: true, Timeout: time.Second}); !errors.Is(err, storage.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("a read-only open created the file")
	}
}

func TestBadRecordAndNewerFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zp.db")
	b := open(t, path, storage.Options{Timeout: time.Second})
	// Something else wrote junk in the reserved bucket: it's left out.
	db := b.(*boltBackend).DB()
	if err := db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(rootBucket).Bucket(settingsBucket).Put([]byte("k"), []byte("not json"))
	}); err != nil {
		t.Fatal(err)
	}
	recs, bad, err := b.Records()
	if err != nil || len(recs) != 0 || bad["k"] == nil {
		t.Errorf("records %v, bad %v, %v", recs, bad, err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(rootBucket).Bucket(metaBucket).Put(formatKey, []byte("99"))
	}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if _, err := Open(path, storage.Options{Timeout: time.Second}); err == nil {
		t.Error("a newer format opened")
	}
}
