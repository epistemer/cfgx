package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", // want "group \"g\" has a key and a nested group that are both Retry in Values"
	dsl.Int("tries", "Tries").GoName("Retry"),
	dsl.Group("retry", "Retry", dsl.Int("max", "Max")),
)

var App = dsl.App("app", "A", G)
