package schema

import (
	"errors"

	"github.com/epistemer/cfgx/dsl"
)

type Mode int

func (Mode) MarshalText() ([]byte, error) { return nil, errors.New("boom") }

var G = dsl.Group("g", "G",
	dsl.Enum("mode", "Mode", Mode(1)), // want "g.mode: value 1: MarshalText failed: boom"
)

var App = dsl.App("app", "A", G)
