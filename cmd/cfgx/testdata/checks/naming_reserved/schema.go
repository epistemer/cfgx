package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.String("a", "A").Flag("config"),    // want "flag --config of g.a collides with the config file flag"
	dsl.String("b", "B").Flag("help"),      // want "flag --help of g.b collides with the built-in help flag"
	dsl.String("c", "C").Short('h'),        // want "short flag -h of g.c collides with the built-in help flag"
	dsl.String("d", "D").Env("APP_CONFIG"), // want "env var APP_CONFIG of g.d collides with the env var that names the config file"
)

var App = dsl.App("app", "A", G).ConfigFile("app", ".")
