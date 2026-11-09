package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Enum("n", "N", 0, 1).Required(), // want "g.n is Required, but its value \"0\" is the zero value of int, which counts as not set"
)

var App = dsl.App("app", "A", G)
