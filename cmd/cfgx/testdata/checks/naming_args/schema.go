package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.String("host", "Host").Short('n'))

var App = dsl.App("app", "A", G)

var Run = dsl.Command("Run").Uses(G).Args(
	dsl.Int("steps", "Steps").Env("STEPS"), // want "argument steps of Run is flag-only, so Env, Key and Sources don't apply"
	dsl.Bool("dry", "Dry run").Short('n'),  // want "short flag -n of dry collides with another key's"
	dsl.Bool("down", "Down").GoName("Rollback").Short('d'),
)
