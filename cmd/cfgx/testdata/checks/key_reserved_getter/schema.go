package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("on", "O",
	dsl.Bool("change", "C"), // want "getter OnChange of on.change is reserved"
)

var App = dsl.App("app", "A", G)
