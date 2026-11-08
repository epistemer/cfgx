package schema

import "github.com/epistemer/cfgx/dsl"

var (
	a     = dsl.String("a", "A")
	stray = dsl.String("stray", "S")
)

var G = dsl.Group("g", "G", a).Rules(
	dsl.Requires(a, stray), // want "Requires lists key \"stray\", which isn't in a group passed to dsl.App"
)

var App = dsl.App("app", "A", G)
