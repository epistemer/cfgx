package schema

import "github.com/epistemer/cfgx/dsl"

var Mailer = dsl.Tag("Mailer", "Mail")

var G = dsl.Group("g", "G", dsl.Int("x", "X")).Tags(Mailer)

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Migrate").Uses(G).Args(
	dsl.Bool("down", "D").Tags(Mailer), // want "argument down of Migrate has tags"
)
