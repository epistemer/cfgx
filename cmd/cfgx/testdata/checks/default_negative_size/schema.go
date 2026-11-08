package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.ByteSize("max", "Max").Default(-1), // want "default of g.max: \"-1\" is not a size"
)

var App = dsl.App("app", "A", G)
