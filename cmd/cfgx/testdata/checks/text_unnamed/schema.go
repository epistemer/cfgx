package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Text("raw", "Raw", []byte(nil)), // want "g.raw: text type []uint8 must be a named type from a package"
)

var App = dsl.App("app", "A", G)
