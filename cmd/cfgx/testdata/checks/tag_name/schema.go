package schema

import "github.com/epistemer/cfgx/dsl"

var mailer = dsl.Tag("mailer", "Mail") // want "tag name \"mailer\" must be an exported Go identifier"

var G = dsl.Group("g", "G", dsl.Int("x", "X").Tags(mailer))

var App = dsl.App("app", "A", G)
