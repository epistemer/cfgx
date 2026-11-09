package schema

import (
	"math"

	"github.com/epistemer/cfgx/dsl"
)

var G = dsl.Group("g", "G",
	dsl.Float("ratio", "Ratio").Default(math.NaN()), // want "default of g.ratio: \"NaN\" is not a finite number"
)

var App = dsl.App("app", "A", G)
