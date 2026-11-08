package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("1app", "A", G) // want "the env prefix derived from the app name \"1app\""
