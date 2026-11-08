package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G).ConfigFile("app", ".").Formats(dsl.YAML, dsl.YAML) // want "Formats lists yaml twice"
