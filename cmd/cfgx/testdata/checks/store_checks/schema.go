package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Secret("pw", "Password").Mutable(), // want "g.pw is a secret, which can't be Mutable in v0.1.0"
	dsl.Int("rate", "Rate").Mutable(),      // want "g.rate is Mutable, but the app has no store; add App.Store(pathKey)"
	dsl.File("ca", "CA").Mutable(),         // want "g.ca is a file key, which can't be Mutable: its value is a file's contents"
)

var App = dsl.App("app", "A", G)

var Run = dsl.Command("Run").Uses(G).WithStore().Args( // want "command Run uses WithStore, but the app has no store"
	dsl.Int("n", "N").Mutable(), // want "argument n of Run can't be Mutable"
)
