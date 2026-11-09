package schema

import (
	"net/netip"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Enum("addr", "Address", netip.IPv6Loopback()), // want "g.addr: enum type netip.Addr must have a string, integer, float or bool underlying type"
)

var App = dsl.App("app", "A", G)
