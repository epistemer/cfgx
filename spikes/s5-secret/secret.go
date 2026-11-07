// Package secret prototypes cfgx.Secret (spike S5, roadmap item M1.5).
//
// Secret stores its underlying string inside a closure so that formatting,
// serialization, logging, and pure-Go reflection cannot accidentally read or
// print the raw value.
package secret

import (
	"crypto/subtle"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
)

const (
	mask     = "******"
	jsonMask = `"******"`
)

var (
	_ fmt.Stringer           = Secret{}
	_ fmt.GoStringer         = Secret{}
	_ fmt.Formatter          = Secret{}
	_ json.Marshaler         = Secret{}
	_ encoding.TextMarshaler = Secret{}
	_ slog.LogValuer         = Secret{}
)

// Secret holds a sensitive configuration string hidden inside an unexported
// closure field. Because Secret contains a func field, it is not comparable
// with == (use Equal instead) and cannot be used as a map key.
type Secret struct {
	reveal func() string
}

// New returns a Secret wrapping s. An empty string produces an unset Secret
// (zero value), matching cfgx's value-based "set" semantics (interface.md §3.5).
func New(s string) Secret {
	if s == "" {
		return Secret{}
	}
	return Secret{
		reveal: func() string { return s },
	}
}

// Reveal returns the cleartext secret string, or "" if the Secret is unset.
func (s Secret) Reveal() string {
	if s.reveal == nil {
		return ""
	}
	return s.reveal()
}

// Bytes returns a fresh byte-slice copy of the cleartext secret, or nil if
// the Secret is unset. Mutating the returned slice does not affect the Secret.
func (s Secret) Bytes() []byte {
	if s.reveal == nil {
		return nil
	}
	v := s.reveal()
	if v == "" {
		return nil
	}
	return append([]byte(nil), v...)
}

// IsSet reports whether the Secret holds a non-empty value.
func (s Secret) IsSet() bool {
	return s.reveal != nil && s.reveal() != ""
}

// Equal reports whether the Secret is set and matches v in constant time.
// An unset Secret always returns false, even when compared against "".
func (s Secret) Equal(v string) bool {
	if !s.IsSet() {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(s.reveal()), []byte(v)) == 1
}

// String returns the fixed mask "******".
func (s Secret) String() string {
	return mask
}

// GoString returns the fixed mask "******".
func (s Secret) GoString() string {
	return mask
}

// Format writes the fixed mask "******" to f for every fmt verb.
func (s Secret) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, mask)
}

// MarshalJSON returns the JSON string "\"******\"".
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(jsonMask), nil
}

// MarshalText returns the fixed mask "******" as bytes.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(mask), nil
}

// MarshalYAML returns the fixed mask "******" for YAML libraries that check
// the MarshalYAML() (any, error) interface.
func (s Secret) MarshalYAML() (any, error) {
	return mask, nil
}

// LogValue returns a slog.Value containing the fixed mask "******".
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(mask)
}
