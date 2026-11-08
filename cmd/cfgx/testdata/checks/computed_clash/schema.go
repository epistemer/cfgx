package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D", dsl.Int("x", "X")).Computed("DatabaseX") // want "interface has two methods named DatabaseX"

var App = dsl.App("app", "A", G)
