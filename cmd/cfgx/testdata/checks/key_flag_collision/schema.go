package schema

import "github.com/epistemer/cfgx/dsl"

var (
	A = dsl.Group("database", "D", dsl.Int("host_x", "X"))
	B = dsl.Group("database_host", "H", dsl.Int("x", "X")) // want "of database_host.x collides with another key or group"
)

var App = dsl.App("app", "A", A, B)
