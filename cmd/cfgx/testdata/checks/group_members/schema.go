package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Bool("tls", "TLS"),
	dsl.Group("tls", "TLS settings", dsl.Int("x", "X")), // want "group \"server\" has two members named \"tls\""
)

var App = dsl.App("app", "A", G)
