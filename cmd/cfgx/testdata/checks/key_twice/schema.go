package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Int("port", "P"),
	dsl.Int("port", "P"), // want "key \"port\" is declared twice in group \"server\""
)

var App = dsl.App("app", "A", G)
