package schema

import "github.com/epistemer/cfgx/dsl"

var (
	G     = dsl.Group("g", "G", dsl.Int("x", "X"))
	Extra = dsl.Group("extra", "E", dsl.Int("y", "Y"))
)

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Serve").Uses(G, Extra) // want "command Serve uses group \"extra\""
