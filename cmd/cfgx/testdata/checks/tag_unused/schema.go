package schema

import "github.com/epistemer/cfgx/dsl"

var Mailer = dsl.Tag("Mailer", "Mail") // want "tag Mailer isn't on any key"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G)
