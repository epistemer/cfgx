package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.String("a", "A").Short('x'),
	dsl.String("b", "B").Flag("g-a"),    // want "flag --g-a of g.b collides with another key or group"
	dsl.String("c", "C").Env("APP_G_A"), // want "env var APP_G_A of g.c collides with another key or group"
	dsl.String("d", "D").GoName("A"),    // want "Go name GA of g.d collides with another key or group"
	dsl.String("e", "E").Key("a"),       // want "file key g.a of g.a collides with another key or group"
	dsl.String("f", "F").Short('x'),     // want "short flag -x of g.f collides with another key or group"
)

var App = dsl.App("app", "A", G)
