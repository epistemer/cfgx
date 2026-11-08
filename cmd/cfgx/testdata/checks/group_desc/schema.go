package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "", dsl.Int("x", "X")) // want "group \"server\" needs a description"

var App = dsl.App("app", "A", G)
