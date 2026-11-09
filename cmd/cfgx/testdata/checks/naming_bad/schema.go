package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.String("a", "A").Flag("--a"),        // want "flag name \"--a\" of g.a must start with a-z"
	dsl.String("b", "B").Env("app_b"),       // want "env var name \"app_b\" of g.b must be upper-case"
	dsl.String("c", "C").GoName("lower"),    // want "GoName \"lower\" of g.c must be an exported Go identifier"
	dsl.String("d", "D").Key("Bad-Key"),     // want "Key \"Bad-Key\" of g.d must be lower-case words"
	dsl.String("e", "E").Short('é'),         // want "Short \"é\" of g.e must be one ASCII letter or digit"
	dsl.Int("f", "F").Example(0).Short('f'), // ok
)

var App = dsl.App("app", "A", G)
