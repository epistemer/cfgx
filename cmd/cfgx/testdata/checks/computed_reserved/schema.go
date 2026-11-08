package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D", dsl.Int("x", "X")).Computed("Close") // want "computed method Close of group \"database\" is reserved"

var App = dsl.App("app", "A", G)
