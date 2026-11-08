package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G).ConfigFile("app", ".")

var Cmd = dsl.Command("Migrate").Uses(G).Args(
	dsl.String("config", "C"), // want "flag --config of config collides with the config file flag"
)
