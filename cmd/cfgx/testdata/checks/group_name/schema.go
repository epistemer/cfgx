package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("Server", "S", dsl.Int("x", "X")) // want "group name \"Server\" must start with a lower-case letter"

var App = dsl.App("app", "A", G)
