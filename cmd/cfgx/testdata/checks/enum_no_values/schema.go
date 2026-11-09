package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum[string]("mode", "Mode"), // want "g.mode: an enum needs at least one value"
)

var App = dsl.App("app", "A", G)
