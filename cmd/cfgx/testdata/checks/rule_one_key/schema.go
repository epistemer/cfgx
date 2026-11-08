package schema

import "github.com/epistemer/cfgx/dsl"

var a = dsl.String("a", "A")

var G = dsl.Group("g", "G", a).Rules(
	dsl.AtMostOne(a), // want "AtMostOne needs at least two keys"
)

var App = dsl.App("app", "A", G)
