package schema

import "github.com/epistemer/cfgx/dsl"

var DB = dsl.Group("database", "D", dsl.String("host", "H")).GoName("DB")

var App = dsl.App("app", "A", DB)

var Cmd = dsl.Command("Migrate").Uses(DB).Args(
	dsl.String("database.host", "H"), // want "flag --database-host of database.host collides with another flag on Migrate"
)
