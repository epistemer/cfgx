package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G", dsl.Int("x", "X"))

var App = dsl.App("app", "A", G).Initialisms("otp") // want "initialism \"otp\" must be upper-case"
