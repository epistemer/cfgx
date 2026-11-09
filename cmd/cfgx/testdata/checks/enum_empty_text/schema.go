package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", "a", ""), // want "g.mode: value 2 has empty text, which means not set"
)

var App = dsl.App("app", "A", G)
