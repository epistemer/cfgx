package schema

import "github.com/epistemer/cfgx/dsl"

var missing *dsl.StringKey

var G = dsl.Group("server", "S", missing) // want "group \"server\" lists a nil key or group"

var App = dsl.App("app", "A", G)
