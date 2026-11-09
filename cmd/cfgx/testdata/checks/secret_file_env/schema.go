package schema

import "github.com/epistemer/cfgx/dsl"

// A Secret also claims <ENV>_FILE.
var G = dsl.Group("g", "G",
	dsl.Secret("password", "Password"),
	dsl.String("password_file", "Password file"), // want "env var APP_G_PASSWORD_FILE of g.password_file collides with another key or group"
)

var App = dsl.App("app", "A", G)
