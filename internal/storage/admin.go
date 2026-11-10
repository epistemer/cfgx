package storage

import (
	"context"

	"github.com/epistemer/cfgx"
)

// Admin is the engine's Writer without its type parameter: what
// cfgx/store's admin API is built on. Its methods cover the keys of one
// command.
type Admin interface {
	// AdminKeys describes every key of the command, in table order.
	AdminKeys() []KeyInfo
	// AdminSettings is every key of the command with its current value
	// and where it came from, in table order.
	AdminSettings() []Setting
	// WriteText sets keys to text values and resets others, in one write
	// (engine.Writer.Write). Keys are paths.
	WriteText(ctx context.Context, set map[string]string, reset []string, options ...cfgx.WriteOption) error
	// History returns up to limit of a key's past records, newest first;
	// 0 or less means all.
	History(key string, limit int) ([]Record, error)
	// Backend is the open store, or nil once it is closed.
	Backend() Backend
}

// KeyInfo describes a key.
type KeyInfo struct {
	Path, Desc, Kind string
	Mutable          bool
	Default          string // canonical text; a secret's is masked
	Enum             []string
	Schema           string // JSON Schema of the value
	Tags             []string
}

// Setting is a key's current value and where it came from.
type Setting struct {
	KeyInfo
	// Value is a mutable key's canonical text, which WriteText takes
	// back; a static key's value as config explain shows it, with secrets
	// masked and URL passwords redacted.
	Value  string
	Source string // "store", "flag --port", "env ZP_DATABASE_HOST", "file config.yaml:7:9", "default" or "unset"
	// Shadowed are the sources a stored value hides.
	Shadowed []string
	Record   *Record // the stored record, if one applies
	Invalid  string  // why a stored value was ignored, if it was
}
