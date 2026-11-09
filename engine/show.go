package engine

import (
	"encoding"
	"fmt"
	"net/url"
	"time"

	"github.com/epistemer/cfgx"
)

// The Show helpers generated Show closures call: they turn a value into
// what config print and explain show, never a secret.

// ShowSecret is the mask, or "" for an unset secret.
func ShowSecret(s cfgx.Secret) any {
	if s.IsSet() {
		return "******"
	}
	return ""
}

// ShowBytes sums up a File key's contents by size.
func ShowBytes(b []byte) any {
	if len(b) == 0 {
		return ""
	}
	return fmt.Sprintf("(%d bytes)", len(b))
}

// ShowURL is the URL with any password redacted.
func ShowURL(u *url.URL) any {
	if u == nil {
		return ""
	}
	return u.Redacted()
}

// ShowLocation is the zone's name.
func ShowLocation(l *time.Location) any {
	if l == nil {
		return ""
	}
	return l.String()
}

// ShowEnum is the value's text.
func ShowEnum[T comparable](v T, texts []string, values []T) any {
	for i, x := range values {
		if x == v {
			return texts[i]
		}
	}
	return fmt.Sprint(v)
}

// ShowText is the value's MarshalText text, if it has one.
func ShowText[T any](v T) any {
	if m, ok := any(&v).(encoding.TextMarshaler); ok {
		if b, err := m.MarshalText(); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}

// ShowDurations is the durations as text.
func ShowDurations(ds []time.Duration) any {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}
