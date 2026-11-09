package schema

import "github.com/epistemer/cfgx/dsl"

type Mode[T any] string

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", Mode[int]("a")), // want "is a generic type; use a non-generic named type"
)

var App = dsl.App("app", "A", G)
