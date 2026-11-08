package schema

import "github.com/epistemer/cfgx/dsl"

var (
	G   = dsl.Group("g", "G", dsl.Int("x", "X"))
	Log = dsl.Group("log", "L", dsl.Bool("debug", "D"))
)

var App = dsl.App("app", "A", G, Log).Global(Log)

var Cmd = dsl.Command("Serve").Uses(G, Log) // want "uses group \"log\", which is Global"
