package schema

import "github.com/epistemer/cfgx/dsl"

type Level int

func (Level) String() string { return "same" }

var G = dsl.Group("g", "G",
	dsl.Enum("level", "Level", Level(1), Level(2)), // want "g.level: values 1 and 2 have the same text \"same\""
)

var App = dsl.App("app", "A", G)
