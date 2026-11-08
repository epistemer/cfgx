package schema

import "github.com/epistemer/cfgx/dsl"

var (
	A = dsl.Group("server", "S", dsl.Int("x", "X"))
	B = dsl.Group("server", "S", dsl.Int("y", "Y")) // want "group \"server\" is declared twice"
)

var App = dsl.App("app", "A", A, B)
