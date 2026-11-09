package schema

import "github.com/epistemer/cfgx/dsl"

var G = dsl.Group("g", "G",
	dsl.Secret("a", "A").Sources(dsl.FromFlag, dsl.FromEnv), // want "g.a: a Secret never has a flag"
	dsl.String("b", "B").Sources(),                          // want "g.b: Sources needs at least one of FromFlag, FromEnv and FromFile"
	dsl.String("c", "C").Flag("cee").Sources(dsl.FromEnv),   // want "g.c names a flag, but Sources leaves out FromFlag"
	dsl.String("d", "D").Sources(dsl.FromFile),              // want "g.d can only come from the config file, but the app has none"
	dsl.Secret("e", "E").Short('e'),                         // want "g.e is a Secret, which never has a flag"
)

type CIDR struct{ s string }

func (c *CIDR) UnmarshalText(b []byte) error { c.s = string(b); return nil }

var H = dsl.Group("h", "H",
	dsl.String("twice", "T").Sources(dsl.FromFile, dsl.FromFile), // want "h.twice can only come from the config file, but the app has none"
	dsl.Text("cidr", "C", CIDR{}).Example(CIDR{"x"}),             // want "h.cidr: an example needs schema.CIDR to implement encoding.TextMarshaler"
)

var App = dsl.App("app", "A", G, H)
