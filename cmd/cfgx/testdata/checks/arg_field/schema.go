package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("x", "X", dsl.String("host", "H")).GoName("ServeX")

var App = dsl.App("app", "A", G)

var Cmd = dsl.Command("Serve").Args(
	dsl.String("x_host", "H"), // want "Go name ServeXHost of x_host collides"
)
