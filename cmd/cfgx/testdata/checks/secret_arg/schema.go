package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.String("host", "Host"))

var App = dsl.App("app", "A", G)

var Run = dsl.Command("Run").Uses(G).Args(
	dsl.Secret("token", "Token"), // want "argument token of Run is a Secret, which never has a flag"
)
