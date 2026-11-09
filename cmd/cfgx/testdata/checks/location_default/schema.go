package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Location("zone", "Zone").Default("Cairo"), // want "default of g.zone: \"Cairo\" isn't a time zone this system knows"
)

var App = dsl.App("app", "A", G)
