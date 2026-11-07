// Package fakedsl stands in for cfgx/dsl in spike S1: constructors register
// themselves, with the schema line that called them, and Export prints the
// registry as JSON.
package fakedsl

import (
	"encoding/json"
	"io"
	"runtime"
)

// Def is one registered declaration.
type Def struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Values []string `json:"values,omitempty"`
}

var registry []Def

func register(kind, name string, values []string) Def {
	_, file, line, _ := runtime.Caller(2)
	d := Def{Kind: kind, Name: name, File: file, Line: line, Values: values}
	registry = append(registry, d)
	return d
}

// Group declares a group of keys.
func Group(name string, keys ...Def) Def { return register("group", name, nil) }

// String declares a string key.
func String(name string) Def { return register("string", name, nil) }

// Enum declares an enum key whose values have a string type.
func Enum[T ~string](name string, values ...T) Def {
	vs := make([]string, len(values))
	for i, v := range values {
		vs[i] = string(v)
	}
	return register("enum", name, vs)
}

// Export writes every registered declaration to w as JSON.
func Export(w io.Writer) error { return json.NewEncoder(w).Encode(registry) }
