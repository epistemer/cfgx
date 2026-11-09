package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", "a", "b", "a", "a"), // want "g.mode: value \"a\" is listed more than once"
)

var App = dsl.App("app", "A", G)
