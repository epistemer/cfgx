package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", "a", "b "), // want "g.mode: value \"b \" has leading or trailing spaces"
)

var App = dsl.App("app", "A", G)
