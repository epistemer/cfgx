package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", "a", "b", "a"), // want "g.mode: value \"a\" is listed twice"
)

var App = dsl.App("app", "A", G)
