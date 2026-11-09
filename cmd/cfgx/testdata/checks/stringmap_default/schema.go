package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.StringMap("labels", "Labels").Default(map[string]string{"a=b": "c"}), // want "g.labels: default key \"a=b\" must be non-empty, without = or outer spaces"
)

var App = dsl.App("app", "A", G)
