package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D",
	dsl.String("host", "Host"),
	dsl.String("addr", "Address").Alias("database.host"), // want "of database.addr collides with another key or group"
)

var App = dsl.App("app", "A", G)
