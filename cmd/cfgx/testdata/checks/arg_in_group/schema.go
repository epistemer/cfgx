package schema

import "github.com/epistemer/cfgx/dsl"

var down = dsl.Bool("down", "D")

var G = dsl.Group("g", "G", down)

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Migrate").Args(down) // want "key \"down\" in command Migrate is also in group \"g\""
