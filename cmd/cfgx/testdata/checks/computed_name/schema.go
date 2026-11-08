package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D", dsl.Int("x", "X")).Computed("dsn") // want "computed method \"dsn\" of group \"database\" must be an exported"

var App = dsl.App("app", "A", G)
