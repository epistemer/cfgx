package schema

import "github.com/epistemer/cfgx/dsl"

var Mailer = dsl.Tag("Mailer", "") // want "tag Mailer needs a description"

var G = dsl.Group("g", "G", dsl.Int("x", "X").Tags(Mailer))

var App = dsl.App("app", "A", G)
