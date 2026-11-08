package schema

import "github.com/epistemer/cfgx/dsl"

var (
	Mailer  = dsl.Tag("Mailer", "Mail")
	Mailer2 = dsl.Tag("Mailer", "Mail") // want "tag Mailer is declared twice"
)

var G = dsl.Group("g", "G", dsl.Int("x", "X").Tags(Mailer, Mailer2))

var App = dsl.App("app", "A", G)
