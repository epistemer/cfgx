// Package schema has 100 keys in ten groups, a quarter each of ints,
// strings, durations and bools, all used by one command.
package schema

import (
	"fmt"
	"time"

	"github.com/epistemer/cfgx/dsl"
)

// The groups are built in a variable's initializer, not in init, which
// would run after App's.
var groups, uses = build()

func build() ([]*dsl.GroupDef, []dsl.Usable) {
	var gs []*dsl.GroupDef
	var us []dsl.Usable
	for g := range 10 {
		var keys []dsl.Member
		for k := range 10 {
			name, desc := fmt.Sprintf("k%d", k), fmt.Sprintf("Key %d of group %d", k, g)
			switch k % 4 {
			case 0:
				keys = append(keys, dsl.Int(name, desc).Default(k).Min(0))
			case 1:
				keys = append(keys, dsl.String(name, desc).Default("v"))
			case 2:
				keys = append(keys, dsl.Duration(name, desc).Default(time.Duration(k)*time.Second))
			case 3:
				keys = append(keys, dsl.Bool(name, desc))
			}
		}
		gr := dsl.Group(fmt.Sprintf("g%d", g), fmt.Sprintf("Group %d", g), keys...)
		gs, us = append(gs, gr), append(us, gr)
	}
	return gs, us
}

var App = dsl.App("bench", "100 keys", groups...).EnvPrefix("BENCH").ConfigFile("bench", ".")

var Run = dsl.Command("Run").Uses(uses...)
