package engine

import (
	"encoding"
	"fmt"
	"reflect"
)

// ParseText parses raw with T's UnmarshalText; empty means T's zero value.
// The text is passed exactly, spaces included.
func ParseText[T any, PT interface {
	*T
	encoding.TextUnmarshaler
}](raw string, dst PT) error {
	if raw == "" {
		var zero T
		*dst = zero
		return nil
	}
	var v T
	if err := PT(&v).UnmarshalText([]byte(raw)); err != nil {
		return fmt.Errorf("%q isn't valid: %v", raw, err)
	}
	*dst = v
	return nil
}

// NonZero reports whether v isn't its type's zero value: whether a Text
// key is set (A4).
func NonZero[T any](v T) bool {
	return !reflect.ValueOf(&v).Elem().IsZero()
}
