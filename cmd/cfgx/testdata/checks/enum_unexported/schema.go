package schema

import "github.com/epistemer/cfgx/dsl"

type mode string

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", mode("a")), // want "g.mode: enum type schema.mode isn't exported"
)

var App = dsl.App("app", "A", G)
