package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Int("port", ""), // want "key server.port needs a description"
)

var App = dsl.App("app", "A", G)
