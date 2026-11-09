package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", "a", "b").Default("c"), // want "g.mode: default \"c\" isn't one of the values"
)

var App = dsl.App("app", "A", G)
