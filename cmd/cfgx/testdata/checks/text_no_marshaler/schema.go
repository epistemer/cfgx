package schema

import "github.com/epistemer/cfgx/dsl"

// Mode parses text but can't write it, so it can't have a default. (The
// DSL's problem is reported before the generator would reject the schema
// package's type.)
type Mode int

func (m *Mode) UnmarshalText(b []byte) error { return nil }

var G = dsl.Group("g", "G",
	dsl.Text("mode", "Mode", Mode(0)).Default(Mode(1)), // want "g.mode: a default needs schema.Mode to implement encoding.TextMarshaler"
)

var App = dsl.App("app", "A", G)
