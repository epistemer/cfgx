package schema

import "github.com/epistemer/cfgx/dsl"

var (
	a = dsl.Bool("a", "A")
	b = dsl.Bool("b", "B")
)

var G = dsl.Group("g", "G", a, b).Rules(
	dsl.RelevantIf(a, b), // want "RelevantIf rules on g.a form a cycle"
	dsl.RelevantIf(b, a),
)

var App = dsl.App("app", "A", G)
