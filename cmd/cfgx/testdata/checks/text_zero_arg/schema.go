package schema

import (
	"net/netip"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Text("cidr", "Range", netip.MustParsePrefix("10.0.0.0/8")), // want "g.cidr: Text's third argument only names the type"
)

var App = dsl.App("app", "A", G)
