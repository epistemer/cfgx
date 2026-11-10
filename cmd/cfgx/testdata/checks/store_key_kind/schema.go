package schema

import "github.com/epistemer/cfgx/dsl"

var (
	bolt = dsl.Path("bolt", "Bolt file").Mutable()
	G    = dsl.Group("g", "G", bolt, dsl.Int("rate", "Rate").Mutable())
)

var App = dsl.App("app", "A", G).Store(bolt) // want "App.Store's key g.bolt can't be Mutable"
