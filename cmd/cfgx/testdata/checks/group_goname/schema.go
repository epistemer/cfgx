package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("database", "D", dsl.Int("x", "X")).GoName("db") // want "GoName \"db\" of group \"database\""

var App = dsl.App("app", "A", G)
