package schema

import (
	"time"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Int("a", "A").Default(0).Range(1, 10),                  // want "g.a: default 0 is below the minimum 1"
	dsl.Int("b", "B").Range(10, 1),                             // want "g.b: Range(10, 1): the minimum is above the maximum"
	dsl.Duration("c", "C").Max(time.Second).Default(time.Hour), // want "g.c: default 1h0m0s is above the maximum 1s"
	dsl.String("d", "D").Pattern(`[a-`),                        // want "g.d: Pattern \"[a-\" doesn't compile"
	dsl.String("e", "E").Pattern(`[a-z]+`).Default("A1"),       // want "g.e: default \"A1\" doesn't match the pattern [a-z]+"
	dsl.String("f", "F").Len(2, 3).Default("long"),             // want "g.f: default \"long\" is 4 characters long, outside Len(2, 3)"
	dsl.String("h", "H").Len(3, 1),                             // want "g.h: Len(3, 1) needs 0 <= min <= max"
)

var App = dsl.App("app", "A", G)
