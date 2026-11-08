package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Migrate").Uses(G).Args(
	dsl.Bool("down", "D").Alias("old.down"), // want "aliases aren't supported on arguments"
)
