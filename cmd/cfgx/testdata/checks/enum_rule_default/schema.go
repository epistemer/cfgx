package schema

import "github.com/epistemer/cfgx/dsl"

// A default that is the type's zero value counts as not set, so only b is
// reported, on the rule.
var (
	a = dsl.Enum("a", "A", 0, 1).Default(0)
	b = dsl.Enum("b", "B", "x", "y").Default("y")
)

var G = dsl.Group("g", "G", a, b).Rules(dsl.AtMostOne(a, b)) // want "AtMostOne can't include g.b, whose default y counts as set"

var App = dsl.App("app", "A", G)
