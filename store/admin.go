package store

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/internal/storage"
)

// Store is the admin API over a command's settings: every key it loads,
// static ones included, and writes to the mutable ones, for a settings
// dashboard or other generic code. A command that has the store returns
// it from Store(); inject it like any other dependency.
//
// Writes go through the same pipeline as a generated Update: the whole
// configuration is validated, every change is written with its history in
// one transaction, and then the values the getters return change. Any error
// means nothing changes.
type Store struct {
	a  storage.Admin
	db *bolt.DB
}

// New is for generated code: a command's Store method returns the Store of
// its loaded configuration.
func New(a storage.Admin) *Store {
	s := &Store{a: a}
	if b, ok := a.Backend().(*boltBackend); ok {
		s.db = b.db
	}
	return s
}

// KeyInfo describes a key.
type KeyInfo struct {
	Key, Description string
	Type             string // the key's kind: int, duration, enum, strings, ...
	Mutable          bool
	Default          string          // canonical text; a secret's is masked
	Enum             []string        // allowed values, if any
	Schema           json.RawMessage // JSON Schema of the value, for rendering forms
	Tags             []string
}

// Setting is a key with its current value.
type Setting struct {
	KeyInfo
	// Value is a mutable key's canonical text, which Update takes back
	// unchanged; a static key's value as config explain shows it, with
	// secrets masked and URL passwords redacted.
	Value  string
	Source string // "store", "flag --port", "env ZP_DATABASE_HOST", "file config.yaml:7:9", "default" or "unset"
	// Shadowed are the sources a stored value hides.
	Shadowed []string
	Change   *Record // the stored change that applies, if any
	Invalid  string  // why a stored value was ignored, if it was
}

// Record is a stored change.
type Record struct {
	Value     string // canonical text
	Epoch     int
	UpdatedAt time.Time
	UpdatedBy string
	Reason    string
	// Reset marks a history entry that removed the stored value, so the
	// flag, env var, config file or default applied again.
	Reset bool
}

// DB is the bbolt database: the app may keep its own buckets in it. It is
// shared; never close it, and never write inside Bucket().
func (s *Store) DB() *bolt.DB { return s.db }

// Bucket is the name of the top-level bucket that belongs to cfgx.
func (s *Store) Bucket() []byte { return slices.Clone(rootBucket) }

func keyInfo(k storage.KeyInfo) KeyInfo {
	ki := KeyInfo{Key: k.Path, Description: k.Desc, Type: k.Kind, Mutable: k.Mutable, Default: k.Default, Enum: k.Enum, Tags: k.Tags}
	if k.Schema != "" {
		ki.Schema = json.RawMessage(k.Schema)
	}
	return ki
}

func record(r storage.Record) Record {
	return Record{Value: r.Raw, Epoch: r.Epoch, UpdatedAt: r.UpdatedAt, UpdatedBy: r.UpdatedBy, Reason: r.Reason, Reset: r.Reset}
}

// Keys describes every key of the command.
func (s *Store) Keys() []KeyInfo {
	ks := s.a.AdminKeys()
	out := make([]KeyInfo, len(ks))
	for i, k := range ks {
		out[i] = keyInfo(k)
	}
	return out
}

// List is every key of the command with its current value.
func (s *Store) List() []Setting {
	ss := s.a.AdminSettings()
	out := make([]Setting, len(ss))
	for i, st := range ss {
		out[i] = Setting{KeyInfo: keyInfo(st.KeyInfo), Value: st.Value, Source: st.Source,
			Shadowed: st.Shadowed, Invalid: st.Invalid}
		if st.Record != nil {
			r := record(*st.Record)
			out[i].Change = &r
		}
	}
	return out
}

// Get is one key with its current value.
func (s *Store) Get(key string) (Setting, error) {
	for _, st := range s.List() {
		if st.Key == key {
			return st, nil
		}
	}
	return Setting{}, &unknownKey{key}
}

type unknownKey struct{ key string }

func (e *unknownKey) Error() string { return e.key + " isn't a key of this command" }

// Update sets mutable keys to values given as canonical text ("5m",
// "parallel"), parsed like any other source, all together or not at all.
func (s *Store) Update(ctx context.Context, values map[string]string, opts ...cfgx.WriteOption) error {
	if len(values) == 0 {
		return nil
	}
	return s.a.WriteText(ctx, values, nil, opts...)
}

// Reset removes the stored values of keys, so their flag, env var, config
// file or default values apply again, all together or not at all.
func (s *Store) Reset(ctx context.Context, keys []string, opts ...cfgx.WriteOption) error {
	if len(keys) == 0 {
		return nil
	}
	return s.a.WriteText(ctx, nil, keys, opts...)
}

// History returns up to limit of a key's stored changes, newest first;
// a limit of 0 or less means all that are kept (the last 1000).
func (s *Store) History(key string, limit int) ([]Record, error) {
	h, err := s.a.History(key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(h))
	for i, r := range h {
		out[i] = record(r)
	}
	return out, nil
}
