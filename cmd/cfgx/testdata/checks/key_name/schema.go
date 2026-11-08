package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Int("Port", "P"), // want "key name \"Port\" must be lower-case words"
)

var App = dsl.App("app", "A", G)
