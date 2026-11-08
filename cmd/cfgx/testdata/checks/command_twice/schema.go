package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G)

var (
	A = dsl.Command("Serve").Uses(G)
	B = dsl.Command("Serve").Uses(G) // want "command Serve is declared twice"
)
