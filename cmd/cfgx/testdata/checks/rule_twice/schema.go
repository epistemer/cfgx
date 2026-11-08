package schema

import "github.com/epistemer/cfgx/dsl"

var (
	a = dsl.String("a", "A")
	b = dsl.String("b", "B")
)

var G = dsl.Group("g", "G", a, b).Rules(
	dsl.AllOrNone(a, b, a), // want "AllOrNone lists g.a twice"
)

var App = dsl.App("app", "A", G)
