package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D",
	dsl.String("sslmode", "SSL").Alias("ssl_mode"), // want "alias \"ssl_mode\" must be a full dotted path"
)

var App = dsl.App("app", "A", G)
