package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D", dsl.Int("x", "X")).Computed("DSN", "DSN") // want "lists computed method DSN twice"

var App = dsl.App("app", "A", G)
