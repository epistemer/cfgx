package schema

import "github.com/epistemer/cfgx/dsl"

var (
	a = dsl.Int("a", "A").Default(1)
	b = dsl.Int("b", "B")
)

var G = dsl.Group("g", "G", a, b).Rules(
	dsl.ExactlyOne(a, b), // want "ExactlyOne can't include g.a, whose default 1 counts as set"
)

var App = dsl.App("app", "A", G)
