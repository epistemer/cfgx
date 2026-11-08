package schema

import "github.com/epistemer/cfgx/dsl"

var Mailer = dsl.Tag("Mailer", "Mail")

var G = dsl.Group("g", "G", dsl.Int("x", "X")).Tags(Mailer)

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Serve").Uses(Mailer, Mailer) // want "command Serve uses tag Mailer twice"
