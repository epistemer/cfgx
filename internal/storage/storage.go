package storage

import (
	"errors"
	"time"
)

// Record is a stored value with who changed it, when and why (interface
// §7.3). Raw is the value's canonical text, parsed by the key's generated
// parser like any other source.
type Record struct {
	Raw       string    `json:"value"`
	Epoch     int       `json:"epoch,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	// Reset marks a history entry that removed the stored value.
	Reset bool `json:"reset,omitempty"`
}

// Change is one key's part of a write: its new record, or nil to remove
// the stored value. Either way, History is appended to the key's history.
type Change struct {
	Key     string
	Record  *Record
	History Record
}

// Backend is a settings store: current records by key, and their history.
type Backend interface {
	// Records returns the current record of each key that has one. A
	// record that fails to decode is left out and reported in bad, by key.
	Records() (records map[string]Record, bad map[string]error, err error)
	// Apply writes the changes in one transaction: every history entry
	// and every record, or none.
	Apply(changes []Change) error
	// History returns up to limit of a key's past records, newest first;
	// a limit of 0 or less means all of them.
	History(key string, limit int) ([]Record, error)
	// Close releases the store.
	Close() error
}

// Options are how a backend is opened.
type Options struct {
	// ReadOnly opens an existing store without the right to write; a
	// missing store is ErrNotExist instead of being created.
	ReadOnly bool
	// Timeout is how long to wait for another process to let go of the
	// store; then ErrLocked.
	Timeout time.Duration
}

// Opener opens the store at path. Generated code passes the bbolt one, so
// only programs whose schema declares a store link bbolt.
type Opener func(path string, o Options) (Backend, error)

var (
	// ErrLocked is returned when another process holds the store.
	ErrLocked = errors.New("store is held by a running process; change settings through the application")
	// ErrNotExist is returned by a read-only open of a missing store.
	ErrNotExist = errors.New("store doesn't exist")
)

// MaxValue is the most a stored value may hold, in bytes.
const MaxValue = 64 << 10

// MaxHistory is how many history entries each key keeps.
const MaxHistory = 1000
