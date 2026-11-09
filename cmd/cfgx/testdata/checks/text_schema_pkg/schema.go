package schema

import "github.com/epistemer/cfgx/dsl"

type Mode int

func (m *Mode) UnmarshalText(b []byte) error { return nil }

var G = dsl.Group("g", "G",
	dsl.Text("mode", "Mode", Mode(0)), // want "text type schema.Mode of g.mode is declared in the schema package"
)

var App = dsl.App("app", "A", G)
