package schema

import (
	"math"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Enum("ratio", "Ratio", 0.5, math.NaN()), // want "g.ratio: value 2: NaN isn't a finite number"
)

var App = dsl.App("app", "A", G)
