package store

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	bolt "go.etcd.io/bbolt"
	bolterrors "go.etcd.io/bbolt/errors"

	"github.com/epistemer/cfgx/internal/storage"
)

// The on-disk layout (implementation.md §3.8): one top-level bucket, with
// the current records, one history bucket per key, and metadata.
var (
	rootBucket     = []byte("cfgx")
	settingsBucket = []byte("settings")
	historyBucket  = []byte("history")
	metaBucket     = []byte("meta")
	formatKey      = []byte("format")
)

// formatVersion is the layout's version, kept in meta.
const formatVersion = 1

// Open is the storage.Opener for bbolt files. Generated code passes it to
// the engine when the schema declares App.Store.
func Open(path string, o storage.Options) (storage.Backend, error) {
	if o.ReadOnly {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotExist
		}
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: o.Timeout, ReadOnly: o.ReadOnly})
	if errors.Is(err, bolterrors.ErrTimeout) {
		return nil, storage.ErrLocked
	}
	if err != nil {
		return nil, fmt.Errorf("opening the store %s: %w", path, err)
	}
	b := &boltBackend{db: db}
	if !o.ReadOnly {
		if err := db.Update(b.init); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting up the store %s: %w", path, err)
		}
	} else if err := db.View(b.checkFormat); err != nil {
		db.Close()
		return nil, err
	}
	return b, nil
}

type boltBackend struct{ db *bolt.DB }

// init creates the buckets and records the format, or checks it.
func (b *boltBackend) init(tx *bolt.Tx) error {
	root, err := tx.CreateBucketIfNotExists(rootBucket)
	if err != nil {
		return err
	}
	for _, name := range [][]byte{settingsBucket, historyBucket, metaBucket} {
		if _, err := root.CreateBucketIfNotExists(name); err != nil {
			return err
		}
	}
	meta := root.Bucket(metaBucket)
	if meta.Get(formatKey) == nil {
		return meta.Put(formatKey, []byte(strconv.Itoa(formatVersion)))
	}
	return b.checkFormat(tx)
}

func (b *boltBackend) checkFormat(tx *bolt.Tx) error {
	root := tx.Bucket(rootBucket)
	if root == nil {
		return nil // nothing stored yet
	}
	meta := root.Bucket(metaBucket)
	if meta == nil {
		return nil
	}
	if v := meta.Get(formatKey); v != nil {
		n, err := strconv.Atoi(string(v))
		if err != nil || n > formatVersion {
			return fmt.Errorf("the store's format %q is newer than this cfgx reads (%d); upgrade the program", v, formatVersion)
		}
	}
	return nil
}

func (b *boltBackend) Records() (map[string]storage.Record, map[string]error, error) {
	recs, bad := map[string]storage.Record{}, map[string]error{}
	err := b.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootBucket)
		if root == nil {
			return nil
		}
		s := root.Bucket(settingsBucket)
		if s == nil {
			return nil
		}
		return s.ForEach(func(k, v []byte) error {
			var r storage.Record
			if err := json.Unmarshal(v, &r); err != nil {
				bad[string(k)] = fmt.Errorf("can't decode the stored record: %v", err)
				return nil
			}
			recs[string(k)] = r
			return nil
		})
	})
	return recs, bad, err
}

func (b *boltBackend) Apply(changes []storage.Change) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootBucket)
		if root == nil {
			return errors.New("the store isn't set up (a cfgx bug)")
		}
		settings, history := root.Bucket(settingsBucket), root.Bucket(historyBucket)
		for _, c := range changes {
			if c.Record != nil && len(c.Record.Raw) > storage.MaxValue {
				return fmt.Errorf("%s: the value is larger than 64KiB", c.Key)
			}
			h, err := history.CreateBucketIfNotExists([]byte(c.Key))
			if err != nil {
				return err
			}
			seq, err := h.NextSequence()
			if err != nil {
				return err
			}
			entry, err := json.Marshal(c.History)
			if err != nil {
				return err
			}
			if err := h.Put(seqKey(seq), entry); err != nil {
				return err
			}
			if err := trim(h, storage.MaxHistory); err != nil {
				return err
			}
			if c.Record == nil {
				if err := settings.Delete([]byte(c.Key)); err != nil {
					return err
				}
				continue
			}
			rec, err := json.Marshal(c.Record)
			if err != nil {
				return err
			}
			if err := settings.Put([]byte(c.Key), rec); err != nil {
				return err
			}
		}
		return nil
	})
}

// seqKey is a history entry's key: the sequence, big-endian, so entries
// sort in order.
func seqKey(n uint64) []byte {
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, n)
	return k
}

// trim deletes the oldest entries of h beyond max. It counts with a cursor:
// Bucket.Stats doesn't see writes in the open transaction.
func trim(h *bolt.Bucket, max int) error {
	c := h.Cursor()
	n := 0
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		n++
	}
	if n <= max {
		return nil
	}
	for k, _ := c.First(); k != nil && n > max; k, _ = c.Next() {
		if err := c.Delete(); err != nil {
			return err
		}
		n--
	}
	return nil
}

func (b *boltBackend) History(key string, limit int) ([]storage.Record, error) {
	var out []storage.Record
	err := b.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootBucket)
		if root == nil {
			return nil
		}
		hs := root.Bucket(historyBucket)
		if hs == nil {
			return nil
		}
		h := hs.Bucket([]byte(key))
		if h == nil {
			return nil
		}
		c := h.Cursor()
		for k, v := c.Last(); k != nil && (limit <= 0 || len(out) < limit); k, v = c.Prev() {
			var r storage.Record
			if err := json.Unmarshal(v, &r); err != nil {
				continue // an entry that doesn't decode isn't history
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

func (b *boltBackend) Close() error { return b.db.Close() }

// DB returns the bbolt handle, for Store.DB.
func (b *boltBackend) DB() *bolt.DB { return b.db }
