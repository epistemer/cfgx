package schema

import "github.com/epistemer/cfgx/dsl"

var Log = dsl.Group("log", "L", dsl.Bool("debug", "D"))

var App = dsl.App("app", "A", Log).Global(Log, Log) // want "Global lists group \"log\" twice"
