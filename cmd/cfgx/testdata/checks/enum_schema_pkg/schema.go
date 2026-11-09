package schema

import "github.com/epistemer/cfgx/dsl"

type Mode string

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", Mode("a")), // want "enum type schema.Mode of g.mode is declared in the schema package, which the generated code can't import"
)

var App = dsl.App("app", "A", G)
