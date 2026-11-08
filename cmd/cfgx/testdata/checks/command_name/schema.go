package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("serve").Uses(G) // want "command name \"serve\" must be an exported Go identifier"
