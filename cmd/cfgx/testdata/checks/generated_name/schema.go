package schema

import "github.com/epistemer/cfgx/dsl"

var DB = dsl.Group("database", "Database", dsl.Int("port", "Port"))

var App = dsl.App("app", "A", DB)

var Cmd = dsl.Command("Database").Uses(DB) // want "command type Database collides with another generated name"
