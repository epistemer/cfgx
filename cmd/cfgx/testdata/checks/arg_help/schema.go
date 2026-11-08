package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Migrate").Uses(G).Args(
	dsl.Bool("help", "H"), // want "flag --help of help collides with the built-in help flag"
)
