package dsl

import (
	"encoding"
	"fmt"
	"go/token"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/epistemer/cfgx/internal/ir"
)

// EnumKey is a setting that takes one of a fixed set of values of type T.
// Create it with Enum.
type EnumKey[T comparable] struct {
	key[*EnumKey[T], T]
	values []T
}

// Enum declares a key whose value is one of values; its getter returns T,
// which Go infers from the values. Files, env vars and flags name a value by
// its text: MarshalText if T has it, else the string itself for a string
// type, else String if T has it, else the number or bool in Go syntax.
// T's underlying type must be a string, integer, float or bool type, and a
// named T must be exported from a package the generated code can import.
//
//	dsl.Enum("execmode", "Task execution mode",
//		models.ZpTaskExecutionModeSequential,
//		models.ZpTaskExecutionModeParallel,
//	).Default(models.ZpTaskExecutionModeSequential)
func Enum[T comparable](name, desc string, values ...T) *EnumKey[T] {
	k := &EnumKey[T]{values: values}
	k.init(k, name, desc, ir.KindEnum, nil, caller(1))
	k.text = func(v T) string { s, _ := enumText(reflect.ValueOf(&v).Elem()); return s }
	k.describe()
	return k
}

// Default sets the key's default, which must be one of the values.
func (k *EnumKey[T]) Default(v T) *EnumKey[T] {
	text, err := enumText(reflect.ValueOf(&v).Elem())
	if err != nil {
		k.problems = append(k.problems, fmt.Sprintf("default: %v", err))
		return k
	}
	if !slices.Contains(k.values, v) {
		k.problems = append(k.problems, fmt.Sprintf("default %q isn't one of the values", text))
	}
	k.def = &text
	return k
}

// describe records T and the values in the IR, and every problem with them.
func (k *EnumKey[T]) describe() {
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
	case !enumKinds[t.Kind()]:
		problem("enum type %s must have a string, integer, float or bool underlying type", t)
		return
	case strings.Contains(t.Name(), "["):
		problem("enum type %s is a generic type; use a non-generic named type", t)
	case t.PkgPath() != "" && !token.IsExported(t.Name()):
		problem("enum type %s isn't exported, so the generated code can't name it", t)
	}
	if len(k.values) == 0 {
		problem("an enum needs at least one value")
		return
	}
	texts := map[string]int{}
	for i, v := range k.values {
		rv := reflect.ValueOf(&v).Elem()
		text, err := enumText(rv)
		if err != nil {
			problem("value %d: %v", i+1, err)
			continue
		}
		lit, err := enumLit(rv)
		if err != nil {
			problem("value %q: %v", text, err)
			continue
		}
		switch {
		case slices.Index(k.values, v) < i:
			problem("value %q is listed twice", text)
			continue
		case text == "":
			problem("value %d has empty text, which means not set", i+1)
			continue
		case strings.TrimSpace(text) != text:
			problem("value %q has leading or trailing spaces, so no input can match it", text)
			continue
		}
		if j, dup := texts[text]; dup {
			problem("values %d and %d have the same text %q", j+1, i+1, text)
			continue
		}
		texts[text] = i
		k.enum = append(k.enum, ir.EnumValue{Text: text, Lit: lit, Zero: rv.IsZero()})
	}
}

var enumKinds = map[reflect.Kind]bool{
	reflect.String: true, reflect.Bool: true,
	reflect.Int: true, reflect.Int8: true, reflect.Int16: true, reflect.Int32: true, reflect.Int64: true,
	reflect.Uint: true, reflect.Uint8: true, reflect.Uint16: true, reflect.Uint32: true, reflect.Uint64: true,
	reflect.Float32: true, reflect.Float64: true,
}

var (
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
	stringer      = reflect.TypeFor[fmt.Stringer]()
)

// enumText returns the text of v, which must be addressable so pointer
// methods count.
func enumText(v reflect.Value) (string, error) {
	switch {
	case v.Addr().Type().Implements(textMarshaler):
		b, err := v.Addr().Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return "", fmt.Errorf("MarshalText failed: %v", err)
		}
		return string(b), nil
	case v.Kind() == reflect.String:
		return v.String(), nil
	case v.Addr().Type().Implements(stringer):
		return v.Addr().Interface().(fmt.Stringer).String(), nil
	}
	return enumLit(v)
}

// enumLit returns a Go literal of v's underlying value.
func enumLit(v reflect.Value) (string, error) {
	switch v.Kind() {
	case reflect.String:
		return strconv.Quote(v.String()), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return "", fmt.Errorf("%v isn't a finite number", f)
		}
		return strconv.FormatFloat(f, 'g', -1, v.Type().Bits()), nil
	}
	return "", fmt.Errorf("%s has no literal form", v.Type())
}
