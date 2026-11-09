package schema

import (
	"time"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Text("tz", "Zone", time.Location{}), // want "g.tz: text type time.Location doesn't implement encoding.TextUnmarshaler"
)

var App = dsl.App("app", "A", G)
