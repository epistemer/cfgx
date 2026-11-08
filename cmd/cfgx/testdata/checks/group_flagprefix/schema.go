package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S", dsl.Int("x", "X")).FlagPrefix("Srv") // want "FlagPrefix \"Srv\" of group \"server\""

var App = dsl.App("app", "A", G)
