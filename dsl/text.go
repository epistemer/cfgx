package dsl

import (
	"encoding"
	"fmt"
	"go/token"
	"reflect"
	"strings"

	"github.com/epistemer/cfgx/internal/ir"
)

// TextKey is a setting of any type that parses itself from text. Create it
// with Text.
type TextKey[T any] struct {
	key[*TextKey[T], T]
}

var textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()

// Text declares a key whose getter returns T, a named type whose pointer
// implements encoding.TextUnmarshaler, such as netip.Prefix. Files, env vars
// and flags give its text, which UnmarshalText parses; empty text means
// T's zero value, which counts as not set. zero only names the type: pass
// T's zero value, and set a default with Default, which needs T (or *T) to
// implement encoding.TextMarshaler too. T must be exported from a package
// the generated code can import.
//
//	dsl.Text("allow", "Allowed client range", netip.Prefix{}).
//		Default(netip.MustParsePrefix("10.0.0.0/8"))
func Text[T any](name, desc string, zero T) *TextKey[T] {
	k := &TextKey[T]{}
	k.init(k, name, desc, ir.KindText, nil, caller(1))
	t := reflect.TypeFor[T]()
	k.typ = &ir.Type{Name: t.Name(), Kind: t.Kind().String()}
	if t.PkgPath() != "" {
		k.typ.Path = t.PkgPath()
		k.typ.Pkg, _, _ = strings.Cut(t.String(), ".")
	}
	problem := func(format string, args ...any) {
		k.problems = append(k.problems, fmt.Sprintf(format, args...))
	}
	switch {
	case t.Name() == "" || t.PkgPath() == "":
		problem("text type %s must be a named type from a package, such as netip.Prefix", t)
	case strings.Contains(t.Name(), "["):
		problem("text type %s is a generic type; use a non-generic named type", t)
	case !token.IsExported(t.Name()):
		problem("text type %s isn't exported, so the generated code can't name it", t)
	case !reflect.PointerTo(t).Implements(textUnmarshaler):
		problem("text type %s doesn't implement encoding.TextUnmarshaler (on *%s), so cfgx can't parse it", t, t.Name())
	}
	if !reflect.ValueOf(&zero).Elem().IsZero() {
		problem("Text's third argument only names the type; pass %s's zero value, and set a default with Default", t)
	}
	return k
}

// Default sets the key's default, recorded as its MarshalText text, which
// must parse back with UnmarshalText to the same value.
func (k *TextKey[T]) Default(v T) *TextKey[T] {
	rv := reflect.ValueOf(&v).Elem()
	if !rv.Addr().Type().Implements(textMarshaler) {
		k.problems = append(k.problems, fmt.Sprintf("a default needs %s to implement encoding.TextMarshaler", rv.Type()))
		return k
	}
	b, err := rv.Addr().Interface().(encoding.TextMarshaler).MarshalText()
	if err != nil {
		k.problems = append(k.problems, fmt.Sprintf("default: MarshalText failed: %v", err))
		return k
	}
	text := string(b)
	if u, ok := any(new(T)).(encoding.TextUnmarshaler); ok {
		switch err := u.UnmarshalText(b); {
		case err != nil:
			k.problems = append(k.problems, fmt.Sprintf("default %q doesn't parse back: %v", text, err))
		case !reflect.DeepEqual(*any(u).(*T), v):
			k.problems = append(k.problems, fmt.Sprintf("default %q parses back to a different value", text))
		}
	}
	k.def = &text
	return k
}
