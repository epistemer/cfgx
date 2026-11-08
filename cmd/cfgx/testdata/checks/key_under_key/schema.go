package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("server", "S",
	dsl.Bool("tls", "TLS"),
	dsl.Bool("tls.enabled", "TLS on"), // want "file key server.tls.enabled needs server.tls to be a map"
)

var App = dsl.App("app", "A", G)
