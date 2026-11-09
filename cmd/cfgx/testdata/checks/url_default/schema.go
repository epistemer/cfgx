package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.URL("db", "DB").Default("localhost:5432"), // want "default of g.db: isn't an absolute URL: localhost: isn't followed by // or /"
)

var App = dsl.App("app", "A", G)
