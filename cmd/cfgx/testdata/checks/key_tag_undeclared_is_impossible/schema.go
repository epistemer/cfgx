package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Int("port", "P").Deprecated(" "), // want "deprecated message of server.port must not be blank"
)

var App = dsl.App("app", "A", G)
