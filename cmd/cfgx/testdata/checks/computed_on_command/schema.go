package schema

import "github.com/epistemer/cfgx/dsl"

var (
	A = dsl.Group("a", "A", dsl.Int("x", "X")).Computed("Ping")
	B = dsl.Group("b", "B", dsl.Int("y", "Y")).Computed("Ping")
)

var App = dsl.App("app", "A", A, B)

var Cmd = dsl.Command("Serve").Uses(A, B) // want "command Serve gets method Ping from both a and group \"b\""
