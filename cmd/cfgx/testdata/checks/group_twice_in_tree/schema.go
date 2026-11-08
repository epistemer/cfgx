package schema

import "github.com/epistemer/cfgx/dsl"

var Shared = dsl.Group("shared", "S", dsl.Int("x", "X")) // want "group \"shared\" is used twice, as a.shared and as b.shared"

var (
	A = dsl.Group("a", "A", Shared)
	B = dsl.Group("b", "B", Shared)
)

var App = dsl.App("app", "A", A, B)
